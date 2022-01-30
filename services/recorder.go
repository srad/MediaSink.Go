package services

import (
	"fmt"
	"github.com/srad/streamsink/media"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/models"
	"github.com/srad/streamsink/utils"
)

var (
	recorded   map[string]*exec.Cmd
	info       map[string]*models.Recording
	pause      bool = false
	isOnline   map[string]bool
	frameWidth = "480"
	frameName  = "live.jpg"
	quit       chan bool
)

const (
	recordingMinMinutes = 15
)

func Init() {
	recorded = make(map[string]*exec.Cmd)
	info = make(map[string]*models.Recording)
	isOnline = make(map[string]bool)
	pause = true
	quit = make(chan bool)
}

func iterate() {
	for {
		select {
		case <-quit:
			fmt.Println("Stopping iteration")
			return
		default:
			// Keep checking channels
			channels, err := models.GetChannels()
			if err != nil {
				log.Println(err)
				return
			}
			for _, channel := range channels {
				if pause {
					break
				}
				time.Sleep(3 * time.Second)
				url, _ := getStreamUrl(channel)

				isOnline[channel.ChannelName] = url != ""
				log.Printf("Checking: %s|%s|%s|%s\n", channel.ChannelName, strconv.FormatBool(channel.IsPaused), strconv.FormatBool(isOnline[channel.ChannelName]), url)

				if url != "" {
					log.Println("[Recorder] Extracting first frame of ", channel.ChannelName)
					err := extractFirstFrame(url, frameWidth, filepath.Join(conf.AbsoluteDataPath(channel.ChannelName), frameName))
					if err != nil {
						log.Printf("Error extracting first frame of channel | file: %s | %s", channel.ChannelName, frameName)
					}
				}

				if url == "" || pause || channel.IsPaused {
					continue
				}
				go capture(channel.ChannelName, url)
			}
			// Wait between each round to reduce the chance of API blocking
			time.Sleep(90 * time.Second)
		}
	}
}

func Resume() {
	pause = false
	go iterate()
}

func Pause() {
	pause = true
	StopAll()
}

func deleteRecordingData(channelName string) {
	delete(recorded, channelName)
	delete(info, channelName)
}

// Starts and also waits for the stream to end or being killed
func capture(channelName, url string) {
	if _, ok := recorded[channelName]; ok {
		log.Println("Already recording: " + channelName)
		return
	}

	conf.MakeChannelFolders(channelName)

	now := time.Now()
	stamp := now.Format("2006_01_02_15_04_05")
	filename := fmt.Sprintf("%s_%s.mp4", channelName, stamp)
	outputFile := filepath.Join(conf.AppCfg.RecordingsAbsolutePath, channelName, filename)
	relativePath := filepath.Join("recordings", channelName, filename)

	log.Println("----------------------------------------Capturing----------------------------------------")
	log.Println("Url: " + url)
	log.Println("to: " + outputFile)
	log.Println("-----------------------------------------------------------------------------------------")

	info[channelName] = &models.Recording{ChannelName: channelName, Filename: filename, Duration: 0, Bookmark: false, CreatedAt: now, PathRelative: relativePath}
	recorded[channelName] = exec.Command("ffmpeg", "-hide_banner", "-loglevel", "quiet", "-i", url, "-c", "copy", outputFile)

	_, err := recorded[channelName].CombinedOutput()

	// 255 is overflow value and is not an error
	if err != nil && !strings.Contains(err.Error(), "255") {
		log.Println(fmt.Sprintf("cmd.Start: %v", err))
		deleteRecordingData(channelName)
		return
	}

	job, jobErr := models.EnqueueRecordingJob(channelName, filename, outputFile)
	if jobErr != nil {
		log.Printf("error enqueuing reccording job")
	}

	if err := recorded[channelName].Wait(); err != nil && !strings.Contains(err.Error(), "255") {
		finishRecording(channelName, filename, job.JobId)
	} else {
		deleteRecordingData(channelName)
		models.DeleteJob(job.JobId)
	}
}

func finishRecording(channelName, filename string, jobId uint) {
	duration := time.Now().Sub(info[channelName].CreatedAt).Minutes()

	if duration > recordingMinMinutes {
		if err := models.AddRecording(info[channelName]); err != nil {
			log.Printf("[Recording] Error adding recording: %v\n", info[channelName])
		}
		models.EnqueuePreviewJob(channelName, filename)
	} else {
		log.Printf("[Recorder] Deleting stream '%s/%s' because it is too short (%vmin)\n", channelName, filename, duration)
		err := models.DeleteRecordingsFile(channelName, filename)
		if err != nil {
			log.Printf("[Recorder] Error deleting '%s/%s': %v\n", channelName, filename, err.Error())
		}
	}

	models.DeleteJob(jobId)
	deleteRecordingData(channelName)

	log.Println("Stream has ended for " + channelName)
}

func StopAll() error {
	log.Println("[Recorder] Stopping all channels")

	// Stop the go routine for iteration over channels
	quit <- true

	// Stop each recording individually
	channels, err := models.GetChannels()
	if err != nil {
		log.Println(err)
		return err
	}
	for _, channel := range channels {
		err := Stop(channel.ChannelName, false)
		if err != nil {
			log.Printf("Error stopping channel '%s': %v", channel.ChannelName, err)
		}
	}

	return nil
}

func Start(channel *models.Channel) error {
	err := models.Pause(channel.ChannelName, false)
	if err != nil {
		return err
	}

	url, err := getStreamUrl(channel)
	if err != nil {
		// Ignore, offline raises also an error
	}
	isOnline[channel.ChannelName] = url != ""

	if url != "" {
		go extractFirstFrame(url, frameWidth, filepath.Join(conf.AbsoluteDataPath(channel.ChannelName), frameName))
	}

	if url == "" {
		return nil
	}
	go capture(channel.ChannelName, url)
	if err != nil {
		return err
	}

	return nil
}

func Stop(channelName string, updateModel bool) error {
	if updateModel {
		err := models.Pause(channelName, true)
		if err != nil {
			return err
		}
	}

	// channel exists?
	if cmd, ok := recorded[channelName]; ok {
		if runtime.GOOS == "windows" {
			if err := utils.TerminateProc(channelName); err != nil {
				go models.AddRecording(info[channelName])
				log.Println("Killed process: " + channelName)
			} else {
				log.Println("Error killing process: " + channelName)
			}
		} else {
			// linux
			log.Printf("Interrupting process '%s'", channelName)
			if err := cmd.Process.Signal(os.Interrupt); err != nil && !strings.Contains(err.Error(), "255") {
				fmt.Println(err.Error())
				log.Println("Error killing process: " + channelName)
			} else {
				log.Printf("Killed process: %s", channelName)
			}
		}
	}

	return nil
}

func IsRecording(channelName string) bool {
	if _, ok := recorded[channelName]; ok {
		return true
	}
	return false
}

func RecordingMinutes(channelName string) float64 {
	if _, ok := recorded[channelName]; ok {
		return time.Now().Sub(info[channelName].CreatedAt).Minutes()
	}
	return 0
}

func IsOnline(channelName string) bool {
	if _, ok := isOnline[channelName]; ok {
		return isOnline[channelName]
	}
	return false
}

func getStreamUrl(channel *models.Channel) (string, error) {
	cmd := exec.Command("youtube-dl", "--force-ipv4", "-f best", "--get-url", channel.Url)
	stdout, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(stdout))

	if err != nil {
		return "", err
	}

	return output, nil

}

func Recording() bool {
	return !pause
}

func extractFirstFrame(input, height, output string) error {
	err := utils.ExecSync("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", input, "-r", "1", "-vf", "scale="+height+":-1", "-q:v", "2", "-frames:v", "1", output)

	if err != nil {
		log.Printf("[Recorder] Error extracting frame: %v", err.Error())
		return nil
	}

	return nil
}

func UpdateVideoInfo() error {
	log.Println("[Recorder] Updating all recordings info")
	recordings, err := models.FindAll()
	count := len(recordings)
	if err != nil {
		log.Printf("Error %v", err)
		return err
	}

	i := 1
	for _, rec := range recordings {
		info, err := media.GetVideoInfo(conf.AbsoluteFilepath(rec.ChannelName, rec.Filename))
		if err != nil {
			log.Printf("[UpdateVideoInfo] Error updating video info: %v", err)
			continue
		}

		errUpdate := models.Db.Updates(&models.Recording{ChannelName: rec.ChannelName, Filename: rec.Filename, Duration: info.Duration, BitRate: info.BitRate, Size: info.Size, Width: info.Width, Height: info.Height}).Error
		if errUpdate != nil {
			log.Printf("[Recorder] Error updating video info: %v", errUpdate.Error())
			continue
		}
		log.Printf("[Recorder] Updated %s (%d/%d)", rec.Filename, i, count)
		i++
	}

	return nil
}

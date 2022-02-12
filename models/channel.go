package models

import (
	"errors"
	"fmt"
	"github.com/srad/streamsink/utils"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/srad/streamsink/conf"
	"gorm.io/gorm"
)

const (
	frameWidth = "480"
	frameName  = "live.jpg"
)

var (
	info     = make(map[string]*Recording)
	pause    = false
	isOnline = make(map[string]bool)
	quit     chan bool
	recorded = make(map[string]*exec.Cmd)
	// "tag1,tag2,...
	rTags, _ = regexp.Compile("^[a-z\\-0-9]+(,[a-z\\-0-9]+)*$")
)

type Channel struct {
	ChannelId   uint   `json:"channelId" gorm:"primaryKey;not null;default:null"`
	ChannelName string `json:"channelName" gorm:"unique;not null;default:null"`
	Url         string `json:"url" gorm:"unique;not null;default:null"`

	Tags string `json:"tags" gorm:"not null;default:''"`

	Fav             bool        `json:"fav" gorm:"not null;default:0"`
	IsPaused        bool        `json:"isPaused" gorm:"not null"`
	CreatedAt       time.Time   `json:"createdAt"`
	Recordings      []Recording `json:"recordings" gorm:"table:recordings;foreignKey:channel_name;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	RecordingsCount uint        `json:"recordingsCount"`
	RecordingsSize  uint        `json:"recordingsSize"`
}

func (channel *Channel) Create(tags *[]string) error {
	channel.ChannelName = strings.ToLower(strings.TrimSpace(channel.ChannelName))
	channel.IsPaused = false
	channel.CreatedAt = time.Now()

	if tags != nil {
		str, err := prepareTags(*tags)
		if err != nil {
			return err
		}
		channel.Tags = str
	}

	if err := Db.Create(&channel).Error; err != nil {
		return err
	}

	conf.MakeChannelFolders(channel.ChannelName)

	return nil
}

func TagChannel(channelName string, tags []string) error {
	if len(tags) > 0 {
		joined, err := prepareTags(tags)
		if err != nil {
			return err
		}

		return Db.Table("channels").
			Where("channel_name = ?", channelName).
			Update("tags", joined).Error
	}

	return Db.Table("channels").
		Where("channel_name = ?", channelName).
		Update("tags", "").Error
}

func prepareTags(tags []string) (string, error) {
	joined := strings.ToLower(strings.Join(tags, ","))
	if !rTags.MatchString(joined) {
		return "", errors.New(fmt.Sprintf("Invalid tags: '%s'", tags))
	}

	return joined, nil
}

func IsRecording(channelName string) bool {
	if _, ok := recorded[channelName]; ok {
		return true
	}
	return false
}

func IsPaused() bool {
	return pause
}

func (channel *Channel) Start() error {
	err := channel.Pause(false)
	if err != nil {
		return err
	}

	url, err := channel.StreamUrl()
	if err != nil {
		// Ignore, offline raises also an error
	}
	isOnline[channel.ChannelName] = url != ""

	if url != "" {
		go ExtractFirstFrame(url, frameWidth, filepath.Join(conf.AbsoluteDataPath(channel.ChannelName), frameName))
	}

	if url == "" {
		return nil
	}
	go channel.Capture(url)
	if err != nil {
		return err
	}

	return nil
}

func (channel *Channel) Stop(updateModel bool) error {
	return channel.terminateProcess(updateModel)
}

// TerminateProcess Terminate the ffmpeg recording process
// There's maximum one recording job per channel.
func (channel *Channel) terminateProcess(updateModel bool) error {
	if updateModel {
		err := channel.Pause(true)
		if err != nil {
			return err
		}
	}

	// channel exists?
	if cmd, ok := recorded[channel.ChannelName]; ok {
		if runtime.GOOS == "windows" {
			if err := utils.TerminateProc(channel.ChannelName); err != nil {
				log.Println("[TerminateProcess] Error killing process: " + channel.ChannelName)
				return err
			} else {
				log.Println("Killed process: " + channel.ChannelName)
			}
		} else {
			// linux
			log.Printf("Interrupting process '%s'", channel.ChannelName)
			if err := cmd.Process.Signal(os.Interrupt); err != nil && !strings.Contains(err.Error(), "255") {
				log.Printf("[TerminateProcess] Error killing process for '%s': %v", channel.ChannelName, err)
				return err
			} else {
				log.Printf("[TerminateProcess] Killed process: '%s'", channel.ChannelName)
			}
		}
	}

	return nil
}

func (channel *Channel) IsOnline() bool {
	if _, ok := isOnline[channel.ChannelName]; ok {
		return isOnline[channel.ChannelName]
	}
	return false
}

func (channel *Channel) IsRecording() bool {
	if _, ok := recorded[channel.ChannelName]; ok {
		return true
	}
	return false
}

func (channel *Channel) StreamUrl() (string, error) {
	cmd := exec.Command("youtube-dl", "--force-ipv4", "--ignore-errors", "--youtube-skip-dash-manifest", "-f best/bestvideo", "--get-url", channel.Url)
	stdout, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(stdout))

	if err != nil {
		return "", err
	}

	return output, nil

}

func GetChannelByName(channelName string) (*Channel, error) {
	var channel Channel
	err := Db.Where("channel_name = ?", channelName).First(&channel).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return &channel, nil
}

func ChannelList() ([]*Channel, error) {
	var result []*Channel

	err := Db.Model(&Channel{}).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_count", "(SELECT SUM(size) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_size").
		Find(&result).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		log.Println(err)
		return nil, err
	}

	return result, nil
}

func ChannelActiveList() ([]*Channel, error) {
	var result []*Channel

	err := Db.Model(&Channel{}).
		Where("is_paused = ?", false).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_count").
		Find(&result).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		log.Println(err)
		return nil, err
	}

	return result, nil
}

func (channel *Channel) FavChannel() error {
	return Db.Table("channels").
		Where("channel_name = ?", channel.ChannelName).
		Update("fav", true).Error
}

func (channel *Channel) UnFavChannel() error {
	return Db.Table("channels").
		Where("channel_name = ?", channel.ChannelName).
		Update("fav", false).Error
}

func (channel *Channel) UpdateStreamInfo(url string) error {
	info, _ := GetVideoInfo(url)

	return Db.Table("channels").
		Where("channel_name = ?", channel.ChannelName).
		Update("bit_rate", info.BitRate).
		Update("width", info.Width).
		Update("height", info.Height).Error
}

func (channel *Channel) Destroy() error {
	if err := channel.DestroyAllRecordings(); err != nil {
		log.Printf("Error deleting recordings of channel '%s': %v", channel.ChannelName, err)
		return err
	}

	if err := Db.Where("channel_name = ?", channel.ChannelName).Delete(Channel{}).Error; err != nil {
		return err
	}

	if errRemove := os.RemoveAll(conf.AbsoluteRecordingsPath(channel.ChannelName)); errRemove != nil {
		log.Printf("Error deleting channel folder: %v", errRemove)
		return errRemove
	}

	return nil
}

func (channel *Channel) DestroyAllRecordings() error {
	var recordings []*Recording
	if err := Db.Where("channel_name = ?", channel.ChannelName).
		Find(&recordings).Error; err != nil {
		return err
	}

	for _, recording := range recordings {
		if err := recording.Destroy(); err != nil {
			log.Printf("Error deleting recording '%s': %v", err, recording.Filename)
		}
	}

	return nil
}

func (channel *Channel) Pause(pauseVal bool) error {
	if err := Db.Table("channels").
		Where("channel_name = ?", channel.ChannelName).
		Update("is_paused", pauseVal).Error; err != nil {
		return err
	}

	return nil
}

func (channel *Channel) RecordingMinutes() float64 {
	if _, ok := recorded[channel.ChannelName]; ok {
		return time.Now().Sub(info[channel.ChannelName].CreatedAt).Minutes()
	}
	return 0
}

func (channel *Channel) RemoveData() {
	delete(recorded, channel.ChannelName)
	delete(info, channel.ChannelName)
	isOnline[channel.ChannelName] = false
}

// Capture Starts and also waits for the stream to end or being killed
func (channel *Channel) Capture(url string) error {
	if _, ok := recorded[channel.ChannelName]; ok {
		log.Println("[Channel] Already recording: " + channel.ChannelName)
		return nil
	}

	conf.MakeChannelFolders(channel.ChannelName)

	now := time.Now()
	stamp := now.Format("2006_01_02_15_04_05")
	filename := fmt.Sprintf("%s_%s.mp4", channel.ChannelName, stamp)
	outputFile := filepath.Join(conf.AppCfg.RecordingsAbsolutePath, channel.ChannelName, filename)
	relativePath := filepath.Join("recordings", channel.ChannelName, filename)

	log.Println("----------------------------------------Capturing----------------------------------------")
	log.Println("Url: " + url)
	log.Println("to: " + outputFile)

	info[channel.ChannelName] = &Recording{ChannelName: channel.ChannelName, Filename: filename, Duration: 0, Bookmark: false, CreatedAt: now, PathRelative: relativePath}
	recorded[channel.ChannelName] = exec.Command("ffmpeg", "-hide_banner", "-loglevel", "quiet", "-i", url, "-c", "copy", outputFile)

	sterr, _ := recorded[channel.ChannelName].StderrPipe()

	if err := recorded[channel.ChannelName].Start(); err != nil {
		log.Printf("cmd.Start: %v", err)
		return err
	}

	// Before recording store that the process has started, for recovery
	recordingJob, err := EnqueueRecordingJob(channel.ChannelName, filename, outputFile)
	if err != nil {
		log.Printf("[Capture] Error enqueuing reccording for: %s/%s: %v", channel.ChannelName, filename, err)
	}

	if b, err := io.ReadAll(sterr); err != nil {
		log.Printf("[Capture] %s: %v", string(b), err)
	}

	// Wait for process to exit
	if err := recorded[channel.ChannelName].Wait(); err != nil && !strings.Contains(err.Error(), "255") {
		log.Printf("[Capture] Wait for process exit '%s' error: %v", channel.ChannelName, err)
		channel.RemoveData()
		channel.DeleteRecordingsFile(filename)
		recordingJob.Destroy()
		if exiterr, ok := err.(*exec.ExitError); ok {
			log.Printf("[Capture] Exec error: %v", err)
			// The program has exited with an exit code != 0

			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if _, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return err
				//return status.ExitStatus()
			}
		}
		return err
	}

	// Finish recording

	duration := int(time.Now().Sub(channel.Info().CreatedAt).Minutes())

	if duration > conf.AppCfg.MinRecMin {
		// keep
		if err := channel.Info().Save("recording"); err != nil {
			log.Printf("[Info] Error adding recording: %v\n", channel.Info())
		}

		// No access to info after this!
		channel.RemoveData()
		recordingJob.Destroy()

		if job, err := EnqueuePreviewJob(channel.ChannelName, filename); err != nil {
			log.Printf("[FinishRecording] Error enqueuing job for %v\n", err)
			return err
		} else {
			log.Printf("[FinishRecording] Job enqueued %v\n", job)
		}
	} else {
		// Throw away
		log.Printf("[FinishRecording] Deleting stream '%s/%s' because it is too short (%vmin)\n", channel.ChannelName, filename, duration)

		channel.RemoveData()
		recordingJob.Destroy()

		if err := channel.DeleteRecordingsFile(filename); err != nil {
			log.Printf("[FinishRecording] Error deleting '%s/%s': %v\n", channel.ChannelName, filename, err.Error())
			return err
		}
	}

	return nil
}

func (channel *Channel) Info() *Recording {
	return info[channel.ChannelName]
}

func (channel *Channel) Online(val bool) {
	isOnline[channel.ChannelName] = val
}

func (channel *Channel) Screenshot(url string) error {
	return ExtractFirstFrame(url, frameWidth, filepath.Join(conf.AbsoluteDataPath(channel.ChannelName), frameName))
}

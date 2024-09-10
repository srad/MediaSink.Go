package services

import (
	"context"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/network"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type StreamInfo struct {
	IsOnline      bool                 `json:"isOnline" extensions:"!x-nullable"`
	IsTerminating bool                 `extensions:"!x-nullable"`
	Url           string               `extensions:"!x-nullable"`
	ChannelName   database.ChannelName `json:"channelName" extensions:"!x-nullable"`
}

type ProcessInfo struct {
	Id     database.ChannelId `json:"id"`
	Pid    int                `json:"pid"`
	Path   string             `json:"path"`
	Args   string             `json:"args"`
	Output string             `json:"output"`
}

var (
	recInfo    = make(map[database.ChannelId]*database.Recording)
	streamInfo = make(map[database.ChannelId]StreamInfo)
	// Pointer to process which executed FFMPEG
	streams = make(map[database.ChannelId]*exec.Cmd)
)

func (si *StreamInfo) Screenshot() error {
	return helpers.ExtractFirstFrame(si.Url, conf.FrameWidth, filepath.Join(si.ChannelName.AbsoluteChannelDataPath(), database.SnapshotFilename))
}

// CaptureChannel Starts and also waits for the stream to end or being killed
// This code is intentionally procedural and contains all the steps to finish a recording.
func CaptureChannel(id database.ChannelId, url string, skip uint) error {
	channel, err := id.GetChannelById()
	if err != nil {
		return err
	}

	if _, ok := streams[id]; ok {
		return nil
	}

	// Folder could not be created and does not exist yet.
	if err := channel.ChannelName.MkDir(); err != nil && !os.IsExist(err) {
		return err
	}

	recording, outputFilePath, err := channel.ChannelId.NewRecording("recording")
	if err != nil {
		return err
	}

	log.Infoln("----------------------------------------Capturing----------------------------------------")
	log.Infoln("Url: " + url)
	log.Infoln("to: " + outputFilePath)

	recInfo[id] = recording
	streams[id] = exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-i", url, "-ss", fmt.Sprintf("%d", skip), "-movflags", "faststart", "-c", "copy", outputFilePath)
	cmdStr := strings.Join([]string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-i", url, "-ss", fmt.Sprintf("%d", skip), "-movflags", "faststart", "-c", "copy", outputFilePath}, " ")
	log.Infof("Executing: %s", cmdStr)

	sterr, _ := streams[id].StderrPipe()

	if err := streams[id].Start(); err != nil {
		log.Errorf("cmd.Start: %s", err)
		return err
	}

	if b, err := io.ReadAll(sterr); err != nil {
		log.Errorf("[Capture] %s: %s", string(b), err)
	}

	// Wait for process to exit
	if err := streams[id].Wait(); err != nil && !strings.Contains(err.Error(), "255") {
		log.Errorf("[Capture] Wait for process exit '%s' error: %s", channel.ChannelName, err)
		DeleteStreamData(id)
		if err := os.Remove(outputFilePath); err != nil {
			log.Errorf("[Capture] Error deleting recording file '%s': %s", outputFilePath, err)
		}
		var exiterr *exec.ExitError
		if errors.As(err, &exiterr) {
			log.Errorf("[Capture] Exec error: %s", err)
			// The program has exited with an exit code != 0

			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if _, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return err
				// return status.ExitStatus()
			}
		}
		return err
	}

	// Finish recording
	duration := time.Now().Sub(recording.CreatedAt)

	// Query the latest minimum recording duration or set a default of 10min.

	log.Infof("Minimum recording duration for channel %s is %dmin", channel.ChannelName, channel.MinDuration)

	// Duration might have changed since the process launch.
	channel, errChannel := id.GetChannelById()
	var minDuration = 10 * time.Minute // default
	if errChannel != nil {
		log.Errorf("[Capture] Error querying channel-id %d: %s", id, errChannel)
	} else {
		minDuration = time.Duration(channel.MinDuration) * time.Minute
	}

	// Keep the recording
	if duration.Seconds() >= minDuration.Seconds() {
		info := Info(id)
		if newRecording, err := database.CreateRecording(info.ChannelId, info.Filename, "recording"); err != nil {
			log.Errorf("[Info] Error adding recording '%s': %s", outputFilePath, err)
		} else {
			network.BroadCastClients("recording:add", newRecording)

			if job, err := EnqueuePreviewJob(newRecording.RecordingId); err != nil {
				log.Errorf("[FinishRecording] Error enqueuing job for %s", err)
				return err
			} else {
				log.Infof("[FinishRecording] Job enqueued %v\n", job)
			}
		}
	} else { // Throw away
		log.Infof("[FinishRecording] Deleting stream '%s/%s' because it is too short (%dmin)", channel.ChannelName, recording.Filename, duration)

		if err := os.Remove(outputFilePath); err != nil {
			log.Errorf("[Capture] Error destroying recording: %s", err)
		}
	}

	DeleteStreamData(id)

	return nil
}

func GetRecordingMinutes(id database.ChannelId) float64 {
	if _, ok := streams[id]; ok {
		return time.Now().Sub(recInfo[id].CreatedAt).Minutes()
	}
	return 0
}

func Info(id database.ChannelId) *database.Recording {
	return recInfo[id]
}

func Start(id database.ChannelId) error {
	channel, err := id.GetChannelById()
	if err != nil {
		return err
	}

	// Stop any previous recording, restart
	if err := id.PauseChannel(false); err != nil {
		return err
	}

	url, err := channel.QueryStreamUrl()
	streamInfo[channel.ChannelId] = StreamInfo{IsOnline: url != "", Url: url, ChannelName: channel.ChannelName, IsTerminating: false}
	if url == "" {
		// Channel offline
		return fmt.Errorf("no url found for channel '%s'", channel.ChannelName)
	}
	if err != nil {
		return err
	}

	log.Infof("[Start] Starting '%s' at '%s'", channel.ChannelName, url)

	go func() {
		if err := helpers.ExtractFirstFrame(url, conf.FrameWidth, filepath.Join(channel.ChannelName.AbsoluteChannelDataPath(), database.SnapshotFilename)); err != nil {
			log.Errorf("Error: %s", err)
		}
	}()

	go func() {
		log.Infof("Start capturing url: %s", url)
		if err := CaptureChannel(id, url, channel.SkipStart); err != nil {
			log.Errorf("Error capturing video: %s", err)
		}
	}()

	return nil
}

func TerminateAll() {
	for channelId := range streams {
		if err := TerminateProcess(channelId); err != nil {
			log.Errorf("Error terminating channel: %s", err)
		}
	}
}

// TerminateProcess Interrupt the ffmpeg recording process
// There's maximum one recording job per channel.
func TerminateProcess(id database.ChannelId) error {
	// Is current recording at all?
	if cmd, ok := streams[id]; ok {
		if info, ok2 := streamInfo[id]; ok2 {
			streamInfo[id] = StreamInfo{
				IsOnline:      info.IsOnline,
				IsTerminating: true, // <---------------- only update.
				Url:           info.Url,
				ChannelName:   info.ChannelName,
			}
		}
		if err := cmd.Process.Signal(os.Interrupt); err != nil && !strings.Contains(err.Error(), "255") {
			log.Errorf("[TerminateProcess] Error killing process for channel id %d: %s", id, err)
			return err
		} else {
			log.Infof("[TerminateProcess] Killed process: %d", id)
		}
	}

	return nil
}

func IsOnline(id database.ChannelId) bool {
	if _, ok := streamInfo[id]; ok {
		return streamInfo[id].IsOnline
	}
	return false
}

func IsTerminating(id database.ChannelId) bool {
	if _, ok := streamInfo[id]; ok {
		return streamInfo[id].IsTerminating
	}
	return false
}

func IsRecordingStream(id database.ChannelId) bool {
	if _, ok := streams[id]; ok {
		return true
	}
	return false
}

func DeleteStreamData(id database.ChannelId) {
	delete(streams, id)
	delete(recInfo, id)
	delete(streamInfo, id)
}

func ProcessList() []*ProcessInfo {
	var info []*ProcessInfo

	for id, cmd := range streams {
		var s = ""
		if output, err := cmd.CombinedOutput(); err == nil {
			s = strings.TrimSpace(string(output))
		}

		args := strings.TrimSpace(strings.Join(cmd.Args, " "))

		info = append(info, &ProcessInfo{
			Id:     id,
			Pid:    cmd.Process.Pid,
			Path:   cmd.Path,
			Args:   args,
			Output: s,
		})
	}

	return info
}

// startThumbnailWorker Creates in intervals snapshots of the video as a preview.
func startThumbnailWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Infoln("[startThumbnailWorker] stopped")
			return
		case <-time.After(captureThumbInterval):
			for channelId, info := range streamInfo {
				if info.Url != "" && !info.IsTerminating {
					if err := info.Screenshot(); err != nil {
						log.Errorf("[Recorder] Error extracting first frame of channel-id %d: %s", channelId, err)
					} else {
						network.BroadCastClients("channel:thumbnail", channelId)
					}
				}
			}
		}
	}
}

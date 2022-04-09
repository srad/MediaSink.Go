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
	"strings"
	"syscall"
	"time"

	"github.com/srad/streamsink/conf"
	"gorm.io/gorm"
)

var (
	recInfo    = make(map[string]*Recording)
	pause      = false
	streamInfo = make(map[string]StreamInfo)
	streams    = make(map[string]*exec.Cmd)
	// "tag1,tag2,...
	rTags, _ = regexp.Compile("^[a-z\\-0-9]+(,[a-z\\-0-9]+)*$")
)

type Channel struct {
	ChannelId       uint        `json:"channelId" gorm:"primaryKey;not null;default:null"`
	ChannelName     string      `json:"channelName" gorm:"unique;not null;default:null"`
	DisplayName     string      `json:"displayName" gorm:"not null;default:''"`
	SkipStart       uint        `json:"skipStart" gorm:"not null;default:0"`
	Url             string      `json:"url" gorm:"unique;not null;default:null"`
	Tags            string      `json:"tags" gorm:"not null;default:''"`
	Fav             bool        `json:"fav" gorm:"not null;default:0"`
	IsPaused        bool        `json:"isPaused" gorm:"not null;default:0"`
	Deleted         bool        `json:"deleted" gorm:"not null;default:0"`
	CreatedAt       time.Time   `json:"createdAt"`
	Recordings      []Recording `json:"-" gorm:"table:recordings;foreignKey:channel_name;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	RecordingsCount uint        `json:"recordingsCount"`
	RecordingsSize  uint        `json:"recordingsSize"`
}

type StreamInfo struct {
	IsOnline    bool `json:"isOnline"`
	Url         string
	ChannelName string `json:"channelName"`
}

func (channel *Channel) Create(tags *[]string) (*Channel, error) {
	channel.ChannelName = strings.ToLower(strings.TrimSpace(channel.ChannelName))
	channel.CreatedAt = time.Now()

	if tags != nil {
		str, err := prepareTags(*tags)
		if err != nil {
			return nil, err
		}
		channel.Tags = str
	}

	if err := Db.Create(&channel).Error; err != nil {
		return nil, err
	}

	conf.MakeChannelFolders(channel.ChannelName)

	return channel, nil
}

func (channel *Channel) Update() error {
	return Db.Table("channels").
		Where("channel_name = ?", channel.ChannelName).
		Updates(map[string]interface{}{"display_name": channel.DisplayName, "url": channel.Url, "skip_start": channel.SkipStart}).Error
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

func (channel *Channel) Start() error {
	// Stop any previous recording, restart
	if err := channel.Pause(false); err != nil {
		return err
	}

	url, err := channel.QueryStreamUrl()
	streamInfo[channel.ChannelName] = StreamInfo{IsOnline: url != "", Url: url}
	if err != nil {
		return err
	}
	if url == "" {
		return errors.New("channel offline")
	}

	log.Printf("[Start] Starting '%s' at '%s'", channel.ChannelName, url)
	go utils.ExtractFirstFrame(url, conf.FrameWidth, filepath.Join(conf.AbsoluteDataPath(channel.ChannelName), conf.FrameName))
	go channel.Capture(url, channel.SkipStart)

	return nil
}

func TerminateAll() {
	for channelName, _ := range streams {
		channel := Channel{ChannelName: channelName}
		if err := channel.TerminateProcess(); err != nil {
			log.Printf("Error terminating channel: '%s': %s", channelName, err.Error())
		}
	}
}

// TerminateProcess Interrupt the ffmpeg recording process
// There's maximum one recording job per channel.
func (channel *Channel) TerminateProcess() error {
	// Is current recording at all?
	if cmd, ok := streams[channel.ChannelName]; ok {
		if err := cmd.Process.Signal(os.Interrupt); err != nil && !strings.Contains(err.Error(), "255") {
			log.Printf("[TerminateProcess] Error killing process for '%s': %v", channel.ChannelName, err)
			return err
		} else {
			log.Printf("[TerminateProcess] Killed process: '%s'", channel.ChannelName)
		}
	}

	return nil
}

func (channel *Channel) IsOnline() bool {
	if _, ok := streamInfo[channel.ChannelName]; ok {
		return streamInfo[channel.ChannelName].IsOnline
	}
	return false
}

func (channel *Channel) IsRecording() bool {
	if _, ok := streams[channel.ChannelName]; ok {
		return true
	}
	return false
}

func (channel *Channel) QueryStreamUrl() (string, error) {
	cmd := exec.Command("youtube-dl", "--force-ipv4", "--ignore-errors", "--youtube-skip-dash-manifest", "-f best/bestvideo", "--get-url", channel.Url)
	stdout, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(stdout))

	if err != nil {
		return "", err
	}

	return output, nil
}

//func (channel *Channel) GetStreamUrl() string {
//	if _, ok := streamInfo[channel.ChannelName]; ok {
//		return streamInfo[channel.ChannelName].Data.Url
//	}
//	return ""
//}

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

func ChannelListNotDeleted() ([]*Channel, error) {
	var result []*Channel

	err := Db.Model(&Channel{}).
		Where("deleted = ?", false).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_count", "(SELECT SUM(size) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_size").
		Find(&result).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		log.Println(err)
		return nil, err
	}

	return result, nil
}

func EnabledChannelList() ([]*Channel, error) {
	var result []*Channel

	// Query favourites first
	err := Db.Model(&Channel{}).
		Where("deleted = ?", false).
		Where("is_paused = ?", false).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_count").
		Order("fav desc").
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

// SoftDestroy Delete all recordings and mark channel to delete.
// Often the folder is locked for multiple reasons and can only be deleted on restart.
func (channel *Channel) SoftDestroy() error {
	if err := channel.DestroyAllRecordings(); err != nil {
		log.Printf("Error deleting recordings of channel '%s': %v", channel.ChannelName, err)
		return err
	}

	if err := Db.Table("channels").Where("channel_name = ?", channel.ChannelName).Update("deleted", true).Error; err != nil {
		log.Printf("[SoftDestroy] Error updating channels table: %s", err.Error())
		return err
	}

	return nil
}

func (channel *Channel) Destroy() error {
	// Channel folder
	if err := os.RemoveAll(conf.AbsoluteRecordingsPath(channel.ChannelName)); err != nil && !os.IsNotExist(err) {
		log.Printf("Error deleting channel folder: %v", err)
		return err
	}
	if err := Db.Where("channel_name = ?", channel.ChannelName).Delete(Channel{}).Error; err != nil {
		return err
	}
	return nil
}

func (channel *Channel) DestroyAllRecordings() error {
	var recordings []*Recording
	if err := Db.Where("channel_name = ?", channel.ChannelName).
		Find(&recordings).Error; err != nil {
		return err
	}
	// TODO: Also Cancel running jobs from this channel

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
	if _, ok := streams[channel.ChannelName]; ok {
		return time.Now().Sub(recInfo[channel.ChannelName].CreatedAt).Minutes()
	}
	return 0
}

func (channel *Channel) DestroyData() {
	delete(streams, channel.ChannelName)
	delete(recInfo, channel.ChannelName)
	delete(streamInfo, channel.ChannelName)
}

func (channel *Channel) NewRecording() (Recording, string) {
	filename, timestamp := utils.CreateRecordingName(channel.ChannelName)
	relativePath := filepath.Join("recordings", channel.ChannelName, filename)
	outputFile := filepath.Join(conf.AppCfg.RecordingsAbsolutePath, channel.ChannelName, filename)

	return Recording{ChannelName: channel.ChannelName, Filename: filename, Duration: 0, Bookmark: false, CreatedAt: timestamp, PathRelative: relativePath}, outputFile
}

// Capture Starts and also waits for the stream to end or being killed
func (channel *Channel) Capture(url string, skip uint) error {
	if _, ok := streams[channel.ChannelName]; ok {
		//log.Println("[Channel] Already recording: " + channel.ChannelName)
		return nil
	}

	conf.MakeChannelFolders(channel.ChannelName)
	recording, outputPath := channel.NewRecording()

	log.Println("----------------------------------------Capturing----------------------------------------")
	log.Println("Url: " + url)
	log.Println("to: " + outputPath)

	recInfo[channel.ChannelName] = &recording
	streams[channel.ChannelName] = exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-i", url, "-ss", fmt.Sprintf("%d", skip), "-movflags", "faststart", "-c", "copy", outputPath)
	str := strings.Join([]string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-i", url, "-ss", fmt.Sprintf("%d", skip), "-movflags", "faststart", "-c", "copy", outputPath}, " ")
	log.Printf("Executing: %s", str)

	sterr, _ := streams[channel.ChannelName].StderrPipe()

	if err := streams[channel.ChannelName].Start(); err != nil {
		log.Printf("cmd.Start: %v", err)
		return err
	}

	// Before recording store that the process has started, for recovery
	recJob, err := EnqueueRecordingJob(channel.ChannelName, recording.Filename, outputPath)
	recJob.UpdateInfo(streams[channel.ChannelName].Process.Pid, str)
	if err != nil {
		log.Printf("[Capture] Error enqueuing reccording for: %s/%s: %v", channel.ChannelName, recording.Filename, err)
	}

	if b, err := io.ReadAll(sterr); err != nil {
		log.Printf("[Capture] %s: %v", string(b), err)
	}

	// Wait for process to exit
	if err := streams[channel.ChannelName].Wait(); err != nil && !strings.Contains(err.Error(), "255") {
		log.Printf("[Capture] Wait for process exit '%s' error: %v", channel.ChannelName, err)
		channel.DestroyData()
		channel.DeleteRecordingsFile(recording.Filename)
		recJob.Destroy()
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

	// keep
	if duration > conf.AppCfg.MinRecMin {
		if err := channel.Info().Save("recording"); err != nil {
			log.Printf("[Info] Error adding recording: %v\n", channel.Info())
		}

		// No access to info after this!
		channel.DestroyData()
		recJob.Destroy()

		if job, err := EnqueuePreviewJob(channel.ChannelName, recording.Filename); err != nil {
			log.Printf("[FinishRecording] Error enqueuing job for %v\n", err)
			return err
		} else {
			log.Printf("[FinishRecording] Job enqueued %v\n", job)
		}
	} else { // Throw away
		log.Printf("[FinishRecording] Deleting stream '%s/%s' because it is too short (%vmin)\n", channel.ChannelName, recording.Filename, duration)

		channel.DestroyData()
		recJob.Destroy()

		if err := channel.DeleteRecordingsFile(recording.Filename); err != nil {
			log.Printf("[FinishRecording] Error deleting '%s/%s': %v\n", channel.ChannelName, recording.Filename, err.Error())
			return err
		}
	}

	return nil
}

func (channel *Channel) Info() *Recording {
	return recInfo[channel.ChannelName]
}

func (streamInfo *StreamInfo) Screenshot() error {
	return utils.ExtractFirstFrame(streamInfo.Url, conf.FrameWidth, filepath.Join(conf.AbsoluteDataPath(streamInfo.ChannelName), conf.FrameName))
}

func GetStreamInfo() map[string]StreamInfo {
	return streamInfo
}

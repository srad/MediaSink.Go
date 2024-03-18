package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/astaxie/beego/utils"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/helpers"
	"gorm.io/gorm"
)

// Channel Represent a single stream, that shall be recorded. It can also serve as a folder for videos.
type Channel struct {
	ChannelId   uint      `json:"channelId" gorm:"autoIncrement;primaryKey;column:channel_id" extensions:"!x-nullable"`
	ChannelName string    `json:"channelName" gorm:"unique;not null;" extensions:"!x-nullable"`
	DisplayName string    `json:"displayName" gorm:"not null;default:''" extensions:"!x-nullable"`
	SkipStart   uint      `json:"skipStart" gorm:"not null;default:0" extensions:"!x-nullable"`
	Url         string    `json:"url" gorm:"not null;default:''" extensions:"!x-nullable"`
	Tags        string    `json:"tags" gorm:"not null;default:''" extensions:"!x-nullable"`
	Fav         bool      `json:"fav" gorm:"index:idx_fav,not null" extensions:"!x-nullable"`
	IsPaused    bool      `json:"isPaused" gorm:"not null,default:false" extensions:"!x-nullable"`
	Deleted     bool      `json:"deleted" gorm:"not null,default:false" extensions:"!x-nullable"`
	CreatedAt   time.Time `json:"createdAt" extensions:"!x-nullable"`

	// Only for query result.
	RecordingsCount uint `json:"recordingsCount" gorm:"" extensions:"!x-nullable"`
	RecordingsSize  uint `json:"recordingsSize" gorm:"" extensions:"!x-nullable"`

	// 1:n
	Recordings []Recording `json:"recordings" gorm:"foreignKey:channel_id;constraint:OnDelete:CASCADE;foreignKey:ChannelId"`
}

// ChannelFile This type is used to store a JSON file in each channel folder to restore the database, if it is absent.
type ChannelFile struct {
	ChannelName string    `json:"channelName" extensions:"!x-nullable"`
	DisplayName string    `json:"displayName" extensions:"!x-nullable"`
	SkipStart   uint      `json:"skipStart" extensions:"!x-nullable"`
	Url         string    `json:"url" extensions:"!x-nullable"`
	Tags        string    `json:"tags" extensions:"!x-nullable"`
	Fav         bool      `json:"fav" extensions:"!x-nullable"`
	CreatedAt   time.Time `json:"createdAt" extensions:"!x-nullable"`
}

type StreamInfo struct {
	IsOnline      bool   `json:"isOnline" extensions:"!x-nullable"`
	IsTerminating bool   `extensions:"!x-nullable"`
	Url           string `extensions:"!x-nullable"`
	ChannelName   string `json:"channelName" extensions:"!x-nullable"`
}

var (
	recInfo    = make(map[string]*Recording)
	streamInfo = make(map[string]StreamInfo)
	streams    = make(map[string]*exec.Cmd)
	// "tag1,tag2,...
	rTags, _ = regexp.Compile("^[a-z\\-0-9]+(,[a-z\\-0-9]+)*$")
)

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
	channel.WriteJson()

	return channel, nil
}

func (channel *Channel) ExistsJson() bool {
	return utils.FileExists(channel.jsonPath())
}

func (channel *Channel) jsonPath() string {
	return path.Join(conf.AbsoluteChannelPath(channel.ChannelName), "channel.json")
}

func (channel *Channel) ReadJson() (*ChannelFile, error) {
	if data, err := os.ReadFile(channel.jsonPath()); err != nil {
		return nil, err
	} else {
		var content *ChannelFile = &ChannelFile{}
		json.Unmarshal(data, content)
		return content, nil
	}
}

// WriteJson Additionally write a backup of the channel data to a JSON file. This can be used to re-import data from disks.
func (channel *Channel) WriteJson() {
	jsonPath := channel.jsonPath()
	content := &ChannelFile{
		ChannelName: channel.ChannelName,
		DisplayName: channel.DisplayName,
		SkipStart:   channel.SkipStart,
		Url:         channel.Url,
		Tags:        channel.Tags,
		Fav:         channel.Fav,
		CreatedAt:   channel.CreatedAt,
	}
	file, _ := json.MarshalIndent(content, "", " ")
	if err := os.WriteFile(jsonPath, file, 0644); err != nil {
		log.Printf("Error writing channel.json file to: %s", jsonPath)
	}
}

func (channel *Channel) Update() error {
	err := Db.Table("channels").
		Where("channel_name = ?", channel.ChannelName).
		Updates(map[string]interface{}{"display_name": channel.DisplayName, "url": channel.Url, "skip_start": channel.SkipStart}).Error

	if err == nil {
		channel.WriteJson()
	}

	return err
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
		return "", fmt.Errorf("invalid tags: %s", tags)
	}

	return joined, nil
}

func (channel *Channel) Start() error {
	// Stop any previous recording, restart
	if err := channel.Pause(false); err != nil {
		return err
	}

	url, err := channel.QueryStreamUrl()
	streamInfo[channel.ChannelName] = StreamInfo{IsOnline: url != "", Url: url, ChannelName: channel.ChannelName, IsTerminating: false}
	if url == "" {
		// Channel offline
		return fmt.Errorf("no url found for channel '%s'", channel.ChannelName)
	}
	if err != nil {
		return err
	}

	log.Printf("[Start] Starting '%s' at '%s'", channel.ChannelName, url)

	go func() {
		if err := helpers.ExtractFirstFrame(url, conf.FrameWidth, filepath.Join(conf.AbsoluteDataPath(channel.ChannelName), conf.SnapshotFilename)); err != nil {
			log.Printf("Error: %s", err.Error())
		}
	}()

	go func() {
		log.Printf("Start capturing url: %s", url)
		if err := channel.Capture(url, channel.SkipStart); err != nil {
			log.Printf("Error capturing video: %s", err.Error())
		}
	}()

	return nil
}

func TerminateAll() {
	for channelName := range streams {
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
		if info, ok2 := streamInfo[channel.ChannelName]; ok2 {
			streamInfo[channel.ChannelName] = StreamInfo{
				IsOnline:      info.IsOnline,
				IsTerminating: true, // <---------------- only update.
				Url:           info.Url,
				ChannelName:   info.ChannelName,
			}
		}
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

func (channel *Channel) IsTerminating() bool {
	if _, ok := streamInfo[channel.ChannelName]; ok {
		return streamInfo[channel.ChannelName].IsTerminating
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

func GetChannelByName(channelName string) (*Channel, error) {
	var channel Channel
	err := Db.Where("channel_name = ?", channelName).First(&channel).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return &channel, nil
}

func ChannelList() ([]*Channel, error) {
	var result []*Channel

	err := Db.Model(&Channel{}).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_count", "(SELECT SUM(size) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_size").
		Find(&result).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Println(err)
		return nil, err
	}

	return result, nil
}

func ChannelListNotDeleted() ([]*Channel, error) {
	var result []*Channel

	err := Db.Model(&Channel{}).
		Where("channels.deleted = ?", false).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_count", "(SELECT SUM(size) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_size").
		Find(&result).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
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

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
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
	if err := os.RemoveAll(conf.AbsoluteChannelPath(channel.ChannelName)); err != nil && !os.IsNotExist(err) {
		log.Printf("Error deleting channel folder: %v", err)
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
	if err := os.RemoveAll(conf.AbsoluteChannelPath(channel.ChannelName)); err != nil && !os.IsNotExist(err) {
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
	if err := Db.Where("channel_name = ?", channel.ChannelName).Find(&recordings).Error; err != nil {
		log.Printf("No recordings found to destroy for channel %s", channel.ChannelName)
		return err
	}

	if jobs, err := channel.Jobs(); err != nil {
		log.Printf("Error querying all jobs for this channel")
	} else {
		for _, job := range jobs {
			job.Destroy()
		}
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
	filename, timestamp := helpers.CreateRecordingName(channel.ChannelName)
	relativePath := filepath.Join("recordings", channel.ChannelName, filename)
	outputFile := filepath.Join(conf.AppCfg.RecordingsAbsolutePath, channel.ChannelName, filename)

	return Recording{ChannelName: channel.ChannelName, Filename: filename, Duration: 0, Bookmark: false, CreatedAt: timestamp, PathRelative: relativePath}, outputFile
}

// Capture Starts and also waits for the stream to end or being killed
func (channel *Channel) Capture(url string, skip uint) error {
	if _, ok := streams[channel.ChannelName]; ok {
		// log.Println("[Channel] Already recording: " + channel.ChannelName)
		return nil
	}

	conf.MakeChannelFolders(channel.ChannelName)
	recording, outputPath := channel.NewRecording()

	log.Println("----------------------------------------Capturing----------------------------------------")
	log.Println("Url: " + url)
	log.Println("to: " + outputPath)

	recInfo[channel.ChannelName] = &recording
	streams[channel.ChannelName] = exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-i", url, "-ss", fmt.Sprintf("%d", skip), "-movflags", "faststart", "-c", "copy", outputPath)
	cmdStr := strings.Join([]string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-i", url, "-ss", fmt.Sprintf("%d", skip), "-movflags", "faststart", "-c", "copy", outputPath}, " ")
	log.Printf("Executing: %s", cmdStr)

	sterr, _ := streams[channel.ChannelName].StderrPipe()

	if err := streams[channel.ChannelName].Start(); err != nil {
		log.Printf("cmd.Start: %v", err)
		return err
	}

	// Before recording store that the process has started, for recovery
	recJob, err := EnqueueRecordingJob(channel.ChannelName, recording.Filename, outputPath)
	if err != nil {
		log.Printf("[Capture] Error enqueuing reccording for: %s/%s: %v", channel.ChannelName, recording.Filename, err)
	}

	if err := recJob.UpdateInfo(streams[channel.ChannelName].Process.Pid, cmdStr); err != nil {
		log.Printf("[recJob.UpdateInfo]: %s / %v", channel.ChannelName, err)
	}

	if b, err := io.ReadAll(sterr); err != nil {
		log.Printf("[Capture] %s: %v", string(b), err)
	}

	// Wait for process to exit
	if err := streams[channel.ChannelName].Wait(); err != nil && !strings.Contains(err.Error(), "255") {
		log.Printf("[Capture] Wait for process exit '%s' error: %v", channel.ChannelName, err)
		channel.DestroyData()
		if err := channel.DeleteRecordingsFile(recording.Filename); err != nil {
			log.Printf("[Capture] Error deleting recordings file: '%s' error: %v", channel.ChannelName, err)
		}
		if err := recJob.Destroy(); err != nil {
			log.Printf("[Capture] Error destroying recording: '%s' error: %v", channel.ChannelName, err)
		}
		var exiterr *exec.ExitError
		if errors.As(err, &exiterr) {
			log.Printf("[Capture] Exec error: %v", err)
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
	duration := int(time.Now().Sub(channel.Info().CreatedAt).Minutes())

	// keep
	if duration > conf.AppCfg.MinRecMin {
		if err := channel.Info().Save("recording"); err != nil {
			log.Printf("[Info] Error adding recording: %v\n", channel.Info())
		}

		// No access to info after this!
		channel.DestroyData()
		if err := recJob.Destroy(); err != nil {
			log.Printf("[Capture] Error destroying recording: %v\n", err)
		}

		if job, err := recording.EnqueuePreviewJob(); err != nil {
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

func (si *StreamInfo) Screenshot() error {
	return helpers.ExtractFirstFrame(si.Url, conf.FrameWidth, filepath.Join(conf.AbsoluteDataPath(si.ChannelName), conf.SnapshotFilename))
}

func GetStreamInfo() map[string]StreamInfo {
	return streamInfo
}

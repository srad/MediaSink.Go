package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/astaxie/beego/utils"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/helpers"
	"gorm.io/gorm"
)

var (
	recInfo    = make(map[ChannelName]*Recording)
	streamInfo = make(map[ChannelName]StreamInfo)
	// Pointer to process which executed FFMPEG
	streams = make(map[ChannelName]*exec.Cmd)
)

// Channel Represent a single stream, that shall be recorded. It can also serve as a folder for videos.
type Channel struct {
	ChannelId   uint        `json:"channelId" gorm:"autoIncrement;primaryKey;column:channel_id" extensions:"!x-nullable"`
	ChannelName ChannelName `json:"channelName" gorm:"unique;not null;" extensions:"!x-nullable"`
	DisplayName string      `json:"displayName" gorm:"not null;default:''" extensions:"!x-nullable"`
	SkipStart   uint        `json:"skipStart" gorm:"not null;default:0" extensions:"!x-nullable"`
	MinDuration uint        `json:"minDuration" gorm:"not null;default:0" extensions:"!x-nullable"`
	Url         string      `json:"url" gorm:"not null;default:''" extensions:"!x-nullable"`
	Tags        *Tags       `json:"tags" gorm:"type:text;default:null"`
	Fav         bool        `json:"fav" gorm:"index:idx_fav,not null" extensions:"!x-nullable"`
	IsPaused    bool        `json:"isPaused" gorm:"not null,default:false" extensions:"!x-nullable"`
	Deleted     bool        `json:"deleted" gorm:"not null,default:false" extensions:"!x-nullable"`
	CreatedAt   time.Time   `json:"createdAt" gorm:"not null;default:current_timestamp" extensions:"!x-nullable"`

	// Only for query result.
	RecordingsCount uint `json:"recordingsCount" gorm:"<-:false;-:migration" extensions:"!x-nullable"`
	RecordingsSize  uint `json:"recordingsSize" gorm:"<-:false;-:migration" extensions:"!x-nullable"`

	// 1:n
	Recordings []Recording `json:"recordings" gorm:"foreignKey:channel_id;constraint:OnDelete:CASCADE"`
}

type ChannelTagsUpdate struct {
	ChannelId uint  `json:"channelId" extensions:"!x-nullable"`
	Tags      *Tags `json:"tags" extensions:"!x-nullable"`
}

// ChannelFile This type is used to store a JSON file in each channel folder to restore the models, if it is absent.
type ChannelFile struct {
	ChannelId   uint      `json:"channelId" extensions:"!x-nullable"`
	ChannelName string    `json:"channelName" extensions:"!x-nullable"`
	DisplayName string    `json:"displayName" extensions:"!x-nullable"`
	SkipStart   uint      `json:"skipStart" extensions:"!x-nullable"`
	MinDuration uint      `json:"minDuration" extensions:"!x-nullable"`
	Url         string    `json:"url" extensions:"!x-nullable"`
	Tags        *Tags     `json:"tags"`
	Fav         bool      `json:"fav" extensions:"!x-nullable"`
	CreatedAt   time.Time `json:"createdAt" extensions:"!x-nullable"`
}

func (channel *Channel) Create() error {
	channel.Url = strings.TrimSpace(channel.Url)

	if err := Db.Create(&channel).Error; err != nil {
		return err
	}

	if err := channel.ChannelName.MkDir(); err != nil {
		return err
	}
	//channel.WriteJson()

	return nil
}

func (channel *Channel) ExistsJson() bool {
	return utils.FileExists(channel.jsonPath())
}

func (channel *Channel) jsonPath() string {
	return path.Join(channel.ChannelName.AbsoluteChannelPath(), "channel.json")
}

func (channel *Channel) ReadJson() (*ChannelFile, error) {
	if data, err := os.ReadFile(channel.jsonPath()); err != nil {
		return nil, err
	} else {
		var content = &ChannelFile{}
		err := json.Unmarshal(data, content)
		return content, err
	}
}

// WriteJson Additionally write a backup of the channel data to a JSON file. This can be used to re-import data from disks.
//func (channel *Channel) WriteJson() {
//	jsonPath := channel.jsonPath()
//	content := &ChannelFile{
//		ChannelId:   channel.ChannelId,
//		ChannelName: channel.ChannelName,
//		DisplayName: channel.DisplayName,
//		SkipStart:   channel.SkipStart,
//		MinDuration: channel.MinDuration,
//		Url:         channel.Url,
//		Tags:        channel.Tags,
//		Fav:         channel.Fav,
//	}
//	file, _ := json.MarshalIndent(content, "", " ")
//	if err := os.WriteFile(jsonPath, file, 0644); err != nil {
//		log.Errorf("Error writing channel.json file to: %s", jsonPath)
//	}
//}

func (channel *Channel) Update() error {
	// Validation
	url := strings.TrimSpace(channel.Url)
	displayName := strings.TrimSpace(channel.DisplayName)

	if len(url) == 0 || len(displayName) == 0 {
		return fmt.Errorf("invalid parameters: %v", channel)
	}

	err := Db.Save(&channel).Error

	//if err == nil {
	//	channel.WriteJson()
	//}

	return err
}

func (update *ChannelTagsUpdate) TagChannel() error {
	return Db.Table("channels").
		Where("channel_id = ?", update.ChannelId).
		Update("tags", update.Tags).Error
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

	log.Infof("[Start] Starting '%s' at '%s'", channel.ChannelName, url)

	go func() {
		if err := helpers.ExtractFirstFrame(url, conf.FrameWidth, filepath.Join(channel.ChannelName.AbsoluteChannelDataPath(), SnapshotFilename)); err != nil {
			log.Errorf("Error: %s", err)
		}
	}()

	go func() {
		log.Infof("Start capturing url: %s", url)
		if err := channel.Capture(url, channel.SkipStart); err != nil {
			log.Errorf("Error capturing video: %s", err)
		}
	}()

	return nil
}

func TerminateAll() {
	for channelName := range streams {
		channel := Channel{ChannelName: channelName}
		if err := channel.TerminateProcess(); err != nil {
			log.Errorf("Error terminating channel: '%s': %s", channelName, err)
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
			log.Errorf("[TerminateProcess] Error killing process for '%s': %s", channel.ChannelName, err)
			return err
		} else {
			log.Infof("[TerminateProcess] Killed process: %s", channel.ChannelName)
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
	// We only want to extract the URL, disable all additional text output
	cmd := exec.Command("youtube-dl", "--force-ipv4", "--ignore-errors", "--no-warnings", "--youtube-skip-dash-manifest", "-f best/bestvideo", "--get-url", channel.Url)
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

func GetChannelById(id uint) (*Channel, error) {
	var channel Channel

	err := Db.Model(&Channel{}).
		Where("channels.channel_id = ?", id).
		Preload("Recordings").
		Find(&channel).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return &channel, nil
}

func ChannelList() ([]*Channel, error) {
	var result []*Channel

	err := Db.Model(&Channel{}).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_id = channels.channel_id) recordings_count", "(SELECT SUM(size) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_size").
		Find(&result).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorln(err)
		return nil, err
	}

	return result, nil
}

func ChannelListNotDeleted() ([]*Channel, error) {
	var result []*Channel

	err := Db.Model(&Channel{}).
		Where("channels.deleted = ?", false).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_id = channels.channel_id) recordings_count", "(SELECT SUM(size) FROM recordings WHERE recordings.channel_id = channels.channel_id) recordings_size").
		Find(&result).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorln(err)
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
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_id = channels.channel_id) recordings_count").
		Order("fav desc").
		Find(&result).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorln(err)
		return nil, err
	}

	return result, nil
}

func (channel *Channel) FavChannel() error {
	return Db.Table("channels").
		Where("channel_id = ?", channel.ChannelId).
		Update("fav", true).Error
}

func (channel *Channel) UnFavChannel() error {
	return Db.Table("channels").
		Where("channel_id = ?", channel.ChannelId).
		Update("fav", false).Error
}

// SoftDestroy Delete all recordings and mark channel to delete.
// Often the folder is locked for multiple reasons and can only be deleted on restart.
func (channel *Channel) SoftDestroy() error {
	if err := channel.DestroyAllRecordings(); err != nil {
		log.Errorf("Error deleting recordings of channel '%s': %s", channel.ChannelName, err)
		return err
	}
	if err := os.RemoveAll(channel.ChannelName.AbsoluteChannelPath()); err != nil && !os.IsNotExist(err) {
		log.Errorf("Error deleting channel folder: %s", err)
		return err
	}

	if err := Db.Table("channels").Where("channel_id = ?", channel.ChannelId).Update("deleted", true).Error; err != nil {
		log.Errorf("[SoftDestroy] Error updating channels table: %s", err)
		return err
	}

	return nil
}

func (channel *Channel) Destroy() error {
	// Channel folder
	if err := os.RemoveAll(channel.ChannelName.AbsoluteChannelPath()); err != nil && !os.IsNotExist(err) {
		log.Infof("Error deleting channel folder: %s", err)
		return err
	}
	if err := Db.Where("channel_id = ?", channel.ChannelId).Delete(Channel{}).Error; err != nil {
		return err
	}
	return nil
}

func (channel *Channel) DestroyAllRecordings() error {
	var recordings []*Recording
	if err := Db.Where("channel_id = ?", channel.ChannelId).Find(&recordings).Error; err != nil {
		log.Errorf("No recordings found to destroy for channel %s", channel.ChannelName)
		return err
	}

	if jobs, err := channel.Jobs(); err != nil {
		log.Errorln("Error querying all jobs for this channel")
	} else {
		for _, job := range jobs {
			job.Destroy()
		}
	}

	// TODO: Also Cancel running jobs from this channel
	for _, recording := range recordings {
		if err := recording.Destroy(); err != nil {
			log.Errorf("Error deleting recording %s: %s", recording.Filename, err)
		}
	}

	return nil
}

func (channel *Channel) Pause(pauseVal bool) error {
	if err := Db.Table("channels").
		Where("channel_id = ?", channel.ChannelId).
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

func (channel *Channel) NewRecording(videoType string) (Recording, string) {
	filename, timestamp := channel.ChannelName.MakeRecordingFilename()
	relativePath := filepath.Join(channel.ChannelName.String(), filename)
	outputFile := channel.ChannelName.AbsoluteChannelFilePath(filename)

	return Recording{
			ChannelId:     channel.ChannelId,
			ChannelName:   channel.ChannelName,
			Filename:      filename,
			Bookmark:      false,
			CreatedAt:     timestamp,
			VideoType:     videoType,
			Packets:       0,
			Duration:      0,
			Size:          0,
			BitRate:       0,
			Width:         0,
			Height:        0,
			PathRelative:  relativePath,
			PreviewStripe: nil,
			PreviewVideo:  nil,
			PreviewCover:  nil,
		},
		outputFile
}

// Capture Starts and also waits for the stream to end or being killed
func (channel *Channel) Capture(url string, skip uint) error {
	if _, ok := streams[channel.ChannelName]; ok {
		// log.Println("[Channel] Already recording: " + channel.ChannelName)
		return nil
	}

	// Folder could not be created and does not exist yet.
	if err := channel.ChannelName.MkDir(); err != nil && !os.IsExist(err) {
		return err
	}

	recording, outputPath := channel.NewRecording("recording")

	log.Infoln("----------------------------------------Capturing----------------------------------------")
	log.Infoln("Url: " + url)
	log.Infoln("to: " + outputPath)

	recInfo[channel.ChannelName] = &recording
	streams[channel.ChannelName] = exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-i", url, "-ss", fmt.Sprintf("%d", skip), "-movflags", "faststart", "-c", "copy", outputPath)
	cmdStr := strings.Join([]string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-i", url, "-ss", fmt.Sprintf("%d", skip), "-movflags", "faststart", "-c", "copy", outputPath}, " ")
	log.Infof("Executing: %s", cmdStr)

	sterr, _ := streams[channel.ChannelName].StderrPipe()

	if err := streams[channel.ChannelName].Start(); err != nil {
		log.Errorf("cmd.Start: %s", err)
		return err
	}

	recording.Create()

	// Before recording store that the process has started, for recovery
	recJob, err := recording.EnqueueRecordingJob(outputPath)
	if err != nil {
		log.Errorf("[Capture] Error enqueuing reccording for: %s/%s: %s", channel.ChannelName, recording.Filename, err)
	}

	if err := recJob.UpdateInfo(streams[channel.ChannelName].Process.Pid, cmdStr); err != nil {
		log.Errorf("[recJob.UpdateInfo]: %s / %s", channel.ChannelName, err)
	}

	if b, err := io.ReadAll(sterr); err != nil {
		log.Errorf("[Capture] %s: %s", string(b), err)
	}

	// Wait for process to exit
	if err := streams[channel.ChannelName].Wait(); err != nil && !strings.Contains(err.Error(), "255") {
		log.Errorf("[Capture] Wait for process exit '%s' error: %s", channel.ChannelName, err)
		channel.DestroyData()
		if err := recording.Destroy(); err != nil {
			log.Errorf("[Capture] Error deleting recordings file: '%s' error: %s", channel.ChannelName, err)
		}
		if err := recJob.Destroy(); err != nil {
			log.Errorf("[Capture] Error destroying recording: '%s' error: %s", channel.ChannelName, err)
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
	duration := uint(time.Now().Sub(channel.Info().CreatedAt).Minutes())

	// Query the latest minimum recording duration or set a default of 10min.

	log.Infof("Minimum recording duration for channel %s is %dmin", channel.ChannelName, channel.MinDuration)

	// keep
	if duration > channel.MinDuration {
		if err := channel.Info().Create(); err != nil {
			log.Errorf("[Info] Error adding recording: %v", channel.Info())
		}

		// No access to info after this!
		channel.DestroyData()
		if err := recJob.Destroy(); err != nil {
			log.Errorf("[Capture] Error destroying recording: %s", err)
		}

		if job, err := recording.EnqueuePreviewJob(); err != nil {
			log.Errorf("[FinishRecording] Error enqueuing job for %s", err)
			return err
		} else {
			log.Infof("[FinishRecording] Job enqueued %v\n", job)
		}
	} else { // Throw away
		log.Infof("[FinishRecording] Deleting stream '%s/%s' because it is too short (%vmin)", channel.ChannelName, recording.Filename, duration)

		channel.DestroyData()
		recJob.Destroy()

		if err := recording.Destroy(); err != nil {
			log.Errorf("[FinishRecording] Error deleting '%s/%s': %s", channel.ChannelName, recording.Filename, err)
			return err
		}
	}

	return nil
}

func (channel *Channel) Info() *Recording {
	return recInfo[channel.ChannelName]
}

func NewChannel(channelName ChannelName, displayName, url string) Channel {
	return Channel{
		ChannelName: channelName,
		DisplayName: displayName,
		SkipStart:   0,
		MinDuration: 10,
		Url:         url,
		Tags:        nil,
		Fav:         false,
		IsPaused:    false,
		Deleted:     false,
	}
}

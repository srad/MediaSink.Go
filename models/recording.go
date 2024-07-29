package models

import (
	"errors"
	"fmt"
	"github.com/astaxie/beego/utils"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/helpers"
	"gorm.io/gorm"
)

var (
	UpdatingVideoInfo = false
)

type Recording struct {
	RecordingId uint `json:"recordingId" gorm:"autoIncrement;primaryKey;column:recording_id" extensions:"!x-nullable"`

	Channel     Channel     `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:channel_id;references:channel_id"`
	ChannelId   uint        `json:"channelId" gorm:"not null;default:null" extensions:"!x-nullable"`
	ChannelName ChannelName `json:"channelName" gorm:"not null;default:null" extensions:"!x-nullable"`
	Filename    string      `json:"filename" gorm:"not null;default:null" extensions:"!x-nullable"`
	Bookmark    bool        `json:"bookmark" gorm:"index:idx_bookmark;not null" extensions:"!x-nullable"`
	CreatedAt   time.Time   `json:"createdAt" gorm:"not null;default:null;index" extensions:"!x-nullable"`
	VideoType   string      `json:"videoType" gorm:"default:null;not null" extensions:"!x-nullable"`

	Packets  uint64  `json:"packets" gorm:"default:0;not null" extensions:"!x-nullable"` // Total number of video packets/frames.
	Duration float64 `json:"duration" gorm:"default:0;not null" extensions:"!x-nullable"`
	Size     uint64  `json:"size" gorm:"default:0;not null" extensions:"!x-nullable"`
	BitRate  uint64  `json:"bitRate" gorm:"default:0;not null" extensions:"!x-nullable"`
	Width    uint    `json:"width" gorm:"default:0" extensions:"!x-nullable"`
	Height   uint    `json:"height" gorm:"default:0" extensions:"!x-nullable"`

	PathRelative string `json:"pathRelative" gorm:"default:null;not null"`

	PreviewStripe *string `json:"previewStripe" gorm:"default:null"`
	PreviewVideo  *string `json:"previewVideo" gorm:"default:null"`
	PreviewCover  *string `json:"previewCover" gorm:"default:null"`
	//PreviewScreens []string `json:"previewScreens" gorm:"serializer:json"`
}

func (recording *Recording) FindById() error {
	err := Db.Model(Recording{}).
		Select("recordings.*").
		Where("recordings.recording_id = ?", recording.RecordingId).
		Order("recordings.created_at DESC").
		Find(&recording).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return nil
}

func FavRecording(id uint, fav bool) error {
	return Db.Model(Recording{}).
		Where("recording_id = ?", id).
		Update("bookmark", fav).Error
}

func SortBy(column, order string, limit int) ([]*Recording, error) {
	var recordings []*Recording

	err := Db.Model(Recording{}).
		Order(fmt.Sprintf("recordings.%s %s", column, order)).
		Limit(limit).
		Find(&recordings).Error

	if err != nil {
		return nil, err
	}

	return recordings, nil
}

func FindRandom(limit int) ([]*Recording, error) {
	var recordings []*Recording

	err := Db.Model(Recording{}).
		Order("RANDOM()").
		Limit(limit).
		Find(&recordings).Error

	if err != nil {
		return nil, err
	}

	return recordings, nil
}

func RecordingsList() ([]*Recording, error) {
	var recordings []*Recording

	err := Db.Model(Recording{}).
		Select("recordings.*").
		Find(&recordings).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return recordings, nil
}

func BookmarkList() ([]*Recording, error) {
	var recordings []*Recording
	err := Db.Model(Recording{}).
		Where("bookmark = ?", true).
		Select("recordings.*").Order("recordings.channel_name asc").
		Find(&recordings).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return recordings, nil
}

func (recording *Recording) GetPaths() RecordingPaths {
	return recording.ChannelName.GetRecordingsPaths(recording.Filename)
}

func (recording *Recording) PreviewsExist() bool {
	paths := recording.GetPaths()

	videoExists := utils.FileExists(paths.AbsoluteVideosPath)
	stripeExists := utils.FileExists(paths.AbsoluteStripePath)
	posterExists := utils.FileExists(paths.AbsolutePosterPath)

	return videoExists && stripeExists && posterExists
}

func (recording *Recording) Create() error {
	info, err := recording.GetVideoInfo()
	if err != nil {
		return err
	}

	if recording.VideoType == "" {
		recording.VideoType = "recording"
	}

	recording.Duration = info.Duration
	recording.Size = info.Size
	recording.BitRate = info.BitRate
	recording.Width = info.Width
	recording.Height = info.Height
	recording.Packets = info.PacketCount
	recording.CreatedAt = time.Now()

	log.Infof("Creating recording %v", recording)
	if err := Db.Create(&recording).Error; err != nil {
		return fmt.Errorf("error creating record: %s", err)
	}

	return nil
}

// Destroy Deletes all recording related files, jobs, and database item.
func (recording *Recording) Destroy() error {
	// Try to find and destroy all related items: jobs, file, previews, db entry.

	if err := recording.DestroyJobs(); err != nil {
		log.Errorf("Error destroying jobs: %s", err)
	}

	if err := recording.DeleteFile(); err != nil {
		log.Errorf("Error deleting file: %s", err)
	}

	if err := recording.DestroyPreviews(); err != nil {
		log.Errorf("Error destroying preview: %s", err)
	}

	// Remove from database
	if err := Db.Delete(&Recording{}, "recording_id = ?", recording.RecordingId).Error; err != nil {
		return fmt.Errorf("error deleting recordings of file '%s' from channel '%s': %s", recording.Filename, recording.ChannelName, err)
	}

	return nil
}

func (recording *Recording) DestroyJobs() error {
	if jobs, err := recording.FindJobs(); err == nil {
		for _, job := range *jobs {
			if destroyErr := job.Destroy(); destroyErr != nil {
				log.Errorf("Error destroying job-id: %d", job.JobId)
			}
		}
	}
	return nil
}

func (recording *Recording) DeleteFile() error {
	paths := recording.ChannelName.GetRecordingsPaths(recording.Filename)

	if err := os.Remove(paths.Filepath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error deleting recording: %s", err)
	}

	return nil
}

func (channel *Channel) Filename() string {
	return recInfo[channel.ChannelName].Filename
}

func FindRecording(id uint) (*Recording, error) {
	var recording *Recording
	err := Db.Table("recordings").
		Where("recording_id = ?", id).
		First(&recording).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return recording, nil
}

func (recording *Recording) FindJobs() (*[]Job, error) {
	var jobs *[]Job
	err := Db.Model(&Job{}).
		Where("recording_id = ?", recording.RecordingId).
		Find(&jobs).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return jobs, nil
}

func (recording *Recording) AddIfNotExists() error {
	if err := Db.Where("channel_name = ? AND filename = ?", recording.ChannelName, recording.Filename).
		First(&recording).Error; err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		log.Infof("No recording found, creeting: Recording(channel-id: %d, channel-name: '%s', filename: '%s')", recording.ChannelId, recording.ChannelName, recording.Filename)

		if err := recording.Create(); err != nil {
			return fmt.Errorf("error creating recording '%s'", err)
		} else {
			log.Infof("Created recording %s/%s", recording.ChannelName, recording.Filename)
		}
	}

	return nil
}

func (recording *Recording) GetVideoInfo() (*helpers.FFProbeInfo, error) {
	video := helpers.Video{FilePath: recording.ChannelName.AbsoluteChannelFilePath(recording.Filename)}
	return video.GetVideoInfo()
}

func (recording *Recording) UpdateInfo(info *helpers.FFProbeInfo) error {
	return Db.Updates(&Recording{ChannelName: recording.ChannelName, Filename: recording.Filename, Duration: info.Duration, BitRate: info.BitRate, Size: info.Size, Width: info.Width, Height: info.Height, Packets: info.PacketCount}).Error
}

func (recording *Recording) FilePath() string {
	return recording.ChannelName.AbsoluteChannelFilePath(recording.Filename)
}

func (recording *Recording) DataFolder() string {
	return recording.ChannelName.AbsoluteChannelDataPath()
}

func (recording *Recording) AddPreviews() error {
	paths := recording.ChannelName.GetRecordingsPaths(recording.Filename)

	if err := Db.
		Model(&Recording{}).Where("recording_id = ?", recording.RecordingId).
		Update("preview_video", paths.VideosPath).
		Update("preview_stripe", paths.StripePath).
		Update("preview_cover", paths.CoverPath).Error; err != nil {
		return err
	}

	return nil
}

func (recording *Recording) DestroyPreviews() error {
	paths := recording.ChannelName.GetRecordingsPaths(recording.Filename)

	if err := os.Remove(paths.VideosPath); err != nil && !os.IsNotExist(err) {
		log.Errorf("[DestroyPreviews] Error deleting '%s' from channel '%s': %v", paths.VideosPath, recording.ChannelName, err)
	}
	if err := os.Remove(paths.StripePath); err != nil && !os.IsNotExist(err) {
		log.Errorf("[DestroyPreviews] Error deleting '%s' from channel '%s': %v", paths.StripePath, recording.ChannelName, err)
	}
	if err := os.Remove(paths.CoverPath); err != nil && !os.IsNotExist(err) {
		log.Errorf("[DestroyPreviews] Error deleting '%s' from channel '%s': %v", paths.CoverPath, recording.ChannelName, err)
	}

	err := Db.Model(&Recording{}).
		Where("recording_id = ?", recording.RecordingId).
		First(&recording).Error

	// Nothing found to destroy.
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	if err := Db.
		Model(&Recording{}).
		Where("recording_id = ?", recording.RecordingId).
		Update("path_relative", recording.ChannelName.ChannelPath(recording.Filename)).
		Update("preview_video", nil).
		Update("preview_stripe", nil).
		Update("preview_cover", nil).Error; err != nil {
		return err
	}

	return nil
}

func (recording *Recording) Save() error {
	if err := Db.Model(&recording).Save(&recording).Error; err != nil {
		return err
	}

	return nil
}

func UpdateVideoInfo() error {
	log.Infoln("[Recorder] Updating all recordings info")
	recordings, err := RecordingsList()
	if err != nil {
		log.Errorln(err)
		return err
	}
	UpdatingVideoInfo = true
	count := len(recordings)

	i := 1
	for _, rec := range recordings {
		info, err := rec.GetVideoInfo()
		if err != nil {
			log.Errorf("[UpdateVideoInfo] Error updating video info: %s", err)
			continue
		}

		if err := rec.UpdateInfo(info); err != nil {
			log.Errorf("[Recorder] Error updating video info: %s", err)
			continue
		}
		log.Infof("[Recorder] Updated %s (%d/%d)", rec.Filename, i, count)
		i++
	}

	UpdatingVideoInfo = false

	return nil
}

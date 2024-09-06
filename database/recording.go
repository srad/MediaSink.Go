package database

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

type RecordingId uint

type Recording struct {
	RecordingId RecordingId `json:"recordingId" gorm:"autoIncrement;primaryKey;column:recording_id" extensions:"!x-nullable"`

	Channel     Channel           `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:channel_id;references:channel_id"`
	ChannelId   ChannelId         `json:"channelId" gorm:"not null;default:null" extensions:"!x-nullable"`
	ChannelName ChannelName       `json:"channelName" gorm:"not null;default:null;index:idx_file,unique" extensions:"!x-nullable"`
	Filename    RecordingFileName `json:"filename" gorm:"not null;default:null;index:idx_file,unique" extensions:"!x-nullable"`
	Bookmark    bool              `json:"bookmark" gorm:"index:idx_bookmark;not null" extensions:"!x-nullable"`
	CreatedAt   time.Time         `json:"createdAt" gorm:"not null;default:null;index" extensions:"!x-nullable"`
	VideoType   string            `json:"videoType" gorm:"default:null;not null" extensions:"!x-nullable"`

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

func (id RecordingId) FindById() (*Recording, error) {
	var recording *Recording
	if err := Db.Model(Recording{}).
		Select("recordings.*").
		Where("recordings.recording_id = ?", id).
		Order("recordings.created_at DESC").
		Find(&recording).Error; err != nil {
		return nil, err
	}

	return recording, nil
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

func GetPaths(channelName ChannelName, filename RecordingFileName) RecordingPaths {
	return channelName.GetRecordingsPaths(filename)
}

func PreviewsExist(channelName ChannelName, filename RecordingFileName) bool {
	paths := GetPaths(channelName, filename)

	videoExists := utils.FileExists(paths.AbsoluteVideosPath)
	stripeExists := utils.FileExists(paths.AbsoluteStripePath)
	posterExists := utils.FileExists(paths.AbsolutePosterPath)

	return videoExists && stripeExists && posterExists
}

// CreateStreamRecording Stream recording does not read the video data because it's written life to the disk.
func CreateStreamRecording(id ChannelId, filename RecordingFileName) (*Recording, error) {
	channel, errChannel := id.GetChannelById()
	if errChannel != nil {
		return nil, errChannel
	}

	recording := Recording{
		ChannelId:    id,
		ChannelName:  channel.ChannelName,
		Filename:     filename,
		Bookmark:     false,
		CreatedAt:    time.Now(),
		VideoType:    "stream",
		Packets:      0,
		Duration:     0,
		Size:         0,
		BitRate:      0,
		Width:        0,
		Height:       0,
		PathRelative: channel.ChannelName.ChannelPath(filename),
	}

	if err := Db.Create(&recording).Error; err != nil {
		return nil, fmt.Errorf("error creating record: %s", err)
	}

	return &recording, nil
}

func CreateRecording(channelId ChannelId, filename RecordingFileName, videoType string) (*Recording, error) {
	channel, errChannel := channelId.GetChannelById()
	if errChannel != nil {
		return nil, errChannel
	}

	recording := &Recording{
		ChannelId:    channelId,
		ChannelName:  channel.ChannelName,
		Filename:     filename,
		Bookmark:     false,
		CreatedAt:    time.Now(),
		VideoType:    videoType,
		PathRelative: channel.ChannelName.ChannelPath(filename),
	}

	// Check for existing recording.
	if errFind := Db.Model(recording).Where("channel_id = ? AND filename = ?", channelId, filename).FirstOrCreate(recording).Error; errors.Is(errFind, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("error creating record: %s", errFind)
	}

	info, err := GetVideoInfo(recording.ChannelName, recording.Filename)
	if err != nil {
		return nil, fmt.Errorf("[UpdateVideoInfo] Error updating video info: %s", err)
	}

	recording.UpdateInfo(info)

	return recording, nil
}

// Destroy Deletes all recording related files, jobs, and database item.
func (recording *Recording) Destroy() error {
	// Try to find and destroy all related items: jobs, file, previews, db entry.

	if err := recording.ChannelId.DestroyJobs(); err != nil {
		log.Errorf("Error destroying jobs: %s", err)
	}

	if err := DeleteFile(recording.ChannelName, recording.Filename); err != nil {
		log.Errorf("Error deleting file: %s", err)
	}

	if err := recording.RecordingId.DestroyPreviews(); err != nil {
		log.Errorf("Error destroying preview: %s", err)
	}

	// Remove from database
	if err := Db.Delete(&Recording{}, "recording_id = ?", recording.RecordingId).Error; err != nil {
		return fmt.Errorf("error deleting recordings of file '%s' from channel '%s': %s", recording.Filename, recording.ChannelName, err)
	}

	return nil
}

func (channelId ChannelId) DestroyJobs() error {
	if jobs, err := channelId.FindJobs(); err == nil {
		for _, job := range *jobs {
			if destroyErr := job.Destroy(); destroyErr != nil {
				log.Errorf("Error destroying job-id: %d", job.JobId)
			}
		}
	}
	return nil
}

func DeleteFile(channelName ChannelName, filename RecordingFileName) error {
	paths := channelName.GetRecordingsPaths(filename)

	if err := os.Remove(paths.Filepath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error deleting recording: %s", err)
	}

	return nil
}

func (recordingId RecordingId) FindRecordingById() (*Recording, error) {
	var recording *Recording
	err := Db.Table("recordings").
		Where("recording_id = ?", recordingId).
		First(&recording).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return recording, nil
}

func FindRecording(channelId ChannelId, filename RecordingFileName) (*Recording, error) {
	var recording *Recording
	err := Db.Table("recordings").
		Where("channel_id = ? AND filename = ?", channelId, filename).
		First(&recording).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return recording, nil
}

func (channelId ChannelId) FindJobs() (*[]Job, error) {
	var jobs *[]Job
	err := Db.Model(&Job{}).
		Where("channel_id = ?", channelId).
		Find(&jobs).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return jobs, nil
}

func AddIfNotExists(channelId ChannelId, channelName ChannelName, filename RecordingFileName) (*Recording, error) {
	var recording *Recording

	err := Db.Model(Recording{}).
		Where("channel_name = ? AND filename = ?", channelName, filename).
		First(&recording).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Infof("No recording found, creating: Recording(channel-name: %s, filename: '%s')", channelName, filename)

		if created, errCreate := CreateRecording(channelId, filename, "recording"); errCreate != nil {
			return nil, fmt.Errorf("error creating recording '%s'", errCreate)
		} else {
			log.Infof("Created recording %s/%s", channelName, filename)
			return created, nil
		}
	}

	return recording, err
}

func GetVideoInfo(channelName ChannelName, filename RecordingFileName) (*helpers.FFProbeInfo, error) {
	video := helpers.Video{FilePath: channelName.AbsoluteChannelFilePath(filename)}
	return video.GetVideoInfo()
}

func (recording *Recording) UpdateInfo(info *helpers.FFProbeInfo) error {
	return Db.Model(recording).Where("recording_id = ?", recording.RecordingId).Updates(&Recording{ChannelName: recording.ChannelName, Filename: recording.Filename, Duration: info.Duration, BitRate: info.BitRate, Size: info.Size, Width: info.Width, Height: info.Height, Packets: info.PacketCount}).Error
}

func (recording *Recording) AbsoluteFilePath() string {
	return recording.ChannelName.AbsoluteChannelFilePath(recording.Filename)
}

func (recording *Recording) DataFolder() string {
	return recording.ChannelName.AbsoluteChannelDataPath()
}

func (recordingId RecordingId) AddPreviews() error {
	recording, err := recordingId.FindRecordingById()
	if err != nil {
		return err
	}

	paths := recording.ChannelName.GetRecordingsPaths(recording.Filename)

	updates := map[string]interface{}{"preview_video": paths.VideosPath, "preview_stripe": paths.StripePath, "preview_cover": paths.CoverPath}

	if err := Db.Model(Recording{}).Where("recording_id = ?", recordingId).Updates(updates).Error; err != nil {
		return err
	} else {
		log.Infof("Updated preview for record %d", recordingId)
	}

	return nil
}

func (recordingId RecordingId) DestroyPreviews() error {
	recording, err := recordingId.FindRecordingById()
	if err != nil {
		return err
	}

	paths := recording.ChannelName.GetRecordingsPaths(recording.Filename)

	if err := os.Remove(paths.VideosPath); err != nil && !os.IsNotExist(err) {
		log.Errorf("error deleting '%s' from channel '%s': %v", paths.VideosPath, recording.ChannelName, err)
	}
	if err := os.Remove(paths.StripePath); err != nil && !os.IsNotExist(err) {
		log.Errorf("error deleting '%s' from channel '%s': %v", paths.StripePath, recording.ChannelName, err)
	}
	if err := os.Remove(paths.CoverPath); err != nil && !os.IsNotExist(err) {
		log.Errorf("error deleting '%s' from channel '%s': %v", paths.CoverPath, recording.ChannelName, err)
	}

	err = Db.Model(&Recording{}).
		Where("recording_id = ?", recordingId).
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

func (id RecordingId) UpdateVideoType(videoType string) error {
	return Db.Model(Recording{}).Where("recording_id = ?", id).Update("video_type", videoType).Error
}

func (recording *Recording) Save() error {
	return Db.Model(&recording).Save(&recording).Error
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
		info, err := GetVideoInfo(rec.ChannelName, rec.Filename)
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

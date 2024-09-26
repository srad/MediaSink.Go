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
	if id == 0 {
		return nil, fmt.Errorf("invalid recording id %d", id)
	}

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

func (id RecordingId) Exists() error {
	var count int64 = 0
	if err := Db.Model(Recording{}).Where("recordings.recording_id = ?", id).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return nil
	}
	return errors.New("record not exists")
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
func CreateStreamRecording(channelId ChannelId, filename RecordingFileName) (*Recording, error) {
	channel, errChannel := GetChannelById(channelId)
	if errChannel != nil {
		return nil, errChannel
	}

	recording := Recording{
		ChannelId:    channelId,
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
	channel, errChannel := GetChannelById(channelId)
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

func DestroyJobs(id RecordingId) error {
	var jobs *[]Job
	err := Db.Model(&Job{}).
		Where("recording_id = ?", id).
		Find(&jobs).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	for _, job := range *jobs {
		if err := DeleteJob(job.JobId); err != nil {
			log.Warnln(err)
		}
	}

	return nil
}

// DestroyRecording Deletes all recording related files, jobs, and database item.
func DestroyRecording(id RecordingId) error {
	rec, err := id.FindById()
	if err != nil {
		return err
	}

	// Try to find and destroy all related items: jobs, file, previews, db entry.

	var err1, err2, err3, err4 error

	err1 = DestroyJobs(id)
	err2 = DeleteFile(rec.ChannelName, rec.Filename)
	err3 = DestroyPreviews(rec.RecordingId)

	// Remove from database
	if err := Db.Delete(&Recording{}, "recording_id = ?", rec.RecordingId).Error; err != nil {
		err4 = fmt.Errorf("error deleting recordings of file '%s' from channel '%s': %s", rec.Filename, rec.ChannelName, err)
	}

	return errors.Join(err1, err2, err3, err4)
}

func DeleteRecordingData(channelName ChannelName, filename RecordingFileName) error {
	var err1, err2, err3 error

	err1 = DeleteFile(channelName, filename)
	err2 = DeletePreviewFiles(channelName, filename)
	if err := Db.Delete(&Recording{}, "channel_name = ? AND filename = ?", channelName, filename).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		err3 = fmt.Errorf("error deleting recordings of file '%s' from channel '%s': %w", filename, channelName, err)
	}

	return errors.Join(err1, err2, err3)
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

func AddPreviewPaths(recordingId RecordingId) error {
	if recordingId == 0 {
		return errors.New("invalid job id")
	}

	recording, err := recordingId.FindRecordingById()
	if err != nil {
		return err
	}

	paths := recording.ChannelName.GetRecordingsPaths(recording.Filename)

	updates := map[string]interface{}{"preview_video": paths.RelativeVideosPath, "preview_stripe": paths.RelativeStripePath, "preview_cover": paths.RelativeCoverPath}

	if err := Db.Model(Recording{}).Where("recording_id = ?", recordingId).Updates(updates).Error; err != nil {
		return err
	} else {
		log.Infof("Updated preview for record %d", recordingId)
	}

	return nil
}

func DestroyPreviews(id RecordingId) error {
	recording, err := id.FindRecordingById()
	if err != nil {
		return err
	}

	var err1, err2 error

	err1 = DeletePreviewFiles(recording.ChannelName, recording.Filename)
	err2 = nilPreviews(recording.ChannelName, recording.RecordingId, recording.Filename)

	return errors.Join(err1, err2)
}

func nilPreviews(channelName ChannelName, recordingId RecordingId, filename RecordingFileName) error {
	if recordingId == 0 {
		return errors.New("invalid recording id")
	}
	if filename == "" {
		return errors.New("invalid filename")
	}

	return Db.
		Model(&Recording{}).
		Where("recording_id = ?", recordingId).
		Update("path_relative", channelName.ChannelPath(filename)).
		Update("preview_video", nil).
		Update("preview_stripe", nil).
		Update("preview_cover", nil).Error
}

func DeletePreviewFiles(channelName ChannelName, filename RecordingFileName) error {
	paths := channelName.GetRecordingsPaths(filename)

	var err1, err2, err3 error
	if err := os.Remove(paths.AbsoluteVideosPath); err != nil && !os.IsNotExist(err) {
		err1 = fmt.Errorf("error deleting '%s' from channel '%s': %w", paths.RelativeVideosPath, channelName, err)
	}
	if err := os.Remove(paths.AbsoluteStripePath); err != nil && !os.IsNotExist(err) {
		err2 = fmt.Errorf("error deleting '%s' from channel '%s': %w", paths.RelativeStripePath, channelName, err)
	}
	if err := os.Remove(paths.AbsolutePosterPath); err != nil && !os.IsNotExist(err) {
		err3 = fmt.Errorf("error deleting '%s' from channel '%s': %w", paths.RelativeCoverPath, channelName, err)
	}

	if err1 != nil || err2 != nil || err3 != nil {
		return errors.Join(err1, err2, err3)
	}

	return nil
}

func (id RecordingId) UpdateVideoType(videoType string) error {
	return Db.Model(Recording{}).Where("recording_id = ?", id).Update("video_type", videoType).Error
}

func (recording *Recording) Save() error {
	return Db.Model(&recording).Save(&recording).Error
}

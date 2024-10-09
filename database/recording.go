package database

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/astaxie/beego/utils"
	"github.com/go-playground/validator/v10"

	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/helpers"
	"gorm.io/gorm"
)

const (
	PreviewStripe PreviewType = "preview-stripe"
	PreviewVideo  PreviewType = "preview-video"
	PreviewCover  PreviewType = "preview-cover"
)

type PreviewType string

type RecordingID uint

type Recording struct {
	RecordingID RecordingID `json:"recordingId" gorm:"autoIncrement;primaryKey;column:recording_id" extensions:"!x-nullable" validate:"gte=0"`

	Channel     Channel           `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:channel_id;references:channel_id"`
	ChannelID   ChannelID         `json:"channelId" gorm:"not null;default:null" extensions:"!x-nullable" validate:"gte=0"`
	ChannelName ChannelName       `json:"channelName" gorm:"not null;default:null;index:idx_file,unique" extensions:"!x-nullable" validate:"required"`
	Filename    RecordingFileName `json:"filename" gorm:"not null;default:null;index:idx_file,unique" extensions:"!x-nullable" validate:"required"`
	Bookmark    bool              `json:"bookmark" gorm:"index:idx_bookmark;not null" extensions:"!x-nullable"`
	CreatedAt   time.Time         `json:"createdAt" gorm:"not null;default:null;index" extensions:"!x-nullable"`
	VideoType   string            `json:"videoType" gorm:"default:null;not null" extensions:"!x-nullable" validate:"required"`

	Packets  uint64  `json:"packets" gorm:"default:0;not null" extensions:"!x-nullable"` // Total number of video packets/frames.
	Duration float64 `json:"duration" gorm:"default:0;not null" extensions:"!x-nullable"`
	Size     uint64  `json:"size" gorm:"default:0;not null" extensions:"!x-nullable"`
	BitRate  uint64  `json:"bitRate" gorm:"default:0;not null" extensions:"!x-nullable"`
	Width    uint    `json:"width" gorm:"default:0" extensions:"!x-nullable"`
	Height   uint    `json:"height" gorm:"default:0" extensions:"!x-nullable"`

	PathRelative string `json:"pathRelative" gorm:"default:null;not null" validate:"required,filepath"`

	PreviewStripe *string `json:"previewStripe" gorm:"default:null"`
	PreviewVideo  *string `json:"previewVideo" gorm:"default:null"`
	PreviewCover  *string `json:"previewCover" gorm:"default:null"`
}

func FindRecordingByID(recordingID RecordingID) (*Recording, error) {
	if recordingID == 0 {
		return nil, fmt.Errorf("invalid recording recordingID %d", recordingID)
	}

	var recording *Recording
	if err := DB.Model(Recording{}).
		Where("recordings.recording_id = ?", recordingID).
		First(&recording).Error; err != nil {
		return nil, err
	}

	return recording, nil
}

func FavRecording(id uint, fav bool) error {
	return DB.Model(Recording{}).
		Where("recording_id = ?", id).
		Update("bookmark", fav).Error
}

func SortBy(column, order string, limit int) ([]*Recording, error) {
	var recordings []*Recording

	err := DB.Model(Recording{}).
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

	err := DB.Model(Recording{}).
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

	err := DB.Model(Recording{}).
		Select("recordings.*").
		Find(&recordings).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return recordings, nil
}

func BookmarkList() ([]*Recording, error) {
	var recordings []*Recording
	err := DB.Model(Recording{}).
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

func PreviewFileExists(channelName ChannelName, filename RecordingFileName, previewType PreviewType) bool {
	paths := GetPaths(channelName, filename)

	switch previewType {
	case PreviewStripe:
		return utils.FileExists(paths.AbsoluteStripePath)
	case PreviewVideo:
		return utils.FileExists(paths.AbsoluteVideosPath)
	case PreviewCover:
		return utils.FileExists(paths.AbsoluteCoverPath)
	}

	return false
}

func CreateRecording(channelId ChannelID, filename RecordingFileName, videoType string) (*Recording, error) {
	channel, errChannel := GetChannelByID(channelId)
	if errChannel != nil {
		return nil, errChannel
	}

	info, err := GetVideoInfo(channel.ChannelName, filename)
	if err != nil {
		return nil, err
	}

	recording := &Recording{
		RecordingID:   0,
		Channel:       Channel{},
		ChannelID:     channelId,
		ChannelName:   channel.ChannelName,
		Filename:      filename,
		Bookmark:      false,
		CreatedAt:     time.Now(),
		VideoType:     videoType,
		Packets:       info.PacketCount,
		Duration:      info.Duration,
		Size:          info.Size,
		BitRate:       info.BitRate,
		Width:         info.Width,
		Height:        info.Height,
		PathRelative:  channel.ChannelName.ChannelPath(filename),
		PreviewStripe: nil,
		PreviewVideo:  nil,
		PreviewCover:  nil,
	}

	// Check for existing recording.
	if errFind := DB.Model(&Recording{}).Where("channel_id = ? AND filename = ?", channelId, filename).FirstOrCreate(&recording).Error; errors.Is(errFind, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("error creating record: %s", errFind)
	}

	return recording, nil
}

func DestroyJobs(id RecordingID) error {
	var jobs *[]Job
	err := DB.Model(&Job{}).
		Where("recording_id = ?", id).
		Find(&jobs).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	for _, job := range *jobs {
		if err := DeleteJob(job.JobID); err != nil {
			log.Warnln(err)
		}
	}

	return nil
}

// DestroyRecording Deletes all recording related files, jobs, and database item.
func (recording *Recording) DestroyRecording() error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(recording); err != nil {
		return fmt.Errorf("invalid recording values: %w", err)
	}

	// Try to find and destroy all related items: jobs, file, previews, db entry.

	var err1, err2, err3, err4 error

	err1 = DestroyJobs(recording.RecordingID)
	err2 = DeleteFile(recording.ChannelName, recording.Filename)
	err3 = recording.DestroyPreviews()

	// Remove from database
	if err := DB.Delete(&Recording{}, "recording_id = ?", recording.RecordingID).Error; err != nil {
		err4 = fmt.Errorf("error deleting recordings of file '%s' from channel '%s': %w", recording.Filename, recording.ChannelName, err)
	}

	return errors.Join(err1, err2, err3, err4)
}

func DeleteRecordingData(channelName ChannelName, filename RecordingFileName) error {
	var err1, err2, err3 error

	err1 = DeleteFile(channelName, filename)
	err2 = DeletePreviewFiles(channelName, filename)
	if err := DB.Delete(&Recording{}, "channel_name = ? AND filename = ?", channelName, filename).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
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

func (recordingID RecordingID) FindRecordingByID() (*Recording, error) {
	var recording *Recording
	err := DB.Table("recordings").
		Where("recording_id = ?", recordingID).
		First(&recording).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return recording, nil
}

func (channelId ChannelID) FindJobs() (*[]Job, error) {
	var jobs *[]Job
	err := DB.Model(&Job{}).
		Where("channel_id = ?", channelId).
		Find(&jobs).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return jobs, nil
}

func AddIfNotExists(channelId ChannelID, channelName ChannelName, filename RecordingFileName) (*Recording, error) {
	var recording *Recording

	err := DB.Model(Recording{}).
		Where("channel_name = ? AND filename = ?", channelName, filename).
		First(&recording).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Infof("No recording found, creating: Recording(channel-name: %s, filename: '%s')", channelName, filename)

		created, errCreate := CreateRecording(channelId, filename, "recording")
		if errCreate != nil {
			return nil, fmt.Errorf("error creating recording '%s'", errCreate)
		}
		log.Infof("Created recording %s/%s", channelName, filename)
		return created, nil
	}

	return recording, err
}

func GetVideoInfo(channelName ChannelName, filename RecordingFileName) (*helpers.FFProbeInfo, error) {
	video := helpers.Video{FilePath: channelName.AbsoluteChannelFilePath(filename)}
	return video.GetVideoInfo()
}

func (recording *Recording) UpdateInfo(info *helpers.FFProbeInfo) error {
	return DB.Model(recording).Where("recording_id = ?", recording.RecordingID).Updates(&Recording{ChannelName: recording.ChannelName, Filename: recording.Filename, Duration: info.Duration, BitRate: info.BitRate, Size: info.Size, Width: info.Width, Height: info.Height, Packets: info.PacketCount}).Error
}

func (recording *Recording) AbsoluteChannelFilepath() string {
	return recording.ChannelName.AbsoluteChannelFilePath(recording.Filename)
}

func (recording *Recording) DataFolder() string {
	return recording.ChannelName.AbsoluteChannelDataPath()
}

func AddPreviewPaths(recordingID RecordingID) error {
	if recordingID == 0 {
		return errors.New("invalid job id")
	}

	recording, err := recordingID.FindRecordingByID()
	if err != nil {
		return err
	}

	paths := recording.ChannelName.GetRecordingsPaths(recording.Filename)

	updates := map[string]interface{}{"preview_video": paths.RelativeVideosPath, "preview_stripe": paths.RelativeStripePath, "preview_cover": paths.RelativeCoverPath}

	if err := DB.Model(Recording{}).Where("recording_id = ?", recordingID).Updates(updates).Error; err != nil {
		return err
	}

	log.Infof("Updated preview for record %d", recordingID)

	return nil
}

func (recording *Recording) DestroyPreviews() error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(recording); err != nil {
		return nil
	}

	var err1, err2 error

	err1 = DeletePreviewFiles(recording.ChannelName, recording.Filename)
	err2 = recording.nilPreviews()

	return errors.Join(err1, err2)
}

func (recording *Recording) DestroyPreview(previewType PreviewType) error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(recording); err != nil {
		return err
	}

	recording, errFind := recording.RecordingID.FindRecordingByID()
	if errFind != nil {
		return errFind
	}

	paths := recording.ChannelName.GetRecordingsPaths(recording.Filename)

	switch previewType {

	case PreviewStripe:
		if err := os.Remove(paths.AbsoluteStripePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error deleting '%s' from channel '%s': %w", paths.RelativeStripePath, recording.ChannelName, err)
		}

		return DB.
			Model(&Recording{}).
			Where("recording_id = ?", recording.RecordingID).Update("preview_stripe", nil).Error

	case PreviewVideo:
		if err := os.Remove(paths.AbsoluteVideosPath); err != nil && !os.IsNotExist(err) {
			err = fmt.Errorf("error deleting '%s' from channel '%s': %w", paths.RelativeVideosPath, recording.ChannelName, err)
		}
		return DB.
			Model(&Recording{}).
			Where("recording_id = ?", recording.RecordingID).
			Update("preview_video", nil).Error

	case PreviewCover:
		if err := os.Remove(paths.AbsoluteCoverPath); err != nil && !os.IsNotExist(err) {
			err = fmt.Errorf("error deleting '%s' from channel '%s': %w", paths.RelativeCoverPath, paths.RelativeCoverPath, err)
		}
		return DB.
			Model(&Recording{}).
			Where("recording_id = ?", recording.RecordingID).
			Update("preview_cover", nil).Error
	}

	return fmt.Errorf("invalid preview type %s", previewType)
}

func (recording *Recording) UpdatePreviewPath(previewType PreviewType) error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(recording); err != nil {
		return err
	}

	paths := GetPaths(recording.ChannelName, recording.Filename)

	switch previewType {
	case PreviewStripe:
		return DB.Model(&Recording{}).
			Where("recording_id = ?", recording.RecordingID).
			Update("preview_stripe", paths.RelativeStripePath).Error
	case PreviewVideo:
		return DB.Model(&Recording{}).
			Where("recording_id = ?", recording.RecordingID).
			Update("preview_video", paths.RelativeVideosPath).Error
	case PreviewCover:
		return DB.Model(&Recording{}).
			Where("recording_id = ?", recording.RecordingID).
			Update("preview_cover", paths.RelativeCoverPath).Error
	}

	return nil
}

func (recording *Recording) nilPreviews() error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(recording); err != nil {
		return err
	}

	return DB.
		Model(&Recording{}).
		Where("recording_id = ?", recording.RecordingID).
		Update("path_relative", recording.ChannelName.ChannelPath(recording.Filename)).
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
	if err := os.Remove(paths.AbsoluteCoverPath); err != nil && !os.IsNotExist(err) {
		err3 = fmt.Errorf("error deleting '%s' from channel '%s': %w", paths.RelativeCoverPath, channelName, err)
	}

	if err1 != nil || err2 != nil || err3 != nil {
		return errors.Join(err1, err2, err3)
	}

	return nil
}

func (recording *Recording) Save() error {
	return DB.Model(&recording).Save(&recording).Error
}

package database

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/astaxie/beego/utils"

	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/helpers"
	"gorm.io/gorm"
)

type RecordingID uint

type Recording struct {
	RecordingID RecordingID `json:"recordingId" gorm:"autoIncrement;primaryKey;column:recording_id" extensions:"!x-nullable"`

	Channel     Channel           `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:channel_id;references:channel_id"`
	ChannelID   ChannelID         `json:"channelId" gorm:"not null;default:null" extensions:"!x-nullable"`
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
}

func (recordingID RecordingID) FindByID() (*Recording, error) {
	if recordingID == 0 {
		return nil, fmt.Errorf("invalid recording recordingID %d", recordingID)
	}

	var recording *Recording
	if err := DB.Model(Recording{}).
		Select("recordings.*").
		Where("recordings.recording_id = ?", recordingID).
		Order("recordings.created_at DESC").
		Find(&recording).Error; err != nil {
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

func PreviewsExist(channelName ChannelName, filename RecordingFileName) bool {
	paths := GetPaths(channelName, filename)

	videoExists := utils.FileExists(paths.AbsoluteVideosPath)
	stripeExists := utils.FileExists(paths.AbsoluteStripePath)
	posterExists := utils.FileExists(paths.AbsolutePosterPath)

	return videoExists && stripeExists && posterExists
}

// CreateStreamRecording Stream recording does not read the video data because it's written life to the disk.
func CreateStreamRecording(channelId ChannelID, filename RecordingFileName) (*Recording, error) {
	channel, errChannel := GetChannelByID(channelId)
	if errChannel != nil {
		return nil, errChannel
	}

	recording := Recording{
		ChannelID:    channelId,
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

	if err := DB.Create(&recording).Error; err != nil {
		return nil, fmt.Errorf("error creating record: %s", err)
	}

	return &recording, nil
}

func CreateRecording(channelId ChannelID, filename RecordingFileName, videoType string) (*Recording, error) {
	channel, errChannel := GetChannelByID(channelId)
	if errChannel != nil {
		return nil, errChannel
	}

	recording := &Recording{
		ChannelID:    channelId,
		ChannelName:  channel.ChannelName,
		Filename:     filename,
		Bookmark:     false,
		CreatedAt:    time.Now(),
		VideoType:    videoType,
		PathRelative: channel.ChannelName.ChannelPath(filename),
	}

	// Check for existing recording.
	if errFind := DB.Model(recording).Where("channel_id = ? AND filename = ?", channelId, filename).FirstOrCreate(recording).Error; errors.Is(errFind, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("error creating record: %s", errFind)
	}

	info, err := GetVideoInfo(recording.ChannelName, recording.Filename)
	if err != nil {
		return nil, fmt.Errorf("[UpdateVideoInfo] Error updating video info: %s", err)
	}

	if err := recording.UpdateInfo(info); err != nil {
		log.Errorln(err)
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
func DestroyRecording(id RecordingID) error {
	rec, err := id.FindByID()
	if err != nil {
		return err
	}

	// Try to find and destroy all related items: jobs, file, previews, db entry.

	var err1, err2, err3, err4 error

	err1 = DestroyJobs(id)
	err2 = DeleteFile(rec.ChannelName, rec.Filename)
	err3 = DestroyPreviews(rec.RecordingID)

	// Remove from database
	if err := DB.Delete(&Recording{}, "recording_id = ?", rec.RecordingID).Error; err != nil {
		err4 = fmt.Errorf("error deleting recordings of file '%s' from channel '%s': %s", rec.Filename, rec.ChannelName, err)
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

func FindRecording(channelID ChannelID, filename RecordingFileName) (*Recording, error) {
	var recording *Recording
	err := DB.Table("recordings").
		Where("channel_id = ? AND filename = ?", channelID, filename).
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

func (recording *Recording) AbsoluteFilePath() string {
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

func DestroyPreviews(id RecordingID) error {
	recording, err := id.FindRecordingByID()
	if err != nil {
		return err
	}

	var err1, err2 error

	err1 = DeletePreviewFiles(recording.ChannelName, recording.Filename)
	err2 = nilPreviews(recording.ChannelName, recording.RecordingID, recording.Filename)

	return errors.Join(err1, err2)
}

func nilPreviews(channelName ChannelName, recordingID RecordingID, filename RecordingFileName) error {
	if recordingID == 0 {
		return errors.New("invalid recording id")
	}
	if filename == "" {
		return errors.New("invalid filename")
	}

	return DB.
		Model(&Recording{}).
		Where("recording_id = ?", recordingID).
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

func (recordingID RecordingID) UpdateVideoType(videoType string) error {
	return DB.Model(Recording{}).Where("recording_id = ?", recordingID).Update("video_type", videoType).Error
}

func (recording *Recording) Save() error {
	return DB.Model(&recording).Save(&recording).Error
}

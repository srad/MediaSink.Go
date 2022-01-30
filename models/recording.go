package models

import (
	"fmt"
	"github.com/srad/streamsink/media"
	"log"
	"os"
	"time"

	"github.com/srad/streamsink/conf"
	"gorm.io/gorm"
)

type Recording struct {
	Channel     Channel   `json:"channel" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:channel_name"`
	ChannelName string    `json:"channelName" gorm:"primaryKey;not null;default:null"`
	Filename    string    `json:"filename" gorm:"primaryKey;not null;default:null"`
	Bookmark    bool      `json:"bookmark" gorm:"not null"`
	CreatedAt   time.Time `json:"createdAt" gorm:"not null"`

	Duration float64 `json:"duration" gorm:"default:0;not null"`
	Size     uint64  `json:"size" gorm:"default:0;not null"`
	BitRate  uint64  `json:"bitRate" gorm:"default:0;not null"`
	Width    uint    `json:"width" gorm:"default:0"`
	Height   uint    `json:"height" gorm:"default 0"`

	PathRelative  string `json:"pathRelative" gorm:"not null;default:null"`
	PreviewStripe string `json:"previewStripe" gorm:"default:null"`
	PreviewVideo  string `json:"previewVideo" gorm:"default:null"`
}

func FindByName(channelName string) ([]*Recording, error) {
	var recordings []*Recording
	err := Db.Table("recordings").Select("recordings.*").
		Where("recordings.channel_name = ?", channelName).
		Order("recordings.created_at DESC").
		Find(&recordings).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return recordings, nil
}

func FindLatest(limit int) ([]*Recording, error) {
	var recordings []*Recording

	err := Db.Model(Recording{}).
		Select("*").
		Joins("left join channels on recordings.channel_name = channels.channel_name").
		Order("recordings.created_at DESC").
		Limit(limit).
		Scan(&recordings).Error

	if err != nil {
		return nil, err
	}

	return recordings, nil
}

func FindRandom(limit int) ([]*Recording, error) {
	var recordings []*Recording

	err := Db.Model(Recording{}).
		Select("*").
		Joins("left join channels on recordings.channel_name = channels.channel_name").
		Order("RANDOM()").
		Limit(limit).
		Scan(&recordings).Error

	if err != nil {
		return nil, err
	}

	return recordings, nil
}

func FindAll() ([]*Recording, error) {
	var recordings []*Recording

	err := Db.Table("recordings").
		Select("recordings.*").
		Find(&recordings).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return recordings, nil
}

func FindBookmarks() ([]*Recording, error) {
	var recordings []*Recording
	err := Db.Table("recordings").Where("bookmark = 1").
		Select("recordings.*").Order("recordings.channel_name asc").
		Find(&recordings).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return recordings, nil
}

func AddRecording(recording *Recording) error {
	info, err := media.GetVideoInfo(conf.AbsoluteFilepath(recording.ChannelName, recording.Filename))
	if err != nil {
		log.Printf("[AddRecord] Duration error %v for '%s'", err, conf.AbsoluteFilepath(recording.ChannelName, recording.Filename))
		return err
	}

	recording.Duration = info.Duration
	recording.Size = info.Size
	recording.BitRate = info.BitRate
	recording.Width = info.Width
	recording.Height = info.Height

	log.Printf("[AddRecord] Creating %v", recording)
	if err := Db.Create(&recording).Error; err != nil {
		log.Printf("[AddRecord] Error creating record: %v\n", err)
		return err
	}

	//EnqueuePreviewJob(recording.ChannelName, recording.Filename)

	return nil
}

func DeleteRecordings(channelName string) error {
	var recordings []*Recording
	if err := Db.Where("channel_name = ?", channelName).Find(&recordings).Error; err != nil {
		return err
	}

	for _, recording := range recordings {
		if err := DeleteRecording(recording.ChannelName, recording.Filename); err != nil {
			log.Printf("Error deleting recording '%s': %v", err, recording.Filename)
		}
	}

	return nil
}

func DeleteRecording(channelName, filename string) error {
	if err := Db.Delete(&Recording{}, "channel_name = ? AND filename = ?", channelName, filename).Error; err != nil {
		log.Println(fmt.Sprintf("Error deleting recordings of file '%s' from channel '%s': %v", filename, channelName, err))
		return err
	}

	// Remove associated jobs
	if err := Db.Delete(&Job{}, "channel_name = ? AND filename = ?", channelName, filename).Error; err != nil && err != gorm.ErrRecordNotFound {
		log.Println(fmt.Sprintf("Error job for recording of file '%s' from channel '%s': %v", filename, channelName, err))
		return err
	}

	paths := conf.GetRecordingsPaths(channelName, filename)

	if err := os.Remove(paths.Filepath); err != nil {
		log.Println(fmt.Sprintf("Error deleting recording: %v", err))
	}

	return nil
}

func DeleteRecordingsFile(channelName, filename string) error {
	paths := conf.GetRecordingsPaths(channelName, filename)

	if err := os.Remove(paths.Filepath); err != nil {
		log.Println(fmt.Sprintf("Error deleting recording: %v", err))
		return err
	}

	return nil
}

func GetRecord(channelName, filename string) (*Recording, error) {
	var recording *Recording
	err := Db.Table("recordings").
		Where("channel_name = ? AND filename = ?", channelName, filename).
		First(&recording).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return recording, nil
}

func AddIfNotExistsRecording(channelName, filename string) (*Recording, error) {
	rec := Db.First(&Recording{ChannelName: channelName, Filename: filename})
	if rec.Error != nil && rec.Error != gorm.ErrRecordNotFound {
		log.Printf("[AddIfNotExistsRecording] Error %v", rec.Error)
		return nil, rec.Error
	}

	if rec.RowsAffected > 0 {
		log.Printf("[AddIfNotExistsRecording] Recording '%s' already in database", filename)

		return nil, nil
	}

	file := conf.GetAbsoluteRecordingsPath(channelName, filename)
	info, err := media.GetVideoInfo(file)
	if err != nil {
		log.Printf("[AddIfNotExistsRecording] Error reading video information for file '%s': %v", file, err)
	}

	newRec := &Recording{
		ChannelName:  channelName,
		Filename:     filename,
		PathRelative: conf.GetRelativeRecordingsPath(channelName, filename),
		Duration:     info.Duration,
		Width:        info.Width,
		Height:       info.Height,
		Size:         info.Size,
		BitRate:      info.BitRate,
		CreatedAt:    time.Now(),
		Bookmark:     false,
	}

	err = AddRecording(newRec)
	if err != nil {
		log.Printf("[AddIfNotExistsRecording] Error creating: %v", rec.Error)
		return nil, err
	}

	return newRec, nil
}

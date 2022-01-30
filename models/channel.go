package models

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/srad/streamsink/conf"
	"gorm.io/gorm"
)

type Channel struct {
	ChannelId       uint        `json:"channelId" gorm:"primaryKey;not null;default:null"`
	ChannelName     string      `json:"channelName" gorm:"unique;not null;default:null"`
	Url             string      `json:"url" gorm:"unique;not null;default:null"`
	IsPaused        bool        `json:"isPaused" gorm:"not null;"`
	CreatedAt       time.Time   `json:"createdAt"`
	Recordings      []Recording `gorm:"foreignKey:ChannelName;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	RecordingsCount uint        `json:"recordingsCount" gorm:"-"`
}

func AddChannel(channel *Channel) error {
	channel.IsPaused = false
	channel.CreatedAt = time.Now()
	if err := Db.Create(&channel).Error; err != nil {
		return err
	}

	return nil
}

func GetChannel(channelName string) (*Channel, error) {
	var channel Channel
	err := Db.Where("channel_name = ?", channelName).First(&channel).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return &channel, nil
}

func GetChannels() ([]*Channel, error) {
	var result []*Channel
	err := Db.Table("channels").Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_name=channels.channel_name) AS recordings_count").Find(&result).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return result, nil
}

func DeleteChannel(channelName string) error {
	if err := DeleteRecordings(channelName); err != nil {
		log.Println(fmt.Sprintf("Error deleting recordings of channel '%s': %v", channelName, err))
		return err
	}

	if err := Db.Where("channel_name = ?", channelName).Delete(Channel{}).Error; err != nil {
		return err
	}

	if errRemove := os.RemoveAll(conf.AbsoluteRecordingsPath(channelName)); errRemove != nil {
		log.Println(fmt.Sprintf("Error deleting channel folder: %v", errRemove))
		return errRemove
	}

	return nil
}

func Pause(channelName string, pauseVal bool) error {
	if err := Db.Table("channels").Where("channel_name = ?", channelName).Update("is_paused", pauseVal).Error; err != nil {
		return err
	}

	return nil
}

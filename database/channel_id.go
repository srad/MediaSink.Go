package database

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"os"
	"path/filepath"
)

type ChannelId uint

func (channelId ChannelId) TagChannel(tags *Tags) error {
	return Db.Table("channels").
		Where("channel_id = ?", channelId).
		Update("tags", tags).Error
}

func GetChannelById(id ChannelId) (*Channel, error) {
	var channel *Channel

	err := Db.Model(&Channel{}).
		Where("channel_id = ?", id).
		Select("*").
		Find(&channel).Error

	if err != nil {
		return nil, err
	}

	return channel, nil
}

func GetChannelByIdWithRecordings(id ChannelId) (*Channel, error) {
	var channel *Channel

	err := Db.Model(&Channel{}).
		Preload("Recordings").
		Where("channels.channel_id = ?", id).
		Select("*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_id = channels.channel_id) recordings_count", "(SELECT SUM(size) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_size").
		First(&channel).Error

	if err != nil {
		return nil, err
	}

	return channel, nil
}

func (channelId ChannelId) FavChannel() error {
	return Db.Table("channels").
		Where("channel_id = ?", channelId).
		Update("fav", true).Error
}

func (channelId ChannelId) UnFavChannel() error {
	return Db.Table("channels").
		Where("channel_id = ?", channelId).
		Update("fav", false).Error
}

// TryDeleteChannel Delete all recordings and mark channel to delete.
// Often the folder is locked for multiple reasons and can only be deleted on restart.
func TryDeleteChannel(channelId ChannelId) error {
	if channelId == 0 {
		return errors.New("channel id must not be 0")
	}

	var channel Channel
	if err := Db.Model(&Channel{}).
		Where("channel_id = ?", channelId).
		Select("channel_id", "channel_name").
		First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("channel not found")
		}
		return err
	}

	if err := DestroyChannelRecordings(channelId); err != nil {
		log.Errorf("Error deleting recordings of channel '%s': %s", channel.ChannelName, err)
		return err
	}

	// Try remove folder from disk.
	if err := os.RemoveAll(channel.ChannelName.AbsoluteChannelPath()); err != nil && !os.IsNotExist(err) {
		// Folder could not be deleted for some reason.
		// Mark the channel as delete. Folder will be removed on the next program launch.

		log.Errorf("Error deleting channel folder: %s", err)

		if err2 := MarkChannelAsDeleted(channelId); err2 != nil {
			log.Errorln(err2)
		}
		return err
	}

	// Removed channel folder successfully. Not delete from database.
	if err := DeleteChannel(channelId); err != nil {
		return err
	}

	return nil
}

func DeleteChannel(channelId ChannelId) error {
	if channelId == 0 {
		return errors.New("channel id must not be 0")
	}

	return Db.Where("channel_id = ?", channelId).Delete(&Channel{}).Error
}

func MarkChannelAsDeleted(channelId ChannelId) error {
	if err := Db.Model(&Channel{}).
		Where("channel_id = ?", channelId).
		Update("deleted", true).Error; err != nil {
		return fmt.Errorf("error marking channel as deleted: %s", err)
	}

	return nil
}

func DestroyChannel(channelId ChannelId) error {
	channel, err := GetChannelById(channelId)
	if err != nil {
		return err
	}

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

func (channelId ChannelId) PauseChannel(pauseVal bool) error {
	if err := Db.Table("channels").
		Where("channel_id = ?", channelId).
		Update("is_paused", pauseVal).Error; err != nil {
		return err
	}

	return nil
}

func NewRecording(channelId ChannelId, videoType string) (*Recording, string, error) {
	channel, err := GetChannelById(channelId)
	if err != nil {
		return nil, "", err
	}

	filename, timestamp := channel.ChannelName.MakeRecordingFilename()
	relativePath := filepath.Join(channel.ChannelName.String(), filename.String())
	filePath := channel.ChannelName.AbsoluteChannelFilePath(filename)

	return &Recording{
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
		filePath,
		nil
}

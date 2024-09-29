package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ChannelID uint

func (channelId ChannelID) TagChannel(tags *Tags) error {
	return DB.Table("channels").
		Where("channel_id = ?", channelId).
		Update("tags", tags).Error
}

func GetChannelByID(id ChannelID) (*Channel, error) {
	var channel *Channel

	err := DB.Model(&Channel{}).
		Where("channel_id = ?", id).
		Select("*").
		Find(&channel).Error

	if err != nil {
		return nil, err
	}

	return channel, nil
}

func GetChannelByIDWithRecordings(id ChannelID) (*Channel, error) {
	var channel *Channel

	err := DB.Model(&Channel{}).
		Preload("Recordings").
		Where("channels.channel_id = ?", id).
		Select("*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_id = channels.channel_id) recordings_count", "(SELECT SUM(size) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_size").
		First(&channel).Error

	if err != nil {
		return nil, err
	}

	return channel, nil
}

func (channelId ChannelID) FavChannel() error {
	return DB.Table("channels").
		Where("channel_id = ?", channelId).
		Update("fav", true).Error
}

func (channelId ChannelID) UnFavChannel() error {
	return DB.Table("channels").
		Where("channel_id = ?", channelId).
		Update("fav", false).Error
}

// TryDeleteChannel Delete all recordings and mark channel to delete.
// Often the folder is locked for multiple reasons and can only be deleted on restart.
func TryDeleteChannel(channelID ChannelID) error {
	if channelID == 0 {
		return errors.New("channel id must not be 0")
	}

	var channel Channel
	if err := DB.Model(&Channel{}).
		Where("channel_id = ?", channelID).
		Select("channel_id", "channel_name").
		First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("channel not found")
		}
		return err
	}

	if err := DestroyChannelRecordings(channelID); err != nil {
		log.Errorf("Error deleting recordings of channel '%s': %s", channel.ChannelName, err)
		return err
	}

	// Try remove folder from disk.
	if err := os.RemoveAll(channel.ChannelName.AbsoluteChannelPath()); err != nil && !os.IsNotExist(err) {
		// Folder could not be deleted for some reason.
		// Mark the channel as delete. Folder will be removed on the next program launch.

		log.Errorf("Error deleting channel folder: %s", err)

		if err2 := MarkChannelAsDeleted(channelID); err2 != nil {
			log.Errorln(err2)
		}
		return err
	}

	// Removed channel folder successfully. Not delete from database.
	if err := DeleteChannel(channelID); err != nil {
		return err
	}

	return nil
}

func DeleteChannel(channelID ChannelID) error {
	if channelID == 0 {
		return errors.New("channel id must not be 0")
	}

	return DB.Where("channel_id = ?", channelID).Delete(&Channel{}).Error
}

func MarkChannelAsDeleted(channelID ChannelID) error {
	if err := DB.Model(&Channel{}).
		Where("channel_id = ?", channelID).
		Update("deleted", true).Error; err != nil {
		return fmt.Errorf("error marking channel as deleted: %s", err)
	}

	return nil
}

func DestroyChannel(channelID ChannelID) error {
	channel, err := GetChannelByID(channelID)
	if err != nil {
		return err
	}

	// Channel folder
	if err := os.RemoveAll(channel.ChannelName.AbsoluteChannelPath()); err != nil && !os.IsNotExist(err) {
		log.Infof("Error deleting channel folder: %s", err)
		return err
	}
	if err := DB.Where("channel_id = ?", channel.ChannelID).Delete(Channel{}).Error; err != nil {
		return err
	}
	return nil
}

func (channelId ChannelID) PauseChannel(pauseVal bool) error {
	if err := DB.Table("channels").
		Where("channel_id = ?", channelId).
		Update("is_paused", pauseVal).Error; err != nil {
		return err
	}

	return nil
}

func NewRecording(channelID ChannelID, videoType string) (*Recording, string, error) {
	channel, err := GetChannelByID(channelID)
	if err != nil {
		return nil, "", err
	}

	filename, timestamp := channel.ChannelName.MakeRecordingFilename()
	relativePath := filepath.Join(channel.ChannelName.String(), filename.String())
	filePath := channel.ChannelName.AbsoluteChannelFilePath(filename)

	return &Recording{
			ChannelID:     channel.ChannelID,
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

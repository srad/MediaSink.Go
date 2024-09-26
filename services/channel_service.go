package services

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/database"
	"gorm.io/gorm"
	"time"
)

type ChannelInfo struct {
	database.Channel
	IsRecording   bool    `json:"isRecording" extensions:"!x-nullable"`
	IsOnline      bool    `json:"isOnline" extensions:"!x-nullable"`
	IsTerminating bool    `json:"isTerminating" extensions:"!x-nullable"`
	Preview       string  `json:"preview" extensions:"!x-nullable"`
	MinRecording  float64 `json:"minRecording" extensions:"!x-nullable"`
}

// CreateChannel Persistent channel generation.
func CreateChannel(name, displayName string, skipStart, minDuration uint, url string, fav bool, tags *database.Tags, isPaused bool) (*ChannelInfo, error) {
	channel := database.Channel{
		ChannelName: database.ChannelName(name),
		DisplayName: displayName,
		SkipStart:   skipStart,
		MinDuration: minDuration,
		CreatedAt:   time.Now(),
		Url:         url,
		Fav:         fav,
		Tags:        tags,
		IsPaused:    isPaused}

	newChannel, err := database.CreateChannelDetail(channel)

	if err != nil {
		log.Errorln(err)

		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, fmt.Errorf("error creating record: %s", err)
		}
		return nil, err
	}

	info := &ChannelInfo{
		Channel:      *newChannel,
		IsRecording:  false,
		IsOnline:     false,
		Preview:      newChannel.ChannelName.PreviewPath(),
		MinRecording: 0,
	}

	return info, nil
}

// GetChannels Adds additional streaming and recording information to the channel data in the database.
func GetChannels() ([]ChannelInfo, error) {
	channels, err := database.ChannelListNotDeleted()
	if err != nil {
		return nil, err
	}

	response := make([]ChannelInfo, len(channels))

	for index, channel := range channels {
		// Add to each channel current system information
		response[index] = ChannelInfo{
			Channel:       *channel,
			Preview:       channel.ChannelName.PreviewPath(),
			IsOnline:      IsOnline(channel.ChannelId),
			IsTerminating: IsTerminating(channel.ChannelId),
			IsRecording:   IsRecordingStream(channel.ChannelId),
			MinRecording:  GetRecordingMinutes(channel.ChannelId),
		}
	}

	return response, nil
}

// GetChannel Single Channel data with streaming and recording information.
func GetChannel(id uint) (*ChannelInfo, error) {
	channelId := database.ChannelId(id)
	if channel, err := database.GetChannelByIdWithRecordings(channelId); err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	} else {
		return &ChannelInfo{
			Channel:       *channel,
			IsOnline:      IsOnline(channel.ChannelId),
			IsTerminating: IsTerminating(channel.ChannelId),
			IsRecording:   IsRecordingStream(channel.ChannelId),
			MinRecording:  GetRecordingMinutes(channel.ChannelId),
			Preview:       channel.ChannelName.PreviewPath(),
		}, nil
	}

}

func DeleteChannel(channelId database.ChannelId) error {
	var err1, err2 error
	if err := TerminateProcess(channelId); err != nil {
		err1 = fmt.Errorf("process could not be terminated: %s", err.Error())
	}

	if err := database.TryDeleteChannel(channelId); err != nil {
		err2 = fmt.Errorf("channel could not be deleted: %s", err.Error())
	}

	err := errors.Join(err1, err2)
	if err == nil {
		log.Infof("Deleted channel %d", channelId)
	}

	return err
}

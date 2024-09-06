package services

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/database"
	"gorm.io/gorm"
	"path/filepath"
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

	// Success
	cfg := conf.Read()

	info := &ChannelInfo{
		Channel:      *newChannel,
		IsRecording:  false,
		IsOnline:     false,
		Preview:      filepath.Join(newChannel.ChannelName.AbsoluteChannelPath(), cfg.DataPath, database.SnapshotFilename),
		MinRecording: 0,
	}

	return info, nil
}

func GetChannels() ([]ChannelInfo, error) {
	channels, err := database.ChannelListNotDeleted()
	if err != nil {
		return nil, err
	}

	response := make([]ChannelInfo, len(channels))

	cfg := conf.Read()

	for index, channel := range channels {
		// Add to each channel current system information
		response[index] = ChannelInfo{
			Channel:       *channel,
			Preview:       filepath.Join(channel.ChannelName.String(), cfg.DataPath, database.SnapshotFilename),
			IsOnline:      IsOnline(channel.ChannelId),
			IsTerminating: IsTerminating(channel.ChannelId),
			IsRecording:   IsRecordingStream(channel.ChannelId),
			MinRecording:  GetRecordingMinutes(channel.ChannelId),
		}
	}

	return response, nil
}

func GetChannel(id uint) (*ChannelInfo, error) {
	channelId := database.ChannelId(id)
	if channel, err := channelId.GetChannelById(); err != nil {
		return nil, fmt.Errorf("[GetChannel] Error getting channel: %s", err)
	} else {
		return &ChannelInfo{
			Channel:       *channel,
			IsOnline:      IsOnline(channel.ChannelId),
			IsTerminating: IsTerminating(channel.ChannelId),
			IsRecording:   IsRecordingStream(channel.ChannelId),
			MinRecording:  GetRecordingMinutes(channel.ChannelId),
		}, nil
	}

}

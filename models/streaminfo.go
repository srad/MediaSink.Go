package models

import (
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/helpers"
	"path/filepath"
)

type StreamInfo struct {
	IsOnline      bool        `json:"isOnline" extensions:"!x-nullable"`
	IsTerminating bool        `extensions:"!x-nullable"`
	Url           string      `extensions:"!x-nullable"`
	ChannelName   ChannelName `json:"channelName" extensions:"!x-nullable"`
}

func (si *StreamInfo) Screenshot() error {
	return helpers.ExtractFirstFrame(si.Url, conf.FrameWidth, filepath.Join(si.ChannelName.AbsoluteChannelDataPath(), SnapshotFilename))
}

func GetStreamInfo() map[ChannelName]StreamInfo {
	return streamInfo
}

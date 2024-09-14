package requests

import "github.com/srad/streamsink/database"

type ChannelRequest struct {
	ChannelName string         `json:"channelName" binding:"required" extensions:"!x-nullable"`
	DisplayName string         `json:"displayName" binding:"required" extensions:"!x-nullable"`
	SkipStart   uint           `json:"skipStart" binding:"required" extensions:"!x-nullable"`
	MinDuration uint           `json:"minDuration" binding:"required" extensions:"!x-nullable"`
	Url         string         `json:"url" binding:"required" extensions:"!x-nullable"`
	IsPaused    bool           `json:"isPaused" binding:"required" extensions:"!x-nullable"`
	Tags        *database.Tags `json:"tags"`
	Fav         bool           `json:"fav"`
	Deleted     bool           `json:"deleted"`
}

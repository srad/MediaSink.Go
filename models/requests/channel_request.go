package requests

import "github.com/srad/mediasink/database"

type ChannelRequest struct {
    ChannelName string         `json:"channelName" extensions:"!x-nullable"`
    DisplayName string         `json:"displayName" extensions:"!x-nullable"`
    SkipStart   uint           `json:"skipStart" extensions:"!x-nullable"`
    MinDuration uint           `json:"minDuration" extensions:"!x-nullable"`
    Url         string         `json:"url" extensions:"!x-nullable"`
    IsPaused    bool           `json:"isPaused" extensions:"!x-nullable"`
    Tags        *database.Tags `json:"tags"`
    Fav         bool           `json:"fav"`
    Deleted     bool           `json:"deleted"`
}

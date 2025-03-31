package requests

import "github.com/srad/mediasink/database"

type ChannelTagsUpdateRequest struct {
    Tags *database.Tags `json:"tags" extensions:"!x-nullable"`
}

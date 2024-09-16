package requests

import "github.com/srad/streamsink/database"

type ChannelTagsUpdateRequest struct {
	Tags *database.Tags `json:"tags" extensions:"!x-nullable"`
}

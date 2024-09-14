package responses

import "github.com/srad/streamsink/database"

type JobResponse struct {
	Jobs       []*database.Job `json:"jobs"`
	TotalCount int64           `json:"totalCount"`
	Skip       int             `json:"skip"`
	Take       int             `json:"take"`
}

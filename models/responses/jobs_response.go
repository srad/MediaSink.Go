package responses

import "github.com/srad/mediasink/database"

type JobsResponse struct {
    Jobs       []*database.Job `json:"jobs"`
    TotalCount int64           `json:"totalCount" extensions:"!x-nullable"`
    Skip       int             `json:"skip"  extensions:"!x-nullable"`
    Take       int             `json:"take"  extensions:"!x-nullable"`
}

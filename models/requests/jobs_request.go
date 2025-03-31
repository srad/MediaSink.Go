package requests

import "github.com/srad/mediasink/database"

type JobsRequest struct {
    Skip      int                  `json:"skip"`
    Take      int                  `json:"take"`
    States    []database.JobStatus `json:"states" extensions:"!x-nullable"`
    SortOrder database.JobOrder    `json:"sortOrder" extensions:"!x-nullable"`
}

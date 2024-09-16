package requests

import "github.com/srad/streamsink/database"

type JobsRequest struct {
	Skip      int                  `json:"skip"`
	Take      int                  `json:"take"`
	States    []database.JobStatus `json:"states"`
	SortOrder database.JobOrder    `json:"sortOrder"`
}

package responses

type JobWorkerStatus struct {
	IsProcessing bool `json:"isProcessing" extensions:"!x-nullable"`
}

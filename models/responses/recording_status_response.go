package responses

type RecordingStatusResponse struct {
	IsRecording bool `json:"isRecording" extensions:"!x-nullable"`
}

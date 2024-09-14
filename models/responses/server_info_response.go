package responses

type ServerInfoResponse struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

package responses

type ServerInfoResponse struct {
	Version string `json:"version" extensions:"!x-nullable"`
	Commit  string `json:"commit" extensions:"!x-nullable"`
}

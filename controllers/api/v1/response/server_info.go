package response

type ServerInfo struct {
    Version string `json:"version"`
    Commit  string `json:"commit"`
}

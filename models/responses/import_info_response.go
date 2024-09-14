package responses

type ImportInfoResponse struct {
	IsImporting bool `json:"isImporting,omitempty" `
	Progress    int  `json:"progress"`
	Size        int  `json:"size"`
}

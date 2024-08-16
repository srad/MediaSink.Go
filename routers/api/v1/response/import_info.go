package response

type ImportInfo struct {
	IsImporting bool `json:"isImporting,omitempty" `
	Progress    int  `json:"progress"`
	Size        int  `json:"size"`
}

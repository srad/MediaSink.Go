package requests

type CutRequest struct {
	Starts                []string `json:"starts" binding:"required"`
	Ends                  []string `json:"ends" binding:"required"`
	DeleteAfterCompletion bool     `json:"deleteAfterCut" binding:"required"`
}

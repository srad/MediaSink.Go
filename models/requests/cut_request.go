package requests

type CutRequest struct {
	Starts                []string `json:"starts" extensions:"!x-nullable"`
	Ends                  []string `json:"ends" extensions:"!x-nullable"`
	DeleteAfterCompletion bool     `json:"deleteAfterCut" extensions:"!x-nullable"`
}

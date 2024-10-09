package responses

type LoginResponse struct {
	token string `json:"token"  extensions:"!x-nullable"`
}

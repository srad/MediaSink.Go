package responses

type LoginResponse struct {
	Token string `json:"token"  extensions:"!x-nullable"`
}

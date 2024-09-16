package requests

type AuthenticationRequest struct {
	Username string `json:"username" extensions:"!x-nullable"`
	Password string `json:"password" extensions:"!x-nullable"`
}

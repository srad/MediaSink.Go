package entities

type SocketEvent struct {
	Data interface{} `json:"data"`
	Name string      `json:"name"`
}

var (
	SocketChannel = make(chan SocketEvent)
)

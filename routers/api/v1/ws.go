package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var (
	upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		}}
	msg = make(chan SocketMessage)
)

type SocketMessage struct {
	ChannelName string `json:"channelName"`
	Message     string `json:"message"`
	Tag         string `json:"tag"`
}

func WsHandler(c *gin.Context) {
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	defer ws.Close()

	if err != nil {
		log.Println("error get connection")
		log.Fatal(err)
		return
	}

	for {
		select {
		case m := <-msg:
			log.Println(m)
			ws.WriteJSON(m)
		}
		//_, message, err := ws.ReadMessage()
		//if err != nil {
		//	log.Println("error read message")
		//	log.Fatal(err)
		//	return
		//}
		//log.Printf("[Socket] %s", message)
	}
}

func SendMessage(message SocketMessage) {
	msg <- message
}

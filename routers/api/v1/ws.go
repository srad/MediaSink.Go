package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var upGrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	}}

func WsHandler(c *gin.Context) {
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)

	if err != nil {
		log.Println("error get connection")
		log.Fatal(err)
		return
	}

	_, message, err := ws.ReadMessage()
	if err != nil {
		log.Println("error read message")
		log.Fatal(err)
		return
	}
	log.Printf("[Socket] %s", message)
}

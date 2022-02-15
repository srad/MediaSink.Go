package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/srad/streamsink/utils"
	"log"
	"net/http"
	"sync"
)

var (
	upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		}}
	dispatcher = wsDispatcher{}
	msg        = make(chan SocketMessage)
)

type wsDispatcher struct {
	listeners []wsConnection
}

func (d *wsDispatcher) addWs(ws wsConnection) {
	d.listeners = append(d.listeners, ws)
}

func (d *wsDispatcher) notify(msg SocketMessage) {
	for _, l := range d.listeners {
		if err := l.send(msg); err != nil {
			log.Printf("[notify] %v", err)
		}
	}
}

func (p *wsConnection) send(v interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ws.WriteJSON(v)
}

func (d *wsDispatcher) rmWs(ws *websocket.Conn) {
	for i, l := range d.listeners {
		if l.ws == ws {
			d.listeners = append(d.listeners[:i], d.listeners[i+1:]...)
			break
		}
	}
}

type SocketMessage struct {
	Data  map[string]interface{} `json:"data"`
	Event string                 `json:"event"`
}

func NewMessage(event string, data interface{}) SocketMessage {
	return SocketMessage{Event: event, Data: utils.StructToDict(data)}
}

type wsConnection struct {
	ws *websocket.Conn
	mu sync.Mutex
}

func WsListen() {
	for {
		select {
		case m := <-msg:
			dispatcher.notify(m)
		}
	}
}

// WsHandler TODO: Remove *ws from slice in close connection via ws.SetCloseHandler
func WsHandler(c *gin.Context) {
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	defer ws.Close()

	if err != nil {
		log.Printf("error get connection: %v", err)
		return
	}

	dispatcher.addWs(wsConnection{ws: ws})
	ws.SetCloseHandler(func(code int, text string) error {
		log.Println("[WsHandler] Removing ws")
		dispatcher.rmWs(ws)
		return nil
	})

	for {
		msg := &SocketMessage{}
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("[WsHandler] error read message: %v", err)
			return
		}
		log.Printf("[Socket] %v", msg)
	}
}

func SendMessage(message SocketMessage) {
	msg <- message
}

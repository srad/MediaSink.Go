package v1

import (
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/srad/streamsink/entities"
)

// --------------------------------------------------------------------------------------
// This module manages the incoming ws connections and message dispatching
// --------------------------------------------------------------------------------------

var (
	upGrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		return true
	}}
	dispatcher = wsDispatcher{}
)

type wsDispatcher struct {
	listeners []wsConnection
}

func (d *wsDispatcher) addWs(ws wsConnection) {
	d.listeners = append(d.listeners, ws)
}

func (d *wsDispatcher) notify(msg entities.SocketEvent) {
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

func NewSocketEvent(event string, data interface{}) entities.SocketEvent {
	return entities.SocketEvent{Name: event, Data: data}
}

type wsConnection struct {
	ws *websocket.Conn
	mu sync.Mutex
}

func WsListen() {
	for {
		select {
		case m := <-entities.SocketChannel:
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
		msg := &entities.SocketEvent{}
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("[WsHandler] error read message: %v", err)
			return
		}
		log.Printf("[Socket] %v", msg)
	}
}

package network

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

var (
	// Queue size.
	socketChannel = make(chan SocketEvent, 1000)
	upGrader      = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		return true
	}}
	dispatcher = wsDispatcher{}
)

type SocketEvent struct {
	Data interface{} `json:"data"`
	Name string      `json:"name"`
}

// SendSocket Dispatches message asynchronously.
func SendSocket(name string, data interface{}) {
	go SocketEvent{Name: name, Data: data}.channelDispatcher()
}

func (event SocketEvent) channelDispatcher() {
	socketChannel <- event
}

type wsDispatcher struct {
	listeners []wsConnection
}

func (d *wsDispatcher) addWs(ws wsConnection) {
	d.listeners = append(d.listeners, ws)
}

func (d *wsDispatcher) notify(msg SocketEvent) {
	for _, l := range d.listeners {
		if err := l.send(msg); err != nil {
			log.Errorf("[notify] %s", err)
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

type wsConnection struct {
	ws *websocket.Conn
	mu sync.Mutex
}

func WsListen() {
	for {
		select {
		case m := <-socketChannel:
			dispatcher.notify(m)
		}
	}
}

// WsHandler TODO: Remove *ws from slice in close connection via ws.SetCloseHandler
func WsHandler(c *gin.Context) {
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	defer ws.Close()

	if err != nil {
		log.Errorf("error get connection: %s", err)
		return
	}

	dispatcher.addWs(wsConnection{ws: ws})
	ws.SetCloseHandler(func(code int, text string) error {
		log.Infoln("[WsHandler] Removing ws")
		dispatcher.rmWs(ws)
		return nil
	})

	for {
		msg := &SocketEvent{}
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Errorf("[WsHandler] error read message: %s", err)
			return
		}
		log.Infof("[Socket] %v", msg)
	}
}

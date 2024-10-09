package network

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type SocketEventName string

const (
	ChannelOnlineEvent    SocketEventName = "channel:online"
	ChannelOfflineEvent   SocketEventName = "channel:offline"
	ChannelStartEvent     SocketEventName = "channel:start"
	ChannelThumbnailEvent SocketEventName = "channel:thumbnail"

	JobCreateEvent      SocketEventName = "job:create"
	JobStartEvent       SocketEventName = "job:start"
	JobProgressEvent    SocketEventName = "job:progress"
	JobDoneEvent        SocketEventName = "job:done"
	JobActivate         SocketEventName = "job:activate"
	JobDeactivate       SocketEventName = "job:deactivate"
	JobErrorEvent       SocketEventName = "job:error"
	JobPreviewDoneEvent SocketEventName = "job:preview:done"
	JobDeleteEvent      SocketEventName = "job:delete"

	RecordingAddEvent SocketEventName = "recording:add"
)

var (
	// Queue size.
	broadCastChannel = make(chan SocketEvent, 1000)
	upGrader         = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		return true
	}}
	dispatcher = wsDispatcher{}
)

type SocketEvent struct {
	Name SocketEventName `json:"name"`
	Data interface{}     `json:"data"`
}

// BroadCastClients Dispatches message asynchronously.
func BroadCastClients(name SocketEventName, data interface{}) {
	go SocketEvent{Name: name, Data: data}.channelDispatcher()
}

func (event SocketEvent) channelDispatcher() {
	broadCastChannel <- event
}

type wsDispatcher struct {
	listeners []wsConnection
}

func (d *wsDispatcher) addWs(ws wsConnection) {
	d.listeners = append(d.listeners, ws)
}

func (d *wsDispatcher) broadCast(msg SocketEvent) {
	for _, l := range d.listeners {
		if err := l.send(msg); err != nil {
			log.Errorf("[broadCast] %s", err)
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

func (d *wsDispatcher) heartBeat() {
	log.Infoln("Starting websocket heartbeat ...")
	for {
		BroadCastClients("heartbeat", 10)
		time.Sleep(time.Second * 10)
	}
}

func WsListen() {
	go dispatcher.heartBeat()
	for {
		m := <-broadCastChannel
		dispatcher.broadCast(m)
	}
}

// WsHandler TODO: Remove *ws from slice in close connection via ws.SetCloseHandler
func WsHandler(c *gin.Context) {
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("error get connection: %s", err)
		return
	}

	defer ws.Close()

	dispatcher.addWs(wsConnection{ws: ws})
	ws.SetCloseHandler(func(code int, text string) error {
		log.Infoln("[WsHandler] Removing client")
		dispatcher.rmWs(ws)
		return nil
	})

	for {
		msg := &SocketEvent{}
		err := ws.ReadJSON(msg)
		if err != nil {
			log.Errorf("[WsHandler] error read message: %s", err)
			return
		}
		log.Infof("[Socket] %v", msg)
	}
}

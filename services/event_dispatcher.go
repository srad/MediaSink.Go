package services

import (
	"context"
	"log"

	"github.com/srad/streamsink/entities"
	"github.com/srad/streamsink/models"
	"github.com/srad/streamsink/workers"
)

var (
	// MessageChannel                         = make(chan EventMessage)
	dispatchCtx, dispatchCancel = context.WithCancel(context.Background())
)

// StartDispatch Forward all relevant messages to the websocket.
func StartDispatch() {
	go dispatchMessages(dispatchCtx)
}

func StopDispatch() {
	dispatchCancel()
}

func dispatchMessages(ctx context.Context) {
	for {
		select {
		case m := <-workers.JobInfoChannel:
			entities.SocketChannel <- entities.SocketEvent{Name: m.Name, Data: m.Message}
			return
		case m := <-RecorderMessages:
			entities.SocketChannel <- entities.SocketEvent{Name: m.Name, Data: m.Message}
			return
		case m := <-models.JobChannel:
			entities.SocketChannel <- entities.SocketEvent{Name: m.Name, Data: m.Message}
			return
		case <-ctx.Done():
			log.Println("[dispatchWebsocket] stopped")
			return
		}
	}
}

package services

import (
	"context"
	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/entities"
	"github.com/srad/streamsink/workers"
	"log"
)

var (
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
	go dispatchJobInfo(ctx)
	go DispatchRecorder(ctx)
	go dispatchJob(ctx)
}

func dispatchJob(ctx context.Context) {
	for {
		select {
		case m := <-database.JobChannel:
			entities.SocketChannel <- entities.SocketEvent{Name: m.Name, Data: m.Message}
			return
		case <-ctx.Done():
			log.Println("[dispatchMessages] stopped")
			return
		}
	}
}

func dispatchJobInfo(ctx context.Context) {
	for {
		select {
		case m := <-workers.JobInfoChannel:
			entities.SocketChannel <- entities.SocketEvent{Name: m.Name, Data: m.Message}
			return
		case <-ctx.Done():
			log.Println("[dispatchMessages] stopped")
			return
		}
	}
}

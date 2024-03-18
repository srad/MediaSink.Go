package services

import (
	"context"
	"log"

	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/network"
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
	go database.DispatchJob(ctx)
}

func dispatchJobInfo(ctx context.Context) {
	for {
		select {
		case m := <-database.JobInfoChannel:
			network.SendSocket(m.Name, m.Message)
			return
		case <-ctx.Done():
			log.Println("[dispatchMessages] stopped")
			return
		}
	}
}

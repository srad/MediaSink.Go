package services

import (
    "context"

    log "github.com/sirupsen/logrus"
    "github.com/srad/streamsink/models"
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
    go models.DispatchJob(ctx)
}

func dispatchJobInfo(ctx context.Context) {
    for {
        select {
        case m := <-models.JobInfoChannel:
            network.SendSocket(m.Name, m.Message)
            return
        case <-ctx.Done():
            log.Infoln("[dispatchMessages] stopped")
            return
        }
    }
}

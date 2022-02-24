package services

import (
	"context"
	"github.com/srad/streamsink/utils"
	"log"
	"time"

	"github.com/srad/streamsink/models"
)

var (
	isPaused = false
	dispatch = dispatcher{}
	cancel   context.CancelFunc
)

const (
	sleepBetweenRequests = 2 * time.Second
	sleepBetweenRounds   = 10 * time.Second
)

type SocketMessage struct {
	Data  map[string]interface{} `json:"data"`
	Event string                 `json:"event"`
}

func NewMessage(event string, data interface{}) SocketMessage {
	return SocketMessage{Event: event, Data: utils.StructToDict(data)}
}

type dispatcher struct {
	listeners []func(message SocketMessage)
}

type RecorderMessage struct {
	ChannelName string `json:"channelName"`
}

func Subscribe(f func(message SocketMessage)) {
	dispatch.listeners = append(dispatch.listeners, f)
}

func notify(event string, data interface{}) {
	msg := NewMessage(event, data)
	for _, f := range dispatch.listeners {
		f(msg)
	}
}

func iterate(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[iterate] stopped")
			return
		case <-time.After(sleepBetweenRounds):
			checkStreams()
		}
	}
}

func checkStreams() {
	if isPaused {
		return
	}
	channels, err := models.ChannelActiveList()
	if err != nil {
		log.Println(err)
		return
	}
	for _, channel := range channels {
		if isPaused {
			break
		}

		// StopRecorder between each check
		time.Sleep(sleepBetweenRequests)

		url, _ := channel.StreamUrl()
		// Don't spam log if not necessary
		//if err != nil {
		//	log.Printf("Recorder] Get stream url: %v", err)
		//}

		channel.Online(url != "")
		//log.Printf("[Recorder] Checking: channel: '%s' | paused: %t | online: %t | url: '%s'", channel.ChannelName, channel.IsPaused, channel.IsOnline(), url)

		if url != "" {
			notify("channel:online", RecorderMessage{ChannelName: channel.ChannelName})
			log.Println("[Recorder] Extracting first frame of ", channel.ChannelName)
			err := channel.Screenshot(url)
			if err != nil {
				log.Printf("[Recorder] Error extracting first frame of channel | file: %s", channel.ChannelName)
			} else {
				notify("channel:thumbnail", RecorderMessage{ChannelName: channel.ChannelName})
			}
		} else {
			notify("channel:offline", RecorderMessage{ChannelName: channel.ChannelName})
		}

		if url == "" || isPaused || channel.IsPaused {
			continue
		}

		go channel.Capture(url)
		notify("channel:start", RecorderMessage{ChannelName: channel.ChannelName})
	}
}

func IsRecording() bool {
	return !isPaused
}

func StartRecorder() {
	// Create a new context, with its cancellation function
	// from the original context
	ctx, c := context.WithCancel(context.Background())
	cancel = c

	log.Printf("[Recorder] StartRecorder recording thread")
	isPaused = false
	go iterate(ctx)
}

func StopRecorder() error {
	log.Printf("[StopRecorder] Stopping recorder ...")
	// TerminateProcess the go routine for iteration over channels
	isPaused = true
	cancel()
	log.Printf("[StopRecorder] Stopping recorder ...")

	// TerminateProcess each recording individually
	models.TerminateAll()
	log.Printf("[StopRecorder] Terminated streams ...")

	return nil
}

func UpdateVideoInfo() error {
	log.Println("[Recorder] Updating all recordings info")
	recordings, err := models.RecordingList()
	count := len(recordings)
	if err != nil {
		log.Printf("Error %v", err)
		return err
	}

	i := 1
	for _, rec := range recordings {
		info, err := rec.GetVideoInfo()
		if err != nil {
			log.Printf("[UpdateVideoInfo] Error updating video info: %v", err)
			continue
		}

		if err := rec.UpdateInfo(info); err != nil {
			log.Printf("[Recorder] Error updating video info: %v", err.Error())
			continue
		}
		log.Printf("[Recorder] Updated %s (%d/%d)", rec.Filename, i, count)
		i++
	}

	return nil
}

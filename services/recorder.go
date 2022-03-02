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
	requestInterval    = 2 * time.Second
	roundsInterval     = 10 * time.Second
	screenshotInterval = 30 * time.Second
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

func startScreenshotWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[startScreenshotWorker] stopped")
			return
		case <-time.After(screenshotInterval):
			for channelName, info := range models.GetStreamInfo() {
				if info.StreamUrl != "" {
					if err := info.Screenshot(); err != nil {
						log.Printf("[Recorder] Error extracting first frame of channel | file: %s", channelName)
					} else {
						notify("channel:thumbnail", RecorderMessage{ChannelName: channelName})
					}
				}
			}
		}
	}
}

func startStreamWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[startStreamWorker] stopped")
			return
		case <-time.After(roundsInterval):
			checkStreams()
		}
	}
}

// Iterate all enabled channels and query stream url.
func checkStreams() {
	if isPaused {
		return
	}
	channels, err := models.EnabledChannelList()
	if err != nil {
		log.Println(err)
		return
	}
	for _, channel := range channels {
		if isPaused {
			break
		}
		if channel.IsRecording() {
			continue
		}

		url, _ := channel.QueryStreamUrl()
		if url == "" || isPaused || channel.IsPaused {
			notify("channel:offline", RecorderMessage{ChannelName: channel.ChannelName})
			continue
		}

		channel.SetStreamInfo(url)
		notify("channel:online", RecorderMessage{ChannelName: channel.ChannelName})
		log.Println("[Recorder] Extracting first frame of ", channel.ChannelName)

		go channel.Capture(url, channel.SkipStart)
		notify("channel:start", RecorderMessage{ChannelName: channel.ChannelName})

		// StopRecorder between each check
		time.Sleep(requestInterval)
	}
}

func IsRecording() bool {
	return !isPaused
}

func StartRecorder() {
	isPaused = false

	ctx, c := context.WithCancel(context.Background())
	cancel = c

	go startStreamWorker(ctx)
	go startScreenshotWorker(ctx)

	log.Printf("[Recorder] StartRecorder recording thread")
}

func StopRecorder() error {
	log.Printf("[StopRecorder] Stopping recorder ...")

	isPaused = true
	cancel()
	models.TerminateAll()

	return nil
}

func UpdateVideoInfo() error {
	log.Println("[Recorder] Updating all recordings info")
	recordings, err := models.RecordingsList()
	if err != nil {
		log.Printf("Error %v", err)
		return err
	}
	count := len(recordings)

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

func GeneratePosters() error {
	log.Println("[Recorder] Updating all recordings info")
	recordings, err := models.RecordingsList()
	if err != nil {
		log.Printf("Error %v", err)
		return err
	}
	count := len(recordings)

	i := 1
	for _, rec := range recordings {
		filepath := rec.FilePath()
		log.Printf("[] %s (%d/%d)", filepath, i, count)

		if err := models.CreatePreviewPoster(filepath, rec.DataFolder(), utils.FileNameWithoutExtension(rec.Filename)+".jpg"); err != nil {
			log.Printf("[GeneratePosters] Error creating poster: %s", err.Error())
		}
		i++
	}

	return nil
}

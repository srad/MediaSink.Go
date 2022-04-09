package services

import (
	"context"
	"github.com/srad/streamsink/patterns"
	"github.com/srad/streamsink/utils"
	"log"
	"time"

	"github.com/srad/streamsink/models"
)

var (
	isPaused   = false
	cancel     context.CancelFunc
	Dispatcher = &patterns.Dispatcher[RecorderMessage]{}
)

const (
	requestInterval    = 2 * time.Second
	roundsInterval     = 10 * time.Second
	screenshotInterval = 30 * time.Second
)

type RecorderMessage struct {
	ChannelName string `json:"channelName"`
}

func startThumbnailWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[startThumbnailWorker] stopped")
			return
		case <-time.After(screenshotInterval):
			for channelName, info := range models.GetStreamInfo() {
				if info.Url != "" {
					if err := info.Screenshot(); err != nil {
						log.Printf("[Recorder] Error extracting first frame of channel | file: %s", channelName)
					} else {
						Dispatcher.Notify("channel:thumbnail", RecorderMessage{ChannelName: channelName})
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
			return
		}
		if channel.IsRecording() || channel.IsPaused {
			//log.Printf("[checkStreams] Already recording or paused: %s", channel.ChannelName)
			continue
		}

		err := channel.Start()
		log.Printf("%v | %s\n\n", channel, err)

		if err != nil {
			Dispatcher.Notify("channel:offline", RecorderMessage{ChannelName: channel.ChannelName})
		} else {
			Dispatcher.Notify("channel:online", RecorderMessage{ChannelName: channel.ChannelName})
			Dispatcher.Notify("channel:start", RecorderMessage{ChannelName: channel.ChannelName})
		}

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
	go startThumbnailWorker(ctx)

	log.Printf("[Recorder] StartRecorder recording thread")
}

func StopRecorder() error {
	log.Printf("[StopRecorder] Stopping recorder ...")

	isPaused = true
	cancel()
	models.TerminateAll()

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

		if err := utils.CreatePreviewPoster(filepath, rec.DataFolder(), utils.FileNameWithoutExtension(rec.Filename)+".jpg"); err != nil {
			log.Printf("[GeneratePosters] Error creating poster: %s", err.Error())
		}
		i++
	}

	return nil
}

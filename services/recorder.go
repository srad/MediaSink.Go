package services

import (
	"context"
	"log"
	"time"

	"github.com/srad/streamsink/entities"
	"github.com/srad/streamsink/models"
	"github.com/srad/streamsink/utils"
)

var (
	isPaused         = false
	cancel           context.CancelFunc
	RecorderMessages = make(chan entities.EventMessage)
)

const (
	streamCheckBreak         = 2 * time.Second
	breakBetweenCheckStreams = 10 * time.Second
	captureThumbInterval     = 30 * time.Second
)

func startThumbnailWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[startThumbnailWorker] stopped")
			return
		case <-time.After(captureThumbInterval):
			for channelName, info := range models.GetStreamInfo() {
				if info.Url != "" {
					if err := info.Screenshot(); err != nil {
						log.Printf("[Recorder] Error extracting first frame of channel | file: %s", channelName)
					} else {
						RecorderMessages <- entities.EventMessage{Name: "channel:thumbnail", Message: channelName}
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
		case <-time.After(breakBetweenCheckStreams):
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

		// Get the current database value, in case it case been updated meanwhile.
		currentChannel, err := models.GetChannelByName(channel.ChannelName)

		if err != nil {
			log.Printf("[checkStreams] Error channel %s: %s", channel.ChannelName, err)
			continue
		}

		if currentChannel.IsRecording() || currentChannel.IsPaused {
			log.Printf("[checkStreams] Already recording or paused: %s", channel.ChannelName)
			continue
		}

		if err := channel.Start(); err != nil {
			// log.Printf("[checkStreams] Start error: %v | %s\n", channel, err)
			RecorderMessages <- entities.EventMessage{Name: "channel:offline", Message: channel.ChannelName}
		} else {
			RecorderMessages <- entities.EventMessage{Name: "channel:online", Message: channel.ChannelName}
			RecorderMessages <- entities.EventMessage{Name: "channel:start", Message: channel.ChannelName}
		}

		// StopRecorder between each check
		time.Sleep(streamCheckBreak)
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

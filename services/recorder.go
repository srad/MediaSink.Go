package services

import (
	"context"
	"log"
	"time"

	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/network"
)

const (
	streamCheckBreak         = 2 * time.Second
	breakBetweenCheckStreams = 10 * time.Second
	captureThumbInterval     = 30 * time.Second
)

var (
	isPaused         = false
	cancel           context.CancelFunc
	recorderMessages = make(chan network.EventMessage, 1000)
)

func SendMessage(event network.EventMessage) {
	go messageSend(event)
}

func messageSend(event network.EventMessage) {
	recorderMessages <- event
}

func DispatchRecorder(ctx context.Context) {
	for {
		select {
		case m := <-recorderMessages:
			network.SendSocket(m.Name, m.Message)
			return
		case <-ctx.Done():
			log.Println("[dispatchMessages] stopped")
			return
		}
	}
}

// startThumbnailWorker Creates in intervals snapshots of the video as a preview.
func startThumbnailWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[startThumbnailWorker] stopped")
			return
		case <-time.After(captureThumbInterval):
			for channelName, info := range database.GetStreamInfo() {
				if info.Url != "" {
					if err := info.Screenshot(); err != nil {
						log.Printf("[Recorder] Error extracting first frame of channel | file: %s", channelName)
					} else {
						SendMessage(network.EventMessage{Name: "channel:thumbnail", Message: channelName})
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
			log.Println("Sleeping between stream checks zzz...")
		}
	}
}

// Iterate all enabled channels and query stream url.
func checkStreams() {
	if isPaused {
		return
	}
	channels, err := database.EnabledChannelList()
	if err != nil {
		log.Println(err)
		return
	}
	for _, channel := range channels {
		if isPaused {
			return
		}

		// Get the current database value, in case it case been updated meanwhile.
		channel, err := database.GetChannelByName(channel.ChannelName)

		if err != nil {
			log.Printf("[checkStreams] Error channel %s: %s", channel.ChannelName, err)
			continue
		}

		if channel.IsRecording() || channel.IsPaused {
			log.Printf("[checkStreams] Already recording or paused: %s", channel.ChannelName)
			continue
		}

		if err := channel.Start(); err != nil {
			// log.Printf("[checkStreams] Start error: %v | %s\n", channel, err)
			SendMessage(network.EventMessage{Name: "channel:offline", Message: channel.ChannelName})
		} else {
			SendMessage(network.EventMessage{Name: "channel:online", Message: channel.ChannelName})
			SendMessage(network.EventMessage{Name: "channel:start", Message: channel.ChannelName})
		}

		log.Println(channel.ChannelName)

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
	database.TerminateAll()

	return nil
}

func GeneratePosters() error {
	log.Println("[Recorder] Updating all recordings info")
	recordings, err := database.RecordingsList()
	if err != nil {
		log.Printf("Error %v", err)
		return err
	}
	count := len(recordings)

	i := 1
	for _, rec := range recordings {
		filepath := rec.FilePath()
		log.Printf("[] %s (%d/%d)", filepath, i, count)

		video := &helpers.Video{FilePath: filepath}

		if err := video.CreatePreviewPoster(rec.DataFolder(), helpers.FileNameWithoutExtension(rec.Filename)+".jpg"); err != nil {
			log.Printf("[GeneratePosters] Error creating poster: %s", err.Error())
		}
		i++
	}

	return nil
}

package services

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
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
	isPaused       = false
	cancelRecorder context.CancelFunc
)

func startStreamWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Infoln("[startStreamWorker] stopped")
			return
		case <-time.After(breakBetweenCheckStreams):
			checkStreams()
			log.Infoln("Sleeping between stream checks zzz...")
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
		log.Errorln(err)
		return
	}

	for _, channel := range channels {
		if isPaused {
			return
		}

		// Get the current models value, in case it case been updated meanwhile.
		if result, err := database.GetChannelById(channel.ChannelId); err != nil {
			log.Errorf("[checkStreams] Error channel %s: %s", channel.ChannelName, err)
			continue
		} else {
			if IsRecordingStream(result.ChannelId) || result.IsPaused {
				continue
			}

			if err := Start(result.ChannelId); err != nil {
				network.BroadCastClients("channel:offline", result.ChannelId)
			} else {
				network.BroadCastClients("channel:online", result.ChannelId)
				network.BroadCastClients("channel:start", result.ChannelId)
			}

			// StopRecorder between each check
			time.Sleep(streamCheckBreak)
		}
	}
}

func IsRecording() bool {
	return !isPaused
}

func StartRecorder() {
	isPaused = false

	ctx, c := context.WithCancel(context.Background())
	cancelRecorder = c

	go startStreamWorker(ctx)
	go startThumbnailWorker(ctx)

	log.Infoln("[Recorder] StartRecorder recording thread")
}

func StopRecorder() error {
	log.Infoln("[StopRecorder] Stopping recorder ...")

	isPaused = true
	cancelRecorder()
	TerminateAll()

	return nil
}

func GeneratePosters() error {
	log.Infoln("[Recorder] Updating all recordings info")
	recordings, err := database.RecordingsList()
	if err != nil {
		log.Errorln(err)
		return err
	}
	count := len(recordings)

	i := 1
	for _, rec := range recordings {
		filepath := rec.AbsoluteFilePath()
		log.Infof("[GeneratePosters] %s (%d/%d)", filepath, i, count)

		video := &helpers.Video{FilePath: filepath}

		if err := video.CreatePreviewPoster(rec.DataFolder(), helpers.FileNameWithoutExtension(rec.Filename.String())+".jpg"); err != nil {
			log.Errorf("[GeneratePosters] Error creating poster: %s", err)
		}
		i++
	}

	return nil
}

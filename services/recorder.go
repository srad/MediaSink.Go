package services

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/models"
	"github.com/srad/streamsink/network"
)

const (
	streamCheckBreak         = 2 * time.Second
	breakBetweenCheckStreams = 10 * time.Second
	captureThumbInterval     = 30 * time.Second
)

var (
	isPaused = false
	cancel   context.CancelFunc
)

// startThumbnailWorker Creates in intervals snapshots of the video as a preview.
func startThumbnailWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Infoln("[startThumbnailWorker] stopped")
			return
		case <-time.After(captureThumbInterval):
			for channelId, info := range models.GetStreamInfo() {
				if info.Url != "" {
					if err := info.Screenshot(); err != nil {
						log.Errorf("[Recorder] Error extracting first frame of channel-id %d: %s", channelId, err)
					} else {
						network.BroadCastClients("channel:thumbnail", channelId)
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
	channels, err := models.EnabledChannelList()
	if err != nil {
		log.Errorln(err)
		return
	}
	for _, channel := range channels {
		if isPaused {
			return
		}

		// Get the current models value, in case it case been updated meanwhile.
		result, err := models.GetChannelByName(channel.ChannelName)

		if err != nil {
			log.Errorf("[checkStreams] Error channel %s: %s", channel.ChannelName, err)
			continue
		}

		if result.ChannelId.IsRecording() || result.IsPaused {
			log.Infof("[checkStreams] Already recording or paused: %s", result.ChannelName)
			continue
		}

		if err := result.ChannelId.Start(); err != nil {
			// log.Printf("[checkStreams] Start error: %v | %s", channel, err)
			network.BroadCastClients("channel:offline", result.ChannelId)
		} else {
			network.BroadCastClients("channel:online", result.ChannelId)
			network.BroadCastClients("channel:start", result.ChannelId)
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

	log.Infoln("[Recorder] StartRecorder recording thread")
}

func StopRecorder() error {
	log.Infoln("[StopRecorder] Stopping recorder ...")

	isPaused = true
	cancel()
	models.TerminateAll()

	return nil
}

func GeneratePosters() error {
	log.Infoln("[Recorder] Updating all recordings info")
	recordings, err := models.RecordingsList()
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

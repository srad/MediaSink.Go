package services

import (
	"log"
	"time"

	"github.com/srad/streamsink/models"
)

var (
	quit  = make(chan bool)
	pause = false
)

const (
	sleepBetweenRequests = 3 * time.Second
	sleepBetweenRounds   = 60 * time.Second
)

type ChannelObserver struct {
	name string
}

func iterate() {
	checkStreams()
	for {
		select {
		case <-quit:
			log.Println("Stopping iteration")
			return
		case <-time.After(sleepBetweenRounds):
			checkStreams()
			break
			// Wait between each round to reduce the chance of API blocking
		}
	}
}

func checkStreams() {
	if pause {
		return
	}
	channels, err := models.ChannelActiveList()
	if err != nil {
		log.Println(err)
		return
	}
	for _, channel := range channels {
		if pause {
			break
		}

		// Pause between each check
		time.Sleep(sleepBetweenRequests)

		url, _ := channel.StreamUrl()
		// Don't spam log if not necessary
		//if err != nil {
		//	log.Printf("Recorder] Get stream url: %v", err)
		//}

		channel.Online(url != "")
		log.Printf("[Recorder] Checking: channel: '%s' | paused: %t | online: %t | url: '%s'", channel.ChannelName, channel.IsPaused, channel.IsOnline(), url)

		if url != "" {
			log.Println("[Recorder] Extracting first frame of ", channel.ChannelName)
			err := channel.Screenshot(url)
			if err != nil {
				log.Printf("[Recorder] Error extracting first frame of channel | file: %s", channel.ChannelName)
			}

			// The quality might change during streaming, due to bandwidth issue, keep updating
			if err := channel.UpdateStreamInfo(url); err != nil {
				log.Printf("[Recorder] Error updating stream info: %v", err)
			}
		}

		if url == "" || pause || channel.IsPaused {
			continue
		}

		go channel.Capture(url)
	}
}

func IsRecording() bool {
	return !pause
}

func Resume() {
	log.Printf("[Recorder] Resume recording thread")
	pause = false
	go iterate()
}

func Pause() {
	log.Printf("[Recorder] Pausing recording thread")
	pause = true
	StopAll()
}

func StopAll() error {
	// TerminateProcess the go routine for iteration over channels
	quit <- true

	// TerminateProcess each recording individually
	channels, err := models.ChannelActiveList()
	if err != nil {
		log.Println(err)
		return err
	}
	for _, channel := range channels {
		err := channel.Stop(false)
		if err != nil {
			log.Printf("[Recorder] Error stopping channel '%s': %v", channel.ChannelName, err)
		}
	}

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

func (s *ChannelObserver) Update(t string) {
	// do something
	println("StockObserver:", s.name, "has been updated,", "received subject string:", t)
}

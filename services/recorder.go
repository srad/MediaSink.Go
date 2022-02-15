package services

import (
	"log"
	"time"

	"github.com/srad/streamsink/models"
)

var (
	pause    = false
	quit     = make(chan bool)
	dispatch = dispatcher{}
)

const (
	sleepBetweenRequests = 2 * time.Second
	sleepBetweenRounds   = 10 * time.Second
)

type dispatcher struct {
	listeners []func(RecorderMessage)
}

type RecorderMessage struct {
	Event       string
	ChannelName string
}

func ObserveRecorder(f func(RecorderMessage)) {
	dispatch.listeners = append(dispatch.listeners, f)
}

func notify(msg RecorderMessage) {
	for _, f := range dispatch.listeners {
		f(msg)
	}
}

func iterate() {
	for {
		select {
		case <-quit:
			log.Println("[iterate] stopped")
			return
		default:
			checkStreams()
			time.Sleep(sleepBetweenRounds)
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
			notify(RecorderMessage{Event: "online", ChannelName: channel.ChannelName})
			log.Println("[Recorder] Extracting first frame of ", channel.ChannelName)
			err := channel.Screenshot(url)
			if err != nil {
				log.Printf("[Recorder] Error extracting first frame of channel | file: %s", channel.ChannelName)
			} else {
				notify(RecorderMessage{Event: "thumbnail", ChannelName: channel.ChannelName})
			}
		} else {
			notify(RecorderMessage{Event: "offline", ChannelName: channel.ChannelName})
		}

		if url == "" || pause || channel.IsPaused {
			continue
		}

		go channel.Capture(url)
		notify(RecorderMessage{Event: "start", ChannelName: channel.ChannelName})
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

func Pause() error {
	// TerminateProcess the go routine for iteration over channels
	pause = true
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

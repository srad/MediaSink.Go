package services

import (
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/database"
)

var (
	isUpdating = false
)

func UpdateVideoInfo() error {
	log.Infoln("[Recorder] Updating all recordings info")
	recordings, err := database.RecordingsList()
	if err != nil {
		log.Errorln(err)
		return err
	}
	isUpdating = true
	count := len(recordings)

	i := 1
	for _, rec := range recordings {
		info, err := database.GetVideoInfo(rec.ChannelName, rec.Filename)
		if err != nil {
			log.Errorf("[UpdateVideoInfo] Error updating video info: %s", err)
			continue
		}

		if err := rec.UpdateInfo(info); err != nil {
			log.Errorf("[Recorder] Error updating video info: %s", err)
			continue
		}
		log.Infof("[Recorder] Updated %s (%d/%d)", rec.Filename, i, count)
		i++
	}

	isUpdating = false

	return nil
}

func IsUpdatingRecordings() bool {
	return isUpdating
}

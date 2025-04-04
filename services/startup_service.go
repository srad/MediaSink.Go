package services

import (
	"github.com/astaxie/beego/utils"
	log "github.com/sirupsen/logrus"
	"github.com/srad/mediasink/database"
	"github.com/srad/mediasink/helpers"
)

func StartUpJobs() {
	log.Infoln("[StartUpJobs] Running startup job ...")

	if err := deleteChannels(); err != nil { // Blocking
		log.Errorf("[DeleteChannels] ChannelList error: %s", err)
	}
	if err := deleteOrphanedRecordings(); err != nil { // Blocking
		log.Errorln(err)
	}
	StartImport()
	go fixOrphanedFiles()
}

func deleteOrphanedRecordings() error {
	recordings, err := database.RecordingsList()
	if err != nil {
		return err
	}

	for _, recording := range recordings {
		filePath := recording.ChannelName.AbsoluteChannelFilePath(recording.Filename)
		if !utils.FileExists(filePath) {
			recording.DestroyRecording()
		}
	}

	return nil
}

func deleteChannels() error {
	channels, err := database.ChannelList()
	if err != nil {
		return err
	}

	for _, channel := range channels {
		if channel.Deleted {
			log.Infof("[DeleteChannels] Deleting channel : %s", channel.ChannelName)
			database.DestroyChannel(channel.ChannelID)
		}
	}

	return nil
}

// fixOrphanedFiles Scans the recording folder and checks if an un-imported file is found on the disk.
// Only uncorrupted files will be imported.
func fixOrphanedFiles() error {
	log.Infoln("Fixing orphaned channels ...")

	// 1. Check if channel exists, otherwise delete.
	channels, err := database.ChannelList()
	if err != nil {
		log.Errorf("[FixOrphanedFiles] ChannelList error: %s", err)
		return err
	}
	for _, channel := range channels {
		if !channel.FolderExists() {
			database.DestroyChannel(channel.ChannelID)
		}
	}

	// 2. Check if recording file within channel exists, otherwise destroy.
	log.Infoln("Fixing orphaned recordings ...")
	recordings, err := database.RecordingsList()

	if err != nil {
		log.Errorf("[FixOrphanedFiles] ChannelList error: %s", err)
		return err
	}

	for _, recording := range recordings {
		log.Infof("Handling channel file %s", recording.AbsoluteChannelFilepath())
		err := helpers.CheckVideo(recording.AbsoluteChannelFilepath())
		if err != nil {
			log.Errorf("The file '%s' is corrupted, deleting from disk ... ", recording.Filename)
			if err := recording.DestroyRecording(); err != nil {
				log.Errorf("Deleted file '%s'", recording.Filename)
			}
		}
	}

	return nil
}

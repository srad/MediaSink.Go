package services

import (
	"errors"
	"fmt"
	"github.com/astaxie/beego/utils"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/helpers"
	"gorm.io/gorm"
	"os"
)

func StartUpJobs() error {
	log.Infoln("[StartUpJobs] Running startup job ...")

	deleteChannels() // Blocking.
	deleteOrphanedRecordings()
	StartImport()
	go fixOrphanedRecordings()

	return nil
}

func deleteOrphanedRecordings() error {
	recordings, err := database.RecordingsList()
	if err != nil {
		return err
	}

	for _, recording := range recordings {
		filePath := recording.ChannelName.AbsoluteChannelFilePath(recording.Filename)
		if !utils.FileExists(filePath) {
			recording.Destroy()
		}
	}

	return nil
}

func deleteChannels() error {
	channels, err := database.ChannelList()
	if err != nil {
		log.Errorf("[DeleteChannels] ChannelList error: %s", err)
		return err
	}

	for _, channel := range channels {
		if channel.Deleted {
			log.Infof("[DeleteChannels] Deleting channel : %s", channel.ChannelName)
			channel.ChannelId.DestroyChannel()
		}
	}

	return nil
}

// FixOrphanedRecordings Go through all open jobs with status "recording" and complete them.
func fixOrphanedRecordings() {
	fixOrphanedFiles()
	fixOrphanedJobs()
}

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
			channel.ChannelId.DestroyChannel()
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
		log.Infof("Handling channel file %s", recording.AbsoluteFilePath())
		err := helpers.CheckVideo(recording.AbsoluteFilePath())
		if err != nil {
			log.Errorf("The file '%s' is corrupted, deleting from disk ... ", recording.Filename)
			if err := recording.Destroy(); err != nil {
				log.Errorf("Deleted file '%s'", recording.Filename)
			}
		}
	}

	return nil
}

func fixOrphanedJobs() {
	log.Infoln("Fixing orphaned jobs ...")
	jobs, err := database.GetJobsByStatus(StatusRecording)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Infof("No jobs with status '%s' found", StatusRecording)
		return
	}
	// Other errors
	if err != nil {
		log.Errorf("Error getting jobs: %s", err)
		return
	}

	// Check for orphaned videos
	for _, job := range jobs {
		log.Infof("Handling Job #%d of '%s/%s'", job.JobId, job.Filepath, job.Filename)
		err := helpers.CheckVideo(job.Filepath)
		if err != nil {
			log.Errorf("The file '%s' is corrupted, deleting from disk and job queue: %s", job.Filename, err)
			job.Destroy()
			if err := os.Remove(job.Filepath); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Errorf(fmt.Sprintf("Error deleting recording: %s", err))
				continue
			}
			log.Errorf("Deleted file '%s'", job.Filename)
		} else {
			database.CreateRecording(job.ChannelId, job.Filename, "recording")
			job.Destroy()
			log.Infof("Added recording for '%s' and deleted orphaned recording job", job.Filename)
		}
	}
}

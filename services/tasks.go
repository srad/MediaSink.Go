package services

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/models"
	"gorm.io/gorm"
	"os"
	"time"
)

func StartUpJobs() error {
	log.Infoln("[StartUpJobs] Running startup job ...")

	deleteChannels()
	StartImport()
	go fixOrphanedRecordings()

	return nil
}

func deleteChannels() error {
	channels, err := models.ChannelList()
	if err != nil {
		log.Errorf("[DeleteChannels] ChannelList error: %s", err)
		return err
	}

	for _, channel := range channels {
		if channel.Deleted {
			log.Infof("[DeleteChannels] Deleting channel : %s", channel.ChannelName)
			channel.Destroy()
		}
	}

	return nil
}

// FixOrphanedRecordings Go through all open jobs with status "recording" and complete them.
func fixOrphanedRecordings() {
	log.Infoln("Fixing orphaned recordings ...")
	jobs, err := models.GetJobsByStatus(models.StatusRecording)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Infof("No jobs with status '%s' found", models.StatusRecording)
		return
	}
	// Other errors
	if err != nil {
		log.Errorf("Error getting jobs: %s", err)
		return
	}

	// Check for orphaned videos
	for _, job := range jobs {
		log.Errorf("Handling Job #%d of '%s/%s'", job.JobId, job.Filepath, job.Filename)
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
			rec := &models.Recording{
				ChannelName:  job.ChannelName,
				Duration:     0,
				Filename:     job.Filename,
				PathRelative: job.ChannelName.ChannelPath(job.Filename),
				Bookmark:     false,
				CreatedAt:    time.Now(),
			}
			rec.Create()
			job.Destroy()
			log.Infof("Added recording for '%s' and deleted orphaned recording job", job.Filename)
		}
	}
}

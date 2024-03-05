package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/srad/streamsink/workers"

	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/database"
	"gorm.io/gorm"
)

var (
	importing    = false
	cancelImport context.CancelFunc
)

// FixOrphanedRecordings Go through all open jobs with status "recording" and complete them.
func fixOrphanedRecordings() {
	log.Println("Fixing orphaned recordings ...")
	jobs, err := database.GetJobsByStatus(database.StatusRecording)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("No jobs with status '%s' found\n", database.StatusRecording)
		return
	}
	// Other errors
	if err != nil {
		log.Printf("Error getting jobs: %v\n", err)
		return
	}

	// Check for orphaned videos
	for _, job := range jobs {
		log.Printf("Handling Job #%d of '%s/%s'", job.JobId, job.Filepath, job.Filename)
		err := workers.CheckVideo(job.Filepath)
		if err != nil {
			log.Printf("The file '%s' is corrupted, deleting from disk and job queue: %v\n", job.Filename, err)
			job.Destroy()
			if err := os.Remove(job.Filepath); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Println(fmt.Sprintf("Error deleting recording: %v", err))
				continue
			}
			log.Printf("Deleted file '%s'", job.Filename)
		} else {
			rec := &database.Recording{
				ChannelName:  job.ChannelName,
				Duration:     0,
				Filename:     job.Filename,
				PathRelative: conf.GetRelativeRecordingsPath(job.ChannelName, job.Filename),
				Bookmark:     false,
				CreatedAt:    time.Now(),
			}
			rec.Save("recording")
			job.Destroy()
			log.Printf("Added recording for '%s' and deleted orphaned recording job\n", job.Filename)
		}
	}
}

func StartUpJobs() error {
	log.Println("[StartUpJobs] Running startup job")

	deleteChannels() // wait for this to complete
	StartImport()
	go fixOrphanedRecordings()

	return nil
}

func deleteChannels() error {
	channels, err := database.ChannelList()
	if err != nil {
		log.Printf("[StartUpJobs] ChannelList error: %s", err.Error())
		return err
	}

	for _, channel := range channels {
		if channel.Deleted {
			log.Printf("[StartUpJobs] Deleting channel : %s", channel.ChannelName)
			channel.Destroy()
		}
	}

	return nil
}

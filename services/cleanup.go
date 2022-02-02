package services

import (
	"errors"
	"fmt"
	"github.com/srad/streamsink/media"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/models"
	"gorm.io/gorm"
)

// FixOrphanedRecordings Go through all open jobs with status "recording" and complete them.
func FixOrphanedRecordings() {
	log.Println("Fixing orphaned recordings ...")
	jobs, err := models.GetJobsByStatus(models.StatusRecording)

	if err == gorm.ErrRecordNotFound {
		log.Printf("No jobs with status '%s' found\n", models.StatusRecording)
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
		err := media.CheckVideo(job.Filepath)
		if err != nil {
			log.Printf("The file '%s' is corrupted, deleting from disk and job queue: %v\n", job.Filename, err)
			job.Destroy()
			if err := os.Remove(job.Filepath); err != nil && err != os.ErrNotExist {
				log.Println(fmt.Sprintf("Error deleting recording: %v", err))
				continue
			}
			log.Printf("Deleted file '%s'", job.Filename)
		} else {
			rec := &models.Recording{
				ChannelName:  job.ChannelName,
				Duration:     0,
				Filename:     job.Filename,
				PathRelative: conf.GetRelativeRecordingsPath(job.ChannelName, job.Filename),
				Bookmark:     false,
				CreatedAt:    time.Now(),
			}
			rec.Save()
			job.Destroy()
			log.Printf("Added recording for '%s' and deleted orphaned recording job\n", job.Filename)
		}
	}
}

func ImportRecordings() error {
	log.Println("////////////////////////////////////// ImportRecordings //////////////////////////////////////")
	log.Printf("[Import] Importing files from file system: %s", conf.AppCfg.RecordingsAbsolutePath)

	file, err := os.Open(conf.AppCfg.RecordingsAbsolutePath)
	if err != nil {
		log.Printf(" [Import] Failed opening directory: %s\n", err.Error())
		return err
	}
	defer file.Close()

	channelFolders, _ := file.Readdirnames(0)
	for _, channelName := range channelFolders {
		log.Printf("[Import] Reading folder: %s\n", channelName)

		channel := &models.Channel{
			ChannelName: channelName,
			Url:         fmt.Sprintf(conf.AppCfg.Default.ImportUrl, channelName),
		}

		if err := channel.Save(); err != nil {
			log.Printf(" + Error adding channel channel '%s': %v", channelName, err)
		}

		files, err := os.ReadDir(conf.AbsoluteRecordingsPath(channelName))
		if err != nil {
			log.Printf("[Import] Error reading '%s': %v", channelName, err)
			continue
		}
		// Traverse all mp4 files and add to database if not existent
		for _, file := range files {
			if !file.IsDir() && filepath.Ext(file.Name()) == ".mp4" {
				log.Printf(" + [Import] Checking file: %s, %s", channelName, file.Name())

				if _, err := media.GetVideoInfo(conf.GetAbsoluteRecordingsPath(channelName, file.Name())); err != nil {
					log.Printf(" + [Import] File '%s' seems corrupted, deleting", file.Name())
					if err := channel.DeleteRecordingsFile(file.Name()); err != nil {
						log.Printf(" + [Import] Error deleting '%s'", file.Name())
					} else {
						models.DestroyPreviews(channelName, file.Name())
						log.Printf(" + [Import] Deleted file '%s'", file.Name())
					}
					continue
				}
				if _, err := models.AddIfNotExistsRecording(channelName, file.Name()); err != nil {
					log.Printf(" + [Import] Error: %s\n", err.Error())
					continue
				}

				// Not new record inserted and therefore not automatically new previews generated.
				// So check if the files exist and if not generate them.
				// Create preview if any not existent
				paths := conf.GetRecordingsPaths(channelName, file.Name())
				_, err1 := os.Stat(paths.AbsoluteVideosPath)
				_, err2 := os.Stat(paths.AbsoluteStripePath)

				if err1 == nil && err2 == nil {
					log.Println(" + [Import] Preview files exist")
					models.UpdatePreview(channelName, file.Name())
					continue
				} else if errors.Is(err1, os.ErrNotExist) || errors.Is(err2, os.ErrNotExist) {
					log.Printf(" + [Import] Adding job for %s\n", file.Name())
					models.EnqueuePreviewJob(channelName, file.Name())
				} else {
					// Schrodinger: file may or may not exist. See err for details.
					// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
					log.Printf(" + [Import] Error: %v, %v", err1, err2)
				}
			}
		}
	}
	log.Println("//////////////////////////////////////////////////////////////////////////////////////////////")

	return nil
}

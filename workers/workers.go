package workers

import (
	"fmt"
	"github.com/srad/streamsink/media"
	"log"
	"time"

	"github.com/srad/streamsink/conf"
	"gorm.io/gorm"

	"github.com/srad/streamsink/models"
	"github.com/srad/streamsink/utils"
)

var (
	ActiveJobId uint = 0
)

func JobWorker() {
	for {
		// These jobs do not run in parallel,
		// so they don't use up too much CPU
		previewJobs()
		cuttingJobs()
		time.Sleep(10 * time.Second)
	}
}

// Handles one single job.
func previewJobs() {
	job, err := models.GetNextJob(models.StatusPreview)
	if job == nil && err == nil {
		// log.Printf("No jobs found with status '%s'", models.StatusPreview)
		return
	}
	if err != nil {
		log.Printf("[Job] Error handlung job: %v", err)
		return
	}

	// Delete any old previews first
	errDelete := models.DeletePreview(job.ChannelName, job.Filename)
	if errDelete != nil && errDelete != gorm.ErrRecordNotFound {
		log.Printf("[Job] Error deleting existing previews: %v", errDelete)
	}

	log.Printf("[Job] Generating preview for '%s'", job.Filename)
	err = models.ActiveJob(job.JobId)
	if err != nil {
		log.Printf("[Job] Error activating job: %d", job.JobId)
		ActiveJobId = job.JobId
	}

	err = media.GeneratePreviews(job.ChannelName, job.Filename)
	if err != nil {
		// Delete the file if it is corrupted
		checkFileErr := media.CheckVideo(conf.GetRecordingsPaths(job.ChannelName, job.Filename).Filepath)
		if checkFileErr != nil {
			models.DeleteRecording(job.ChannelName, job.Filename)
			log.Printf("[Job] File corrupted, deleting '%s', %v\n", job.Filename, checkFileErr)
		}
		// Since the job failed for some reason, remove it
		models.DeleteJob(job.JobId)
		log.Printf("[Job] Error generating preview for '%s' : %v\n", job.Filename, err)
		return
	}

	_, err2 := models.AddPreview(job.ChannelName, job.Filename)
	if err2 != nil {
		log.Printf("[Job] Error adding previews: %v", err2)
		return
	}

	err3 := models.DeleteJob(job.JobId)
	if err3 != nil {
		log.Printf("[Job] Error deleteing job: %v", err3)
		return
	}

	ActiveJobId = 0
	log.Printf("[Job] Preview job complete for '%s'", job.Filepath)
}

func cuttingJobs() {
	job, err := models.GetNextJob(models.StatusCut)
	if job == nil && err == nil {
		// log.Printf("No jobs found with status '%s'", models.StatusPreview)
		return
	}
	if err != nil {
		log.Printf("[Job] Error handlung job: %v", err)
		return
	}

	log.Printf("Starting video cutting for '%s'", job.Filename)
	models.ActiveJob(job.JobId)

	filename := utils.FileNameWithoutExtension(job.Filename)

	// Count the number of copies to enumerate them
	var count int64
	models.Db.Model(&models.Recording{}).Where("filename LIKE ?", filename+"_%").Count(&count)

	outputFilename := fmt.Sprintf("%s_%04d.mp4", filename, count)
	absolutePath := conf.AbsoluteFilepath(job.ChannelName, outputFilename)

	err = media.CutVideo(job.Filepath, absolutePath, *job.Args)
	if err != nil {
		log.Printf("Error cutting video: '%s'", outputFilename)
		return
	}

	err2 := models.AddRecording(&models.Recording{
		ChannelName:  job.ChannelName,
		Bookmark:     false,
		CreatedAt:    time.Now(),
		PathRelative: conf.GetRelativeRecordingsPath(job.ChannelName, outputFilename),
		Filename:     outputFilename,
	})

	if err2 != nil {
		log.Printf("Error adding job for cut '%s': %v", job.Filename, err2)
		return
	}
	log.Printf("Completed cutting '%s'", absolutePath)

	err3 := models.DeleteJob(job.JobId)
	if err3 != nil {
		log.Printf("Error deleteing job: %v", err3)
	}

	_, err4 := models.EnqueuePreviewJob(job.ChannelName, outputFilename)
	if err4 != nil {
		log.Printf("Error adding preview job for '%s': %v", outputFilename, err4)
	}
}

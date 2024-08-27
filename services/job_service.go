package services

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/network"
	"os"
	"strings"
	"time"
)

const (
	// StatusRecording Recording in progress
	StatusRecording = "recording"
	StatusConvert   = "convert"
	// StatusPreview Generating preview currently
	StatusPreview = "preview"
	StatusCut     = "cut"
)

var (
	JobInfoChannel      = make(chan network.EventMessage, 1000)
	sleepBetweenRounds  = 1 * time.Second
	ctxJobs, cancelJobs = context.WithCancel(context.Background())
)

type JobMessage struct {
	JobId       uint                       `json:"jobId,omitempty" extensions:"!x-nullable"`
	RecordingId database.RecordingId       `json:"recordingId,omitempty" extensions:"!x-nullable"`
	ChannelId   database.ChannelId         `json:"channelId,omitempty" extensions:"!x-nullable"`
	ChannelName string                     `json:"channelName,omitempty" extensions:"!x-nullable"`
	Filename    database.RecordingFileName `json:"filename,omitempty"`
	Type        string                     `json:"type,omitempty"`
	Data        interface{}                `json:"data,omitempty"`
}

func DestroyJog(id uint) error {
	if job, err := database.FindJobById(id); err != nil {
		return err
	} else {
		job.Destroy()
		log.Infof("[Job] Job id delete %d", job.JobId)
		network.BroadCastClients("job:destroy", JobMessage{JobId: job.JobId, ChannelId: job.ChannelId, ChannelName: job.ChannelName.String(), Filename: job.Filename})
	}

	return nil
}

func ActivateJob(id uint) error {
	if job, err := database.FindJobById(id); err != nil {
		return err
	} else {
		job.Activate()
		log.Infof("[Job] Job id activate %d", job.JobId)
	}

	return nil
}

func CreateJob(recording *database.Recording, status string, args *string) (*database.Job, error) {
	job := &database.Job{
		ChannelId:   recording.ChannelId,
		ChannelName: recording.ChannelName,
		RecordingId: recording.RecordingId,
		Filename:    recording.Filename,
		Filepath:    recording.ChannelName.AbsoluteChannelFilePath(recording.Filename),
		Status:      status,
		Args:        args,
		Active:      false,
		CreatedAt:   time.Now(),
	}

	if err := job.CreateJob(); err != nil {
		log.Errorf("[Job] Error enqueing job: '%s/%s' -> %s: %s", recording.ChannelName, recording.Filename, status, err)
	} else {
		log.Infof("[Job] Enqueued job: '%s/%s' -> %s", recording.ChannelName, recording.Filename, status)
		network.BroadCastClients("job:create", JobMessage{JobId: job.JobId, Type: status, ChannelId: job.ChannelId, ChannelName: job.ChannelName.String(), Filename: job.Filename})
	}

	return job, nil
}

// PreviewJobs Handles one single job.
func previewJobs() error {
	job, errNextJob := database.GetNextJob(StatusPreview)
	if job == nil {
		return errNextJob
	}

	recording, err := job.RecordingId.FindRecordingById()
	if err != nil {
		return err
	}
	log.Infof("[previewJobs] Recording: %v", recording)

	// Delete any old previews first
	if err := recording.RecordingId.DestroyPreviews(); err != nil {
		log.Errorf("[Job] Error deleting existing previews: %s", err)
	}

	log.Infof("[Job] Start generating preview for '%s'", job.Filename)

	if err := job.Activate(); err != nil {
		log.Infof("[Job] Error activating job: %d", job.JobId)
	}

	conversion := &helpers.VideoConversionArgs{
		OnStart: func(info *helpers.CommandInfo) {
			_ = job.UpdateInfo(info.Pid, info.Command)
			log.Infof("Updating job %d pid=%d, command=%s", job.JobId, job.Pid, info.Command)

			network.BroadCastClients("job:start", JobMessage{
				JobId:       job.JobId,
				RecordingId: job.RecordingId,
				ChannelId:   job.ChannelId,
				ChannelName: job.ChannelName.String(),
				Filename:    job.Filename,
				Type:        job.Status,
			})
		},
		OnProgress: func(info *helpers.ProcessInfo) {
			network.BroadCastClients("job:progress", JobMessage{
				JobId:       job.JobId,
				ChannelId:   job.ChannelId,
				ChannelName: job.ChannelName.String(),
				Filename:    job.Filename,
				Data:        database.JobVideoInfo{Frame: info.Frame, Packets: uint64(info.Total)}})
		},
		InputPath:  job.ChannelName.AbsoluteChannelPath(),
		OutputPath: job.ChannelName.AbsoluteChannelDataPath(),
		Filename:   job.Filename.String(),
	}

	// 1. First check if input file is corrupt.
	if _, err := helpers.GeneratePreviews(conversion); err != nil {
		// 2. Delete the file if it is corrupted
		checkFileErr := helpers.CheckVideo(job.ChannelName.GetRecordingsPaths(job.Filename).Filepath)
		if checkFileErr != nil {
			if rec, errJob := job.RecordingId.FindRecordingById(); errJob != nil && rec != nil {
				_ = rec.Destroy()
			}
			log.Errorf("[Job] File corrupted, deleting '%s', %v", job.Filename, checkFileErr)
		}
		// 3. Since the job failed for some reason, remove it
		_ = job.Destroy()

		return fmt.Errorf("error generating preview for '%s' : %s", job.Filename, err)
	}

	// 4. Went ok.
	if err := recording.RecordingId.AddPreviews(); err != nil {
		return fmt.Errorf("error adding previews: %v", err)
	}

	network.BroadCastClients("job:preview:done", JobMessage{JobId: job.JobId, RecordingId: job.RecordingId, ChannelId: job.ChannelId, ChannelName: job.ChannelName.String(), Filename: job.Filename})

	if errDestroy := job.Destroy(); errDestroy != nil {
		return fmt.Errorf("error deleting job: %v", errDestroy)
	}

	return nil
}

func conversionJobs() error {
	job, err := database.GetNextJob(StatusConvert)
	if job == nil {
		return err
	}

	if err := job.Activate(); err != nil {
		log.Errorf("Error activating job: %s", err)
	}

	log.Infof("Job info: %v", job)

	result, err := helpers.ConvertVideo(&helpers.VideoConversionArgs{
		OnStart: func(info *helpers.CommandInfo) {
			_ = job.UpdateInfo(info.Pid, info.Command)
		},
		OnProgress: func(info *helpers.ProcessInfo) {
			if err := job.UpdateProgress(fmt.Sprintf("%f", float32(info.Frame)/float32(info.Total)*100)); err != nil {
				log.Errorf("Error updating job progress: %s", err)
			}

			network.BroadCastClients("job:progress", JobMessage{JobId: job.JobId, ChannelId: job.ChannelId, Data: database.JobVideoInfo{Packets: uint64(info.Total), Frame: info.Frame}, Type: job.Status, ChannelName: job.ChannelName.String(), Filename: job.Filename})
		},
		InputPath:  job.ChannelName.AbsoluteChannelPath(),
		Filename:   job.Filename.String(),
		OutputPath: job.ChannelName.AbsoluteChannelPath(),
	}, *job.Args)

	if err != nil {
		log.Errorf("[conversionJobs] Error converting '%s' to '%s': %s", job.Filename, *job.Args, err)
		if err := os.Remove(result.Filepath); err != nil {
			log.Errorf("Error deleting file '%s': %s", result.Filepath, err)
		}
		if err := job.Destroy(); err != nil {
			log.Errorf("Error destroying job: %s", err)
		}
		return err
	} else {
		log.Infof("[conversionJobs] Completed conversion of '%s' with args '%s'", job.Filename, *job.Args)
	}

	if err := job.Destroy(); err != nil {
		log.Errorf("[conversionJobs] Error deleting job: %s", err)
	}

	// Also, when fails, destroy it, some reason it is foul.
	if _, err := database.CreateRecording(job.ChannelId, database.RecordingFileName(result.Filename), "recording"); err != nil {
		if errRemove := os.Remove(result.Filepath); errRemove != nil {
			log.Errorf("Error deleting file '%s': %s", result.Filepath, errRemove)
			return errRemove
		}
	} else {
		if _, errJob := EnqueuePreviewJob(job.RecordingId); errJob != nil {
			log.Errorf("Error enqueing preview job: %s", errJob)
			return errJob
		}
	}

	log.Infof("[processJobs] Conversion completed for '%s'", job.Filepath)

	return nil
}

func processJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Infoln("[processJobs] Worker stopped")
			return
		case <-time.After(sleepBetweenRounds):
			// Intentionally blocking call, one after another.

			// Conversion
			if err := conversionJobs(); err != nil {
				log.Errorf("[processJobs] Error current job: %s", err)
			}

			// Cutting
			if err := cuttingJobs(); err != nil {
				log.Errorf("[processJobs] Error current job: %s", err)
			}

			// Previews
			if err := previewJobs(); err != nil {
				log.Errorf("[processJobs] Error current job: %s", err)
			}

		}
	}
}

// Cut video, add preview job, destroy job.
// This action is intrinsically procedural, keep it together locally.
func cuttingJobs() error {
	job, err := database.GetNextJob(StatusCut)
	if job == nil {
		return err
	}

	log.Errorf("[Job] Generating preview for '%s'", job.Filename)
	err = job.Activate()
	if err != nil {
		log.Errorf("[Job] Error activating job: %d", job.JobId)
	}

	if job.Args == nil {
		log.Errorf("[Job] Error missing args for cutting job: %d", job.JobId)
		return err
	}

	// Parse arguments
	cutArgs := &helpers.CutArgs{}
	s := []byte(*job.Args)
	err = json.Unmarshal(s, &cutArgs)
	if err != nil {
		log.Errorf("[Job] Error parsing cutting job arguments: %s", err)
		_ = job.Destroy()
		return err
	}

	// Filenames
	now := time.Now()
	stamp := now.Format("2006_01_02_15_04_05")
	filename := database.RecordingFileName(fmt.Sprintf("%s_cut_%s.mp4", job.ChannelName, stamp))
	inputPath := job.ChannelName.AbsoluteChannelFilePath(job.Filename)
	outputFile := job.ChannelName.AbsoluteChannelFilePath(filename)
	segFiles := make([]string, len(cutArgs.Starts))
	mergeFileContent := make([]string, len(cutArgs.Starts))

	// Cut
	segmentFilename := fmt.Sprintf("%s_cut_%s", job.ChannelName, stamp)
	for i, start := range cutArgs.Starts {
		segFiles[i] = job.ChannelName.AbsoluteChannelFilePath(database.RecordingFileName(fmt.Sprintf("%s_%04d.mp4", segmentFilename, i)))
		err = helpers.CutVideo(&helpers.CuttingJob{
			OnStart: func(info *helpers.CommandInfo) {
				_ = job.UpdateInfo(info.Pid, info.Command)
			},
			OnProgress: func(s string) {
				log.Infof("[CutVideo] %s", s)
			},
		}, inputPath, segFiles[i], start, cutArgs.Ends[i])
		// Failed, delete all segments
		if err != nil {
			log.Errorf("[Job] Error generating cut for file '%s': %s", inputPath, err)
			log.Infoln("[Job] Deleting orphaned segments")
			for _, file := range segFiles {
				if err := os.RemoveAll(file); err != nil {
					log.Errorf("[Job] Error deleting segment '%s': %s", file, err)
				}
			}
			_ = job.Destroy()
			return err
		}
	}
	// Merge file txt, enumerate
	for i, file := range segFiles {
		mergeFileContent[i] = fmt.Sprintf("file '%s'", file)
	}
	mergeTextFile := job.ChannelName.AbsoluteChannelFilePath(database.RecordingFileName(fmt.Sprintf("%s.txt", segmentFilename)))
	err = os.WriteFile(mergeTextFile, []byte(strings.Join(mergeFileContent, "\n")), 0644)
	if err != nil {
		log.Errorf("[Job] Error writing concat text file '%s': %s", mergeTextFile, err)
		for _, file := range segFiles {
			if err := os.RemoveAll(file); err != nil {
				log.Errorf("[Job] Error deleting %s: %s", file, err)
			}
		}
		_ = os.RemoveAll(mergeTextFile)
		_ = job.Destroy()
		return err
	}

	if err = helpers.MergeVideos(func(s string) { log.Errorf("[MergeVideos] %s\n", s) }, mergeTextFile, outputFile); err != nil {
		// Job failed, destroy all files.
		log.Errorf("[Job] Error merging file '%s': %s", mergeTextFile, err)
		for _, file := range segFiles {
			if err := os.RemoveAll(file); err != nil {
				log.Errorf("[Job] Error deleting %s: %s", file, err)
			}
		}
		_ = os.RemoveAll(mergeTextFile)
		_ = job.Destroy()
		return err
	}

	_ = os.RemoveAll(mergeTextFile)
	for _, file := range segFiles {
		log.Infof("[MergeJob] Deleting segment %s", file)
		if err := os.Remove(file); err != nil {
			log.Errorf("[Job] Error deleting segment '%s': %s", file, err)
		}
	}

	outputVideo := &helpers.Video{FilePath: outputFile}
	if _, err := outputVideo.GetVideoInfo(); err != nil {
		log.Errorf("[Job] Error reading video information for file '%s': %s", filename, err)
	}

	cutRecording, errCreate := database.CreateRecording(job.ChannelId, filename, "cut")
	if errCreate != nil {
		log.Errorf("[Job] Error creating: %s\n", errCreate)
		return errCreate
	}

	// Successfully added cut record, enqueue preview job
	if _, err = EnqueuePreviewJob(cutRecording.RecordingId); err != nil {
		log.Errorf("[Job] Error adding preview for cutting job %d: %s", job.JobId, err)
		return err
	}

	// Finished, destroy job
	if err = job.Destroy(); err != nil {
		log.Errorf("[Job] Error deleteing job: %s", err)
		return err
	}

	log.Infof("[Job] Cutting job complete for '%s'", job.Filepath)
	return nil
}

func EnqueueRecordingJob(id database.RecordingId, outputPath string) (*database.Job, error) {
	recording, err := id.FindRecordingById()
	if err != nil {
		return nil, err
	}
	return CreateJob(recording, StatusRecording, &outputPath)
}

func EnqueueConversionJob(id database.RecordingId, mediaType string) (*database.Job, error) {
	recording, err := id.FindRecordingById()
	if err != nil {
		return nil, err
	}
	return CreateJob(recording, StatusConvert, &mediaType)
}

func EnqueuePreviewJob(id database.RecordingId) (*database.Job, error) {
	recording, err := id.FindRecordingById()
	if err != nil {
		return nil, err
	}
	return CreateJob(recording, StatusPreview, nil)
}

func EnqueueCuttingJob(id database.RecordingId, intervals string) (*database.Job, error) {
	recording, err := id.FindRecordingById()
	if err != nil {
		return nil, err
	}
	return CreateJob(recording, StatusCut, &intervals)
}

func StartJobProcessing() {
	go processJobs(ctxJobs)
}

func StopJobProcessing() {
	cancelJobs()
}

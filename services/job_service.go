package services

import (
	"context"
	"encoding/json"
	"errors"
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
	sleepBetweenRounds  = 1 * time.Second
	ctxJobs, cancelJobs = context.WithCancel(context.Background())
)

type JobMessage struct {
	Job  *database.Job `json:"job"`
	Data interface{}   `json:"data"`
}

func CreateJob[T any](recording *database.Recording, status string, args *T) (*database.Job, error) {
	data := ""
	if args != nil {
		bytes, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		data = string(bytes)
	}

	job := &database.Job{
		ChannelId:   recording.ChannelId,
		ChannelName: recording.ChannelName,
		RecordingId: recording.RecordingId,
		Filename:    recording.Filename,
		Filepath:    recording.ChannelName.AbsoluteChannelFilePath(recording.Filename),
		Status:      status,
		Args:        &data,
		Active:      false,
		CreatedAt:   time.Now(),
	}

	if err := job.CreateJob(); err != nil {
		log.Errorf("[Job] Error enqueing job: '%s/%s' -> %s: %s", recording.ChannelName, recording.Filename, status, err)
	} else {
		log.Infof("[Job] Enqueued job: '%s/%s' -> %s", recording.ChannelName, recording.Filename, status)
		network.BroadCastClients("job:create", JobMessage{Job: job})
	}

	return job, nil
}

// PreviewJobs Handles one single job.
func previewJobs() error {
	job, _, errNextJob := database.GetNextJob[*any](StatusPreview)
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
		OnStart: func(info helpers.TaskInfo) {
			if err := job.UpdateInfo(info.Pid, info.Command); err != nil {
				log.Errorf("[Job] Error updating job info: %s", err)
			}

			network.BroadCastClients("job:start", JobMessage{
				Job:  job,
				Data: info,
			})
		},
		OnProgress: func(info helpers.TaskProgress) {
			network.BroadCastClients("job:progress", JobMessage{
				Job:  job,
				Data: info})
		},
		OnEnd: func(info helpers.TaskComplete) {
			network.BroadCastClients("job:done", JobMessage{
				Data: info,
				Job:  job,
			})
		},
		OnError: func(err error) {
			network.BroadCastClients("job:error", JobMessage{
				Data: err.Error(),
				Job:  job,
			})
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
			log.Errorf("[Job] File corrupted, deleting '%s', %s", job.Filename, checkFileErr)
		}
		// 3. Since the job failed for some reason, remove it
		_ = job.Destroy()

		return fmt.Errorf("error generating preview for '%s' : %s", job.Filename, err)
	}

	// 4. Went ok.
	if err := recording.RecordingId.AddPreviews(); err != nil {
		return fmt.Errorf("error adding previews: %s", err)
	}

	network.BroadCastClients("job:preview:done", JobMessage{Job: job})

	if errDestroy := job.Destroy(); errDestroy != nil {
		return fmt.Errorf("error deleting job: %s", errDestroy)
	}

	return nil
}

func conversionJobs() error {
	job, mediaType, err := database.GetNextJob[string](StatusConvert)
	if job == nil {
		return err
	}

	if mediaType == nil {
		return errors.New("media type is nil")
	}

	if err := job.Activate(); err != nil {
		log.Errorf("Error activating job: %s", err)
	}

	result, err := helpers.ConvertVideo(&helpers.VideoConversionArgs{
		OnStart: func(info helpers.TaskInfo) {
			if err := job.UpdateInfo(info.Pid, info.Command); err != nil {
				log.Errorf("Error updating job info: %s", err)
			}
		},
		OnProgress: func(info helpers.TaskProgress) {
			if err := job.UpdateProgress(fmt.Sprintf("%f", float32(info.Current)/float32(info.Total)*100)); err != nil {
				log.Errorf("Error updating job progress: %s", err)
			}

			network.BroadCastClients("job:progress", JobMessage{Job: job, Data: database.JobVideoInfo{Total: info.Total, Current: info.Current}})
		},
		OnError: func(err error) {
			network.BroadCastClients("job:error", JobMessage{Job: job, Data: err.Error()})
		},
		InputPath:  job.ChannelName.AbsoluteChannelPath(),
		Filename:   job.Filename.String(),
		OutputPath: job.ChannelName.AbsoluteChannelPath(),
	}, *mediaType)

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
			// Intentionally blocking call, one after another. Tasks use all cores.

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

// Three-phase cutting job:
// 1. Cut video at the given time intervals
// 2. Merge the cuts
// 3. Enqueue preview job for new cut
// This action is intrinsically procedural, keep it together locally.
func cuttingJobs() error {
	job, cutArgs, err := database.GetNextJob[helpers.CutArgs](StatusCut)
	if job == nil {
		return err
	}

	if job.Args == nil {
		log.Errorf("[Job] Error missing args for cutting job: %d", job.JobId)
		return err
	}

	log.Infof("[Job] Generating video cut for '%s'", job.Filename)

	err = job.Activate()
	if err != nil {
		log.Errorf("[Job] Error activating job: %d", job.JobId)
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
				network.BroadCastClients("job:progress", JobMessage{Job: job, Data: s})
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

	if err = helpers.MergeVideos(func(s string) {
		network.BroadCastClients("job:progress", JobMessage{Job: job, Data: s})
	}, mergeTextFile, outputFile); err != nil {
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

	// The original file shall be deleted after the process if successful.
	if cutArgs.DeleteAfterCompletion {
		if err := job.Recording.Destroy(); err != nil {
			return err
		}
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
	return CreateJob[string](recording, StatusConvert, &mediaType)
}

func EnqueuePreviewJob(id database.RecordingId) (*database.Job, error) {
	recording, err := id.FindRecordingById()
	if err != nil {
		return nil, err
	}
	return CreateJob[any](recording, StatusPreview, nil)
}

func EnqueueCuttingJob(id database.RecordingId, intervals *helpers.CutArgs) (*database.Job, error) {
	recording, err := id.FindRecordingById()
	if err != nil {
		return nil, err
	}
	return CreateJob(recording, StatusCut, intervals)
}

func StartJobProcessing() {
	go processJobs(ctxJobs)
}

func StopJobProcessing() {
	cancelJobs()
}

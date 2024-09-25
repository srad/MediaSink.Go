package services

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/network"
	"os"
	"strings"
	"time"
)

var (
	sleepBetweenRounds  = 1 * time.Second
	ctxJobs, cancelJobs = context.WithCancel(context.Background())
	processing          = false
)

type JobMessage struct {
	Job  *database.Job `json:"job"`
	Data interface{}   `json:"data"`
}

func processJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Infoln("[processJobs] Worker stopped")
			processing = false
			return
		case <-time.After(sleepBetweenRounds):
			processing = true
			job, err := database.GetNextJob()
			if err != nil {
				_ = job.Error(err)
				continue
			}
			if job == nil {
				continue
			}

			if err := job.Activate(); err != nil {
				log.Errorf("Error activating job: %s", err)
				return
			}

			var jobErr error = nil

			switch job.Task {
			case database.TaskPreview:
				jobErr = processPreview(job)
			case database.TaskCut:
				jobErr = processCutting(job)
			case database.TaskConvert:
				jobErr = processConversion(job)
			}

			if jobErr != nil {
				_ = job.Error(jobErr)
				network.BroadCastClients(network.JobErrorEvent, JobMessage{
					Data: jobErr,
					Job:  job,
				})
			} else {
				_ = job.Completed()
			}
		}
	}
}

// PreviewJobs Handles one single job.
// Intentionally procedural, all steps can be read from top to bottom.
func processPreview(job *database.Job) error {
	recording, err := job.RecordingId.FindRecordingById()
	if err != nil {
		return err
	}
	log.Infof("[previewJobs] Recording: %v", recording)

	// Delete any old previews first
	if err := database.DestroyPreviews(recording.RecordingId); err != nil {
		log.Errorf("[Job] Error deleting existing previews: %s", err)
	}

	log.Infof("[Job] Start generating preview for '%s'", job.Filename)

	previewArgs := &helpers.VideoConversionArgs{
		OnStart: func(info helpers.TaskInfo) {
			if err := job.UpdateInfo(info.Pid, info.Command); err != nil {
				log.Errorf("[Job] Error updating job info: %s", err)
			}

			network.BroadCastClients(network.JobStartEvent, JobMessage{
				Job:  job,
				Data: info,
			})
		},
		OnProgress: func(info helpers.TaskProgress) {
			network.BroadCastClients(network.JobProgressEvent, JobMessage{
				Job:  job,
				Data: info})
		},
		OnEnd: func(info helpers.TaskComplete) {
			network.BroadCastClients(network.JobDoneEvent, JobMessage{
				Data: info,
				Job:  job,
			})
		},
		OnError: func(err error) {
			network.BroadCastClients(network.JobErrorEvent, JobMessage{
				Data: err.Error(),
				Job:  job,
			})
		},
		InputPath:  job.ChannelName.AbsoluteChannelPath(),
		OutputPath: job.ChannelName.AbsoluteChannelDataPath(),
		Filename:   job.Filename.String(),
	}

	// 1. First check if input file is corrupt.
	if _, err := helpers.GeneratePreviews(previewArgs); err != nil {
		// 2. Delete the file if it is corrupted
		if checkFileErr := helpers.CheckVideo(job.ChannelName.GetRecordingsPaths(job.Filename).Filepath); checkFileErr != nil {
			if rec, errJob := job.RecordingId.FindRecordingById(); errJob != nil && rec != nil {
				_ = database.DestroyRecording(rec.RecordingId)
			}

			return fmt.Errorf("error generating previews for file %s is corrupt: %s", job.Filename, checkFileErr)
		}

		// 3. File not corrupt, only error generating the preview
		return fmt.Errorf("error generating previews for file %s: %s", job.Filename, err)
	}

	// 4. Went ok. Now generate previews.
	if err := database.AddPreviewPaths(recording.RecordingId); err != nil {
		return fmt.Errorf("error adding previews: %s", err)
	}

	network.BroadCastClients(network.JobPreviewDoneEvent, JobMessage{Job: job})

	return nil
}

func processConversion(job *database.Job) error {
	mediaType, err := database.UnmarshalJobArg[string](job)
	if err != nil {
		return err
	}

	result, errConvert := helpers.ConvertVideo(&helpers.VideoConversionArgs{
		OnStart: func(info helpers.TaskInfo) {
			if err := job.UpdateInfo(info.Pid, info.Command); err != nil {
				log.Errorf("Error updating job info: %s", err)
			}
		},
		OnProgress: func(info helpers.TaskProgress) {
			if err := job.UpdateProgress(fmt.Sprintf("%f", float32(info.Current)/float32(info.Total)*100)); err != nil {
				log.Errorf("Error updating job progress: %s", err)
			}

			network.BroadCastClients(network.JobProgressEvent, JobMessage{Job: job, Data: info})
		},
		OnError: func(err error) {
			network.BroadCastClients(network.JobErrorEvent, JobMessage{Job: job, Data: err.Error()})
		},
		InputPath:  job.ChannelName.AbsoluteChannelPath(),
		Filename:   job.Filename.String(),
		OutputPath: job.ChannelName.AbsoluteChannelPath(),
	}, *mediaType)

	if errConvert != nil {
		message := fmt.Errorf("error converting %s to %s: %s", job.Filename, *mediaType, errConvert)

		log.Errorln(message)
		if errDelete := os.Remove(result.Filepath); errDelete != nil {
			log.Errorf("error deleting file %s: %s", result.Filepath, errDelete)
		}
		return message
	} else {
		log.Infof("[conversionJobs] Completed conversion of '%s' with args '%s'", job.Filename, *job.Args)
	}

	// Also, when fails, destroy it, some reason it is foul.
	if _, err := database.CreateRecording(job.ChannelId, database.RecordingFileName(result.Filename), "recording"); err != nil {
		if errRemove := os.Remove(result.Filepath); errRemove != nil {
			return fmt.Errorf("error deleting file %s: %s", result.Filepath, errRemove)
		}
	} else {
		if _, errJob := EnqueuePreviewJob(job.RecordingId); errJob != nil {
			return fmt.Errorf("error enqueuing preview job for %s: %s", result.Filename, errJob)
		}
	}

	log.Infof("Conversion completed for %s", job.Filepath)

	return nil
}

// Three-phase cutting job:
// 1. Cut video at the given time intervals
// 2. Merge the cuts
// 3. Enqueue preview job for new cut
// This action is intrinsically procedural, keep it together locally.
func processCutting(job *database.Job) error {
	cutArgs, err := database.UnmarshalJobArg[helpers.CutArgs](job)
	if err != nil {
		return err
	}

	log.Infof("[Job] Generating video cut for '%s'", job.Filename)

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

				network.BroadCastClients(network.JobStartEvent, JobMessage{
					Job: job,
					Data: helpers.TaskInfo{
						Steps:   2,
						Step:    1,
						Pid:     info.Pid,
						Command: info.Command,
						Message: "Starting cutting phase",
					},
				})
			},
			OnProgress: func(s string) {
				network.BroadCastClients(network.JobProgressEvent, JobMessage{Job: job, Data: s})
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
			return err
		}
	}
	// Merge file txt, enumerate
	for i, file := range segFiles {
		mergeFileContent[i] = fmt.Sprintf("file '%s'", file)
	}
	mergeFileAbsolutePath := job.ChannelName.AbsoluteChannelFilePath(database.RecordingFileName(fmt.Sprintf("%s.txt", segmentFilename)))
	errWriteMergeFile := os.WriteFile(mergeFileAbsolutePath, []byte(strings.Join(mergeFileContent, "\n")), 0644)
	if errWriteMergeFile != nil {
		log.Errorf("[Job] Error writing concat text file %s: %s", mergeFileAbsolutePath, errWriteMergeFile)
		for _, file := range segFiles {
			if err := os.RemoveAll(file); err != nil {
				log.Errorf("[Job] Error deleting %s: %s", file, err)
			}
		}
		if err := os.RemoveAll(mergeFileAbsolutePath); err != nil {
			log.Errorf("[Job] Error deleting merge file %s: %s", mergeFileAbsolutePath, err)
		}
		return errWriteMergeFile
	}

	errMerge := helpers.MergeVideos(&helpers.MergeArgs{
		OnStart: func(info helpers.CommandInfo) {
			network.BroadCastClients(network.JobStartEvent, JobMessage{
				Job: job,
				Data: helpers.TaskInfo{
					Steps:   2,
					Step:    2,
					Pid:     info.Pid,
					Command: info.Command,
					Message: "Starting merge phase",
				},
			})
		},
		OnProgress: func(info helpers.PipeMessage) {
			// TODO: For cutting and merging ffmpeg doesnt seem to provide obvious progress information, check again.
			//network.BroadCastClients("job:progress", JobMessage{Job: job, Data: info})
		},
		OnErr: func(err error) {
			network.BroadCastClients(network.JobErrorEvent, JobMessage{Job: job, Data: err.Error()})
		},
		MergeFileAbsolutePath:  mergeFileAbsolutePath,
		AbsoluteOutputFilepath: outputFile,
	})

	if errMerge != nil {
		// Job failed, destroy all files.
		log.Errorf("Error merging file '%s': %s", mergeFileAbsolutePath, err)
		for _, file := range segFiles {
			if err := os.RemoveAll(file); err != nil {
				log.Errorf("Error deleting %s: %s", file, err)
			}
		}
		if err := os.RemoveAll(mergeFileAbsolutePath); err != nil {
			log.Errorf("Error deleting merge file %s: %s", mergeFileAbsolutePath, err)
		}
		return err
	}

	_ = os.RemoveAll(mergeFileAbsolutePath)
	for _, file := range segFiles {
		log.Infof("[MergeJob] Deleting segment %s", file)
		if err := os.Remove(file); err != nil {
			log.Errorf("Error deleting segment '%s': %s", file, err)
		}
	}

	outputVideo := &helpers.Video{FilePath: outputFile}
	if _, err := outputVideo.GetVideoInfo(); err != nil {
		log.Errorf("Error reading video information for file '%s': %s", filename, err)
	}

	cutRecording, errCreate := database.CreateRecording(job.ChannelId, filename, "cut")
	if errCreate != nil {
		return errCreate
	}

	// Successfully added cut record, enqueue preview job
	if _, err = EnqueuePreviewJob(cutRecording.RecordingId); err != nil {
		log.Errorf("Error adding preview for cutting job %d: %s", job.JobId, err)
		return err
	}

	// The original file shall be deleted after the process if successful.
	if cutArgs.DeleteAfterCompletion {
		if err := database.DestroyRecording(job.RecordingId); err != nil {
			log.Errorf("Eror deleting recording after cutting job for %s: %s", outputFile, err)
		}
	}

	return nil
}

func EnqueueConversionJob(id database.RecordingId, mediaType string) (*database.Job, error) {
	return enqueueJob[string](id, database.TaskConvert, &mediaType)
}

func EnqueuePreviewJob(id database.RecordingId) (*database.Job, error) {
	return enqueueJob[*any](id, database.TaskPreview, nil)
}

func EnqueueCuttingJob(id database.RecordingId, args *helpers.CutArgs) (*database.Job, error) {
	return enqueueJob(id, database.TaskCut, args)
}

func enqueueJob[T any](id database.RecordingId, task database.JobTask, args *T) (*database.Job, error) {
	if recording, err := id.FindRecordingById(); err != nil {
		return nil, err
	} else {
		if job, err2 := database.CreateJob(recording, task, args); err2 != nil {
			return nil, err2
		} else {
			network.BroadCastClients(network.JobCreateEvent, job)
			return job, nil
		}
	}
}

func AddPreviews(id database.RecordingId, task database.JobTask) error {
	if jobExists, err := database.JobExists(id, task); err != nil {
		return err
	} else {
		if jobExists {
			return fmt.Errorf("job for task %s already exists for recording id %d", task, id)
		}
		return database.AddPreviewPaths(id)
	}
}

func DeleteJob(id uint) error {
	if err := database.DeleteJob(id); err != nil {
		return err
	}
	network.BroadCastClients(network.JobDeleteEvent, id)
	return nil
}

func StartJobProcessing() {
	ctxJobs, cancelJobs = context.WithCancel(context.Background())
	go processJobs(ctxJobs)
}

func StopJobProcessing() {
	cancelJobs()
}

func IsJobProcessing() bool {
	return processing
}

package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/network"
	"gorm.io/gorm"
)

var (
	sleepBetweenRounds = 1 * time.Second
	JobInfoChannel     = make(chan network.EventMessage, 1000)
	ctx, cancel        = context.WithCancel(context.Background())
)

type JobVideoInfo struct {
	Packets uint64 `json:"packets"`
	Frame   uint64 `json:"frame"`
}

func StartWorker() {
	go processJobs(ctx)
}

func StopWorker() {
	cancel()
}

func processJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[processJobs] Worker stopped")
			return
		case <-time.After(sleepBetweenRounds):
			// Intentionally blocking call, one after another.

			// Conversion
			if job, err := conversionJobs(); err != nil {
				log.Printf("[processJobs] Error current job: %s", err.Error())
			} else if job != nil {
				log.Printf("[processJobs] Conversion completed for '%s'", job.Filepath)
			}

			// Cutting
			if job, err := cuttingJobs(); err != nil {
				log.Printf("[processJobs] Error current job: %s", err.Error())
			} else if job != nil {
				log.Printf("[processJobs] Cutting completed for: %s", job.Filepath)
			}

			// Previews
			if job, err := previewJobs(); err != nil {
				log.Printf("[processJobs] Error current job: %s", err.Error())
			} else if job != nil {
				log.Printf("[processJobs] Preview completed for '%s'", job.Filepath)
			}
		}
	}
}

func conversionJobs() (*database.Job, error) {
	job, err := database.GetNextJob(database.StatusConvert)
	if job == nil || err != nil {
		return job, err
	}

	if err := job.Activate(); err != nil {
		log.Printf("Error activating job: %s", err.Error())
	}

	result, err := helpers.ConvertVideo(&helpers.VideoConversionArgs{
		OnStart: func(info *helpers.CommandInfo) {
			_ = job.UpdateInfo(info.Pid, info.Command)
		},
		OnProgress: func(info *helpers.ProcessInfo) {
			if err := job.UpdateProgress(fmt.Sprintf("%f", float32(info.Frame)/float32(job.Recording.Packets)*100)); err != nil {
				log.Printf("Error updating job progress: %s", err.Error())
			}

			JobInfoChannel <- network.EventMessage{Name: "job:progress", Message: database.JobMessage{JobId: job.JobId, Data: JobVideoInfo{Packets: job.Recording.Packets, Frame: info.Frame}, Type: job.Status, ChannelName: job.ChannelName, Filename: job.Filename}}
		},
		ChannelName: job.ChannelName,
		Filename:    job.Filename,
	}, *job.Args)

	if err != nil {
		log.Printf("[conversionJobs] Error converting '%s' to '%s': %s", job.Filename, *job.Args, err.Error())
		if err := os.Remove(result.Filepath); err != nil {
			log.Printf("Error deleting file '%s': %s", result.Filepath, err.Error())
		}
		if err := job.Destroy(); err != nil {
			log.Printf("Error destroying job: %s", err.Error())
		}
		return job, err
	} else {
		log.Printf("[conversionJobs] Completed conversion of '%s' with args '%s'", job.Filename, *job.Args)
	}

	if err := job.Destroy(); err != nil {
		log.Printf("[conversionJobs] Error deleting job: %s", err.Error())
	}

	// All good now, save the record.
	rec := &database.Recording{
		ChannelName:  job.ChannelName,
		Filename:     result.Filename,
		PathRelative: result.PathRelative,
		Bookmark:     false,
		CreatedAt:    result.CreatedAt,
	}
	// Also, when fails, destroy it, some reason it is foul.
	if err := rec.Save("recording"); err != nil {
		if err := os.Remove(result.Filepath); err != nil {
			log.Printf("Error deleting file '%s': %s", result.Filepath, err.Error())
			return job, err
		}
	} else {
		if _, err := database.EnqueuePreviewJob(result.ChannelName, result.Filename); err != nil {
			log.Printf("Error enqueing preview job: %s", err.Error())
			return job, err
		}
	}

	return job, nil
}

// Handles one single job.
func previewJobs() (*database.Job, error) {
	job, err := database.GetNextJob(database.StatusPreview)
	if job == nil || err != nil {
		return job, err
	}

	// Delete any old previews first
	if err := database.DestroyPreviews(job.ChannelName, job.Filename); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("[Job] Error deleting existing previews: %s", err.Error())
	}

	log.Printf("[Job] Generating preview for '%s'", job.Filename)
	err = job.Activate()
	if err != nil {
		log.Printf("[Job] Error activating job: %d", job.JobId)
	}

	conversion := &helpers.VideoConversionArgs{
		OnStart: func(info *helpers.CommandInfo) {
			_ = job.UpdateInfo(info.Pid, info.Command)
			JobInfoChannel <- network.EventMessage{Name: "job:start", Message: database.JobMessage{
				JobId:       job.JobId,
				ChannelName: job.ChannelName,
				Filename:    job.Filename,
				Type:        job.Status,
			}}
		},
		OnProgress: func(info *helpers.ProcessInfo) {
			JobInfoChannel <- network.EventMessage{Name: "job:progress", Message: database.JobMessage{
				JobId:       job.JobId,
				ChannelName: job.ChannelName,
				Filename:    job.Filename,
				Data:        JobVideoInfo{Frame: info.Frame, Packets: job.Recording.Packets},
			}}
		},
		ChannelName: job.ChannelName,
		Filename:    job.Filename,
	}
	if result, err := GeneratePreviews(conversion); err != nil {
		// Delete the file if it is corrupted
		checkFileErr := helpers.CheckVideo(conf.GetRecordingsPaths(job.ChannelName, job.Filename).Filepath)
		if checkFileErr != nil {
			if rec, err := job.FindRecording(); err != nil {
				_ = rec.Destroy()
			}
			log.Printf("[Job] File corrupted, deleting '%s', %v\n", job.Filename, checkFileErr)
		}
		// Since the job failed for some reason, remove it
		_ = job.Destroy()

		return job, fmt.Errorf("[Job] Error generating preview for '%s' : %v\n", job.Filename, err)
	} else if _, err := database.UpdatePreview(result.ChannelName, result.Filename); err != nil {
		return job, fmt.Errorf("[Job] Error adding previews: %v", err)
	}

	if _, err := job.FindRecording(); err != nil {
		// TODO
		// services.notify("job:preview:done", database.JobMessage{JobId: job.JobId, ChannelName: job.ChannelName, Filename: job.Filename, Data: rec})
	}

	if err := job.Destroy(); err != nil {
		return job, fmt.Errorf("[Job] Error deleteing job: %v", err)
	}

	return job, nil
}

// Cut video, add preview job, destroy job.
// This action is intrinsically procedural, keep it together locally.
func cuttingJobs() (*database.Job, error) {
	job, err := database.GetNextJob(database.StatusCut)
	if job == nil || err != nil {
		return job, err
	}

	log.Printf("[Job] Generating preview for '%s'", job.Filename)
	err = job.Activate()
	if err != nil {
		log.Printf("[Job] Error activating job: %d", job.JobId)
	}

	if job.Args == nil {
		log.Printf("[Job] Error missing args for cutting job: %d", job.JobId)
		return job, err
	}

	// Parse arguments
	cutArgs := &helpers.CutArgs{}
	s := []byte(*job.Args)
	err = json.Unmarshal(s, &cutArgs)
	if err != nil {
		log.Printf("[Job] Error parsing cutting job arguments: %v", err)
		_ = job.Destroy()
		return job, err
	}

	// Filenames
	now := time.Now()
	stamp := now.Format("2006_01_02_15_04_05")
	filename := fmt.Sprintf("%s_cut_%s.mp4", job.ChannelName, stamp)
	inputPath := conf.AbsoluteChannelFilePath(job.ChannelName, job.Filename)
	outputFile := conf.AbsoluteChannelFilePath(job.ChannelName, filename)
	segFiles := make([]string, len(cutArgs.Starts))
	mergeFileContent := make([]string, len(cutArgs.Starts))

	// Cut
	segmentFilename := fmt.Sprintf("%s_cut_%s", job.ChannelName, stamp)
	for i, start := range cutArgs.Starts {
		segFiles[i] = conf.AbsoluteChannelFilePath(job.ChannelName, fmt.Sprintf("%s_%04d.mp4", segmentFilename, i))
		err = helpers.CutVideo(&helpers.CuttingJob{
			OnStart: func(info *helpers.CommandInfo) {
				_ = job.UpdateInfo(info.Pid, info.Command)
			},
			OnProgress: func(s string) {
				log.Printf("[CutVideo] %s", s)
			},
		}, inputPath, segFiles[i], start, cutArgs.Ends[i])
		// Failed, delete all segments
		if err != nil {
			log.Printf("[Job] Error generating cut for file '%s': %v", inputPath, err)
			log.Println("[Job] Deleting orphaned segments")
			for _, file := range segFiles {
				if err := os.RemoveAll(file); err != nil {
					log.Printf("[Job] Error deleting segment '%s': %v", file, err)
				}
			}
			_ = job.Destroy()
			return job, err
		}
	}
	// Merge file txt, enumerate
	for i, file := range segFiles {
		mergeFileContent[i] = fmt.Sprintf("file '%s'", file)
	}
	mergeTextFile := conf.AbsoluteChannelFilePath(job.ChannelName, fmt.Sprintf("%s.txt", segmentFilename))
	err = os.WriteFile(mergeTextFile, []byte(strings.Join(mergeFileContent, "\n")), 0644)
	if err != nil {
		log.Printf("[Job] Error writing concat text file '%s': %v", mergeTextFile, err)
		for _, file := range segFiles {
			if err := os.RemoveAll(file); err != nil {
				log.Printf("[Job] Error deleting %s: %v", file, err)
			}
		}
		_ = os.RemoveAll(mergeTextFile)
		_ = job.Destroy()
		return job, err
	}

	err = helpers.MergeVideos(func(s string) { log.Printf("[MergeVideos] %s", s) }, mergeTextFile, outputFile)
	if err != nil {
		// Job failed, destroy all files.
		log.Printf("[Job] Error merging file '%s': %s", mergeTextFile, err.Error())
		for _, file := range segFiles {
			if err := os.RemoveAll(file); err != nil {
				log.Printf("[Job] Error deleting %s: %s", file, err.Error())
			}
		}
		_ = os.RemoveAll(mergeTextFile)
		_ = job.Destroy()
		return job, err
	}

	_ = os.RemoveAll(mergeTextFile)
	for _, file := range segFiles {
		log.Printf("[MergeJob] Deleting segment %s", file)
		if err := os.Remove(file); err != nil {
			log.Printf("[Job] Error deleting segment '%s': %s", file, err.Error())
		}
	}

	info, err := helpers.GetVideoInfo(outputFile)
	if err != nil {
		log.Printf("[Job] Error reading video information for file '%s': %v", filename, err)
	}

	// Cutting written to dist, add record to database
	newRec := database.Recording{
		ChannelName:  job.ChannelName,
		Filename:     filename,
		PathRelative: conf.ChannelPath(job.ChannelName, filename),
		Duration:     info.Duration,
		Width:        info.Width,
		Height:       info.Height,
		Size:         info.Size,
		BitRate:      info.BitRate,
		Packets:      info.PacketCount,
		CreatedAt:    time.Now(),
		Bookmark:     false,
	}

	err = newRec.Save("cut")
	if err != nil {
		log.Printf("[Job] Error creating: %v", err)
		return job, err
	}

	// Successfully added cut record, enqueue preview job
	_, err = database.EnqueuePreviewJob(job.ChannelName, filename)
	if err != nil {
		log.Printf("[Job] Error adding preview for cutting job %d: %v", job.JobId, err)
		return job, err
	}

	// Finished, destroy job
	err = job.Destroy()
	if err != nil {
		log.Printf("[Job] Error deleteing job: %v", err)
		return job, err
	}

	log.Printf("[Job] Cutting job complete for '%s'", job.Filepath)
	return job, nil
}

func GeneratePreviews(args *helpers.VideoConversionArgs) (*helpers.PreviewResult, error) {
	inputPath := filepath.Join(conf.AppCfg.RecordingsAbsolutePath, args.ChannelName, args.Filename)

	log.Println("---------------------------------------------- Preview Job ----------------------------------------------")
	log.Println(inputPath)
	log.Println("---------------------------------------------------------------------------------------------------------")

	return helpers.CreatePreview(args, inputPath, conf.AbsoluteDataPath(args.ChannelName), conf.FrameCount, 128, 256)
}

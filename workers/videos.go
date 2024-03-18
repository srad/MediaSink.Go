package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/helpers"
)

var (
	sleepBetweenRounds = 1 * time.Second
	ctx, cancel        = context.WithCancel(context.Background())
)

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
			if job, err := database.PreviewJobs(); err != nil {
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

			database.SendJobChannel("job:progress", database.JobMessage{JobId: job.JobId, Data: database.JobVideoInfo{Packets: job.Recording.Packets, Frame: info.Frame}, Type: job.Status, ChannelName: job.ChannelName, Filename: job.Filename})
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
	recording := &database.Recording{
		ChannelName:  job.ChannelName,
		Filename:     result.Filename,
		PathRelative: result.PathRelative,
		Bookmark:     false,
		CreatedAt:    result.CreatedAt,
	}
	// Also, when fails, destroy it, some reason it is foul.
	if err := recording.Save("recording"); err != nil {
		if err := os.Remove(result.Filepath); err != nil {
			log.Printf("Error deleting file '%s': %s", result.Filepath, err.Error())
			return job, err
		}
	} else {
		if _, err := recording.EnqueuePreviewJob(); err != nil {
			log.Printf("Error enqueing preview job: %s", err.Error())
			return job, err
		}
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

	outputVideo := &helpers.Video{FilePath: outputFile}
	info, err := outputVideo.GetVideoInfo()
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
	_, err = newRec.EnqueuePreviewJob()
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

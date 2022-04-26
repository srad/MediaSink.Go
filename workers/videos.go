package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/models"
	"github.com/srad/streamsink/patterns"
	"github.com/srad/streamsink/utils"
	"gorm.io/gorm"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	sleepBetweenRounds = 1 * time.Second
	cancelWorker       context.CancelFunc
	Dispatcher         = &patterns.Dispatcher[models.JobMessage[JobVideoInfo]]{}
)

type JobVideoInfo struct {
	Packets uint64 `json:"packets"`
	Frame   uint64 `json:"frame"`
}

func StartWorker() {
	ctx, c := context.WithCancel(context.Background())
	cancelWorker = c
	go processJobs(ctx)
}

func StopWorker() {
	cancelWorker()
}

func processJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[processJobs] Worker stopped")
			return
		case <-time.After(sleepBetweenRounds):
			conversionJobs()
			cuttingJobs()
			previewJobs()
		}
	}
}

func conversionJobs() {
	job, err := models.GetNextJob(models.StatusConvert)
	if job == nil {
		return
	}
	if err != nil {
		log.Printf("[Job] Error handlung job: %v", err)
		job.Destroy()
		return
	}

	job.Activate()

	result, err := utils.ConvertVideo(&utils.VideoConversionArgs{
		OnStart: func(info *utils.CommandInfo) {
			_ = job.UpdateInfo(info.Pid, info.Command)
		},
		OnProgress: func(info *utils.ProcessInfo) {
			job.UpdateProgress(fmt.Sprintf("%f", float32(info.Frame)/float32(job.Recording.Packets)*100))
			Dispatcher.Notify("job:progress", models.JobMessage[JobVideoInfo]{JobId: job.JobId, Data: JobVideoInfo{Packets: job.Recording.Packets, Frame: info.Frame}, Type: job.Status, ChannelName: job.ChannelName, Filename: job.Filename})
		},
		ChannelName: job.ChannelName,
		Filename:    job.Filename,
	}, *job.Args)

	if err != nil {
		log.Printf("[conversionJobs] Error converting '%s' to '%s': %s", job.Filename, *job.Args, err.Error())
		os.Remove(result.Filepath)
		job.Destroy()
		return
	} else {
		log.Printf("[conversionJobs] Completed conversion of '%s' with args '%s'", job.Filename, *job.Args)
	}

	if err := job.Destroy(); err != nil {
		log.Printf("[conversionJobs] Error deleting job: %s", err.Error())
	}

	// All good now, save the record.
	rec := &models.Recording{
		ChannelName:  job.ChannelName,
		Filename:     result.Filename,
		PathRelative: result.PathRelative,
		Bookmark:     false,
		CreatedAt:    result.CreatedAt,
	}
	// Also, when fails, destroy it, some reason it is foul.
	if err := rec.Save("recording"); err != nil {
		os.Remove(result.Filepath)
	} else {
		models.EnqueuePreviewJob(result.ChannelName, result.Filename)
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
	errDelete := models.DestroyPreviews(job.ChannelName, job.Filename)
	if errDelete != nil && errDelete != gorm.ErrRecordNotFound {
		log.Printf("[Job] Error deleting existing previews: %v", errDelete)
	}

	log.Printf("[Job] Generating preview for '%s'", job.Filename)
	err = job.Activate()
	if err != nil {
		log.Printf("[Job] Error activating job: %d", job.JobId)
	}

	err = GeneratePreviews(&utils.VideoConversionArgs{
		OnStart: func(info *utils.CommandInfo) {
			_ = job.UpdateInfo(info.Pid, info.Command)
			Dispatcher.Notify("job:start", models.JobMessage[JobVideoInfo]{
				JobId:       job.JobId,
				ChannelName: job.ChannelName,
				Filename:    job.Filename,
				Type:        job.Status,
			})
		},
		OnProgress: func(info *utils.ProcessInfo) {
			Dispatcher.Notify("job:progress", models.JobMessage[JobVideoInfo]{
				JobId:       job.JobId,
				ChannelName: job.ChannelName,
				Filename:    job.Filename,
				Data:        JobVideoInfo{Frame: info.Frame, Packets: job.Recording.Packets},
			})
		},
		ChannelName: job.ChannelName,
		Filename:    job.Filename,
	})
	if err != nil {
		// Delete the file if it is corrupted
		checkFileErr := CheckVideo(conf.GetRecordingsPaths(job.ChannelName, job.Filename).Filepath)
		if checkFileErr != nil {
			if rec, err := job.FindRecording(); err != nil {
				_ = rec.Destroy()
			}
			log.Printf("[Job] File corrupted, deleting '%s', %v\n", job.Filename, checkFileErr)
		}
		// Since the job failed for some reason, remove it
		_ = job.Destroy()
		log.Printf("[Job] Error generating preview for '%s' : %v\n", job.Filename, err)
		return
	}

	_, err2 := models.UpdatePreview(job.ChannelName, job.Filename)
	if err2 != nil {
		log.Printf("[Job] Error adding previews: %v", err2)
		return
	}

	if _, err := job.FindRecording(); err != nil {
		//TODO
		//services.notify("job:preview:done", models.JobMessage{JobId: job.JobId, ChannelName: job.ChannelName, Filename: job.Filename, Data: rec})
	}
	err3 := job.Destroy()
	if err3 != nil {
		log.Printf("[Job] Error deleteing job: %v", err3)
		return
	}

	log.Printf("[Job] Preview job complete for '%s'", job.Filepath)
}

// Cut video, add preview job, destroy job.
// This action is intrinsically procedural, keep it together locally.
func cuttingJobs() error {
	job, err := models.GetNextJob(models.StatusCut)
	if err == gorm.ErrRecordNotFound || job == nil {
		return err
	}

	if err != nil {
		log.Printf("[Job] Error handling cutting job: %v", err)
		return err
	}

	log.Printf("[Job] Generating preview for '%s'", job.Filename)
	err = job.Activate()
	if err != nil {
		log.Printf("[Job] Error activating job: %d", job.JobId)
	}

	if job.Args == nil {
		log.Printf("[Job] Error missing args for cutting job: %d", job.JobId)
		return err
	}

	// Parse arguments
	cutArgs := &utils.CutArgs{}
	s := []byte(*job.Args)
	err = json.Unmarshal(s, &cutArgs)
	if err != nil {
		log.Printf("[Job] Error parsing cutting job arguments: %v", err)
		_ = job.Destroy()
		return err
	}

	// Filenames
	now := time.Now()
	stamp := now.Format("2006_01_02_15_04_05")
	filename := fmt.Sprintf("%s_cut_%s.mp4", job.ChannelName, stamp)
	inputPath := conf.AbsoluteFilepath(job.ChannelName, job.Filename)
	outputFile := conf.AbsoluteFilepath(job.ChannelName, filename)
	segFiles := make([]string, len(cutArgs.Starts))
	mergeFileContent := make([]string, len(cutArgs.Starts))

	// Cut
	segmentFilename := fmt.Sprintf("%s_cut_%s", job.ChannelName, stamp)
	for i, start := range cutArgs.Starts {
		segFiles[i] = conf.AbsoluteFilepath(job.ChannelName, fmt.Sprintf("%s_%04d.mp4", segmentFilename, i))
		err = utils.CutVideo(&utils.CuttingJob{
			OnStart: func(info *utils.CommandInfo) {
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
			return err
		}
	}
	// Merge file txt, enumerate
	for i, file := range segFiles {
		mergeFileContent[i] = fmt.Sprintf("file '%s'", file)
	}
	mergeTextfile := conf.AbsoluteFilepath(job.ChannelName, fmt.Sprintf("%s.txt", segmentFilename))
	err = os.WriteFile(mergeTextfile, []byte(strings.Join(mergeFileContent, "\n")), 0644)
	if err != nil {
		log.Printf("[Job] Error writing concat text file '%s': %v", mergeTextfile, err)
		for _, file := range segFiles {
			if err := os.RemoveAll(file); err != nil {
				log.Printf("[Job] Error deleting %s: %v", file, err)
			}
		}
		_ = os.RemoveAll(mergeTextfile)
		_ = job.Destroy()
		return err
	}

	err = utils.MergeVideos(func(s string) { log.Printf("[MergeVideos] %s", s) }, mergeTextfile, outputFile)
	if err != nil {
		// Job failed, destroy all files.
		log.Printf("[Job] Error merging file '%s': %s", mergeTextfile, err.Error())
		for _, file := range segFiles {
			if err := os.RemoveAll(file); err != nil {
				log.Printf("[Job] Error deleting %s: %s", file, err.Error())
			}
		}
		_ = os.RemoveAll(mergeTextfile)
		_ = job.Destroy()
		return err
	}

	_ = os.RemoveAll(mergeTextfile)
	for _, file := range segFiles {
		log.Printf("[MergeJob] Deleting segment %s", file)
		if err := os.Remove(file); err != nil {
			log.Printf("[Job] Error deleting segment '%s': %s", file, err.Error())
		}
	}

	info, err := utils.GetVideoInfo(outputFile)
	if err != nil {
		log.Printf("[Job] Error reading video information for file '%s': %v", filename, err)
	}

	// Cutting written to dist, add record to database
	newRec := models.Recording{
		ChannelName:  job.ChannelName,
		Filename:     filename,
		PathRelative: conf.GetRelativeRecordingsPath(job.ChannelName, filename),
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
		return err
	}

	// Successfully added cut record, enqueue preview job
	_, err = models.EnqueuePreviewJob(job.ChannelName, filename)
	if err != nil {
		log.Printf("[Job] Error adding preview for cutting job %d: %v", job.JobId, err)
		return err
	}

	// Finished, destroy job
	err = job.Destroy()
	if err != nil {
		log.Printf("[Job] Error deleteing job: %v", err)
		return err
	}

	log.Printf("[Job] Cutting job complete for '%s'", job.Filepath)
	return nil
}

func CheckVideo(filepath string) error {
	return utils.ExecSync(&utils.ExecArgs{
		Command:     "ffmpeg",
		CommandArgs: []string{"-v", "error", "-i", filepath, "-f", "null", "-"},
	})
}

func GeneratePreviews(args *utils.VideoConversionArgs) error {
	inputPath := filepath.Join(conf.AppCfg.RecordingsAbsolutePath, args.ChannelName, args.Filename)

	log.Println("---------------------------------------------- Preview Job ----------------------------------------------")
	log.Println(inputPath)
	log.Println("---------------------------------------------------------------------------------------------------------")

	return utils.ExtractFrames(args, inputPath, conf.AbsoluteDataPath(args.ChannelName), conf.FrameCount, 128, 256)
}

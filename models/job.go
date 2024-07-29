package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/network"
	"gorm.io/gorm"
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
	jobChannel         = make(chan network.EventMessage, 1000)
	JobInfoChannel     = make(chan network.EventMessage, 1000)
	sleepBetweenRounds = 1 * time.Second
	ctx, cancel        = context.WithCancel(context.Background())
)

func StartWorker() {
	go processJobs(ctx)
}

func StopWorker() {
	cancel()
}

type JobVideoInfo struct {
	Packets uint64 `json:"packets"`
	Frame   uint64 `json:"frame"`
}

func SendJobChannel(name string, data interface{}) {
	go sendChannel(name, data)
}

func sendChannel(name string, data interface{}) {
	jobChannel <- network.EventMessage{Name: name, Message: data}
}

func DispatchJob(ctx context.Context) {
	for {
		select {
		case m := <-jobChannel:
			network.SendSocket(m.Name, m.Message)
			return
		case <-ctx.Done():
			log.Infoln("[dispatchMessages] stopped")
			return
		}
	}
}

type JobMessage struct {
	JobId       uint        `json:"jobId,omitempty" extensions:"!x-nullable"`
	RecordingId uint        `json:"recordingId,omitempty" extensions:"!x-nullable"`
	ChannelName string      `json:"channelName,omitempty" extensions:"!x-nullable"`
	Filename    string      `json:"filename,omitempty"`
	Type        string      `json:"type,omitempty"`
	Data        interface{} `json:"data,omitempty"`
}

type Job struct {
	Recording Recording `json:"-" gorm:"foreignKey:recording_id;references:recording_id;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Channel   Channel   `json:"-" gorm:"foreignKey:channel_id;references:channel_id;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	JobId uint `json:"jobId" gorm:"autoIncrement" extensions:"!x-nullable"`

	ChannelId   uint `json:"channelId" gorm:"column:channel_id;not null;default:null" extensions:"!x-nullable"`
	RecordingId uint `json:"recordingId" gorm:"column:recording_id;not null;default:null" extensions:"!x-nullable"`

	// Unique entry, this is the actual primary key
	ChannelName ChannelName `json:"channelName" gorm:"not null;default:null" extensions:"!x-nullable"`
	Filename    string      `json:"filename" gorm:"not null;default:null" extensions:"!x-nullable"`
	Status      string      `json:"status" gorm:"not null;default:null" extensions:"!x-nullable"`

	Filepath  string    `json:"pathRelative" gorm:"not null;default:null;" extensions:"!x-nullable"`
	Active    bool      `json:"active" gorm:"not null;default:false" extensions:"!x-nullable"`
	CreatedAt time.Time `json:"createdAt" gorm:"not null;default:current_timestamp;index:idx_create_at" extensions:"!x-nullable"`

	// Additional information
	Pid      *int    `json:"pid" gorm:"default:null"`
	Command  *string `json:"command" gorm:"default:null"`
	Progress *string `json:"progress" gorm:"default:null"`
	Info     *string `json:"info" gorm:"default:null"`
	Args     *string `json:"args" gorm:"default:null"`
}

func (recording *Recording) EnqueueRecordingJob(outputPath string) (*Job, error) {
	return addJob(recording.ChannelId, recording.RecordingId, recording.ChannelName, recording.Filename, outputPath, StatusRecording, nil)
}

func (recording *Recording) EnqueueConversionJob(mediaType string) (*Job, error) {
	filepath := recording.ChannelName.AbsoluteChannelFilePath(recording.Filename)
	return addJob(recording.ChannelId, recording.RecordingId, recording.ChannelName, recording.Filename, filepath, StatusConvert, &mediaType)
}

func (recording *Recording) EnqueuePreviewJob() (*Job, error) {
	return addJob(recording.ChannelId, recording.RecordingId, recording.ChannelName, recording.Filename, recording.ChannelName.AbsoluteChannelFilePath(recording.Filename), StatusPreview, nil)
}

func (recording *Recording) EnqueueCuttingJob(intervals string) (*Job, error) {
	filepath := recording.ChannelName.AbsoluteChannelFilePath(recording.Filename)
	return addJob(recording.ChannelId, recording.RecordingId, recording.ChannelName, recording.Filename, filepath, StatusCut, &intervals)
}

func addJob(channelId, recordingId uint, channelName ChannelName, filename, filepath, status string, args *string) (*Job, error) {
	job := Job{
		ChannelId:   channelId,
		RecordingId: recordingId,
		ChannelName: channelName,
		Filename:    filename,
		Filepath:    filepath,
		Status:      status,
		Args:        args,
		Active:      false,
		CreatedAt:   time.Now(),
	}

	if err := Db.Create(&job).Error; err != nil {
		log.Errorf("[Job] Error enqueing job: '%s/%s' -> %s: %s", channelName, filename, status, err)
		return &job, err
	}
	log.Infof("[Job] Enqueued job: '%s/%s' -> %s", channelName, filename, status)

	SendJobChannel("job:create", JobMessage{JobId: job.JobId, Type: status, ChannelName: job.ChannelName.String(), Filename: job.Filename})

	return &job, nil
}

func JobList() ([]*Job, error) {
	var jobs []*Job
	if err := Db.
		Order("jobs.created_at ASC").
		Find(&jobs).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return jobs, nil
}

func (channel *Channel) Jobs() ([]*Job, error) {
	var jobs []*Job
	if err := Db.Where("channel_id = ?", channel.ChannelId).Find(&jobs).Error; err != nil {
		return nil, err
	}

	return jobs, nil
}

func (job *Job) Destroy() error {
	if job.Pid != nil {
		if err := helpers.Interrupt(*job.Pid); err != nil {
			log.Errorf("[Destroy] Error interrupting process: %s", err)
			return err
		}
	}

	if err := Db.Table("jobs").Where("job_id = ?", job.JobId).Delete(Job{}).Error; err != nil {
		return err
	}
	log.Infof("[Job] Job id delete %d", job.JobId)

	SendJobChannel("job:destroy", JobMessage{JobId: job.JobId, ChannelName: job.ChannelName.String(), Filename: job.Filename})

	return nil
}

func FindJobById(id int) (*Job, error) {
	var job *Job
	if err := Db.Where("job_id = ?", id).Find(&job).Error; err != nil {
		return nil, err
	}

	return job, nil
}

func GetJobsByStatus(status string) ([]*Job, error) {
	var jobs []*Job
	if err := Db.Where("status = ?", status).Find(&jobs).Error; err != nil {
		return nil, err
	}

	return jobs, nil
}

func GetNextJob(status string) (*Job, error) {
	var job *Job
	err := Db.Where("status = ?", status).
		Joins("Recording").
		Order("jobs.created_at asc").First(&job).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return job, err
}

func UpdateJobStatus(jobId uint, status string) error {
	return Db.Model(&Job{}).Where("job_id = ?", jobId).Update("status", status).Error
}

func (job *Job) UpdateInfo(pid int, command string) error {
	return Db.Model(&Job{}).Where("job_id = ?", job.JobId).
		Update("pid", pid).
		Update("command", command).Error
}

func (job *Job) UpdateProgress(progress string) error {
	return Db.Model(&Job{}).Where("job_id = ?", job.JobId).
		Update("progress", progress).Error
}

func UpdateJobInfo(jobId uint, info string) error {
	return Db.Model(&Job{}).Where("job_id = ?", jobId).
		Update("info", info).Error
}

func (job *Job) Activate() error {
	if err := Db.Model(&Job{}).Where("job_id = ?", job.JobId).Update("active", true).Error; err != nil {
		return err
	}

	SendJobChannel("job:active", JobMessage{
		JobId:       job.JobId,
		RecordingId: job.RecordingId,
		ChannelName: job.ChannelName.String(),
		Filename:    job.Filename,
		Type:        job.Status,
	})

	return nil
}

// PreviewJobs Handles one single job.
func previewJobs() error {
	job, errNextJob := GetNextJob(StatusPreview)
	if job == nil {
		return errNextJob
	}

	recording := &Recording{ChannelId: job.ChannelId, ChannelName: job.ChannelName, Filename: job.Filename}
	log.Infof("[Job] Recording: %v", job)

	// Delete any old previews first
	if err := recording.DestroyPreviews(); err != nil {
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

			SendJobChannel("job:start", JobMessage{
				JobId:       job.JobId,
				RecordingId: job.RecordingId,
				ChannelName: job.ChannelName.String(),
				Filename:    job.Filename,
				Type:        job.Status,
			})
		},
		OnProgress: func(info *helpers.ProcessInfo) {
			SendJobChannel("job:progress", JobMessage{
				JobId:       job.JobId,
				RecordingId: job.RecordingId,
				ChannelName: job.ChannelName.String(),
				Filename:    job.Filename,
				Data:        JobVideoInfo{Frame: info.Frame, Packets: job.Recording.Packets}})
		},
		InputPath:  job.ChannelName.AbsoluteChannelPath(),
		OutputPath: job.ChannelName.AbsoluteChannelDataPath(),
		Filename:   job.Filename,
	}

	// 1. First check if input file is corrupt.
	if _, err := helpers.GeneratePreviews(conversion); err != nil {
		// 2. Delete the file if it is corrupted
		checkFileErr := helpers.CheckVideo(job.ChannelName.GetRecordingsPaths(job.Filename).Filepath)
		if checkFileErr != nil {
			if rec, errJob := FindRecording(job.RecordingId); errJob != nil && rec != nil {
				_ = rec.Destroy()
			}
			log.Errorf("[Job] File corrupted, deleting '%s', %v", job.Filename, checkFileErr)
		}
		// 3. Since the job failed for some reason, remove it
		_ = job.Destroy()

		return fmt.Errorf("error generating preview for '%s' : %s", job.Filename, err)
	}

	// 4. Went ok.
	if err := recording.AddPreviews(); err != nil {
		return fmt.Errorf("error adding previews: %v", err)
	}

	if _, err := FindRecording(job.RecordingId); err != nil {
		SendJobChannel("job:preview:done", JobMessage{JobId: job.JobId, ChannelName: job.ChannelName.String(), Filename: job.Filename})
	}

	if errDestroy := job.Destroy(); errDestroy != nil {
		return fmt.Errorf("error deleting job: %v", errDestroy)
	}

	return nil
}

func conversionJobs() error {
	job, err := GetNextJob(StatusConvert)
	if job == nil {
		return err
	}

	if err := job.Activate(); err != nil {
		log.Errorf("Error activating job: %s", err)
	}

	result, err := helpers.ConvertVideo(&helpers.VideoConversionArgs{
		OnStart: func(info *helpers.CommandInfo) {
			_ = job.UpdateInfo(info.Pid, info.Command)
		},
		OnProgress: func(info *helpers.ProcessInfo) {
			if err := job.UpdateProgress(fmt.Sprintf("%f", float32(info.Frame)/float32(job.Recording.Packets)*100)); err != nil {
				log.Errorf("Error updating job progress: %s", err)
			}

			SendJobChannel("job:progress", JobMessage{JobId: job.JobId, Data: JobVideoInfo{Packets: job.Recording.Packets, Frame: info.Frame}, Type: job.Status, ChannelName: job.ChannelName.String(), Filename: job.Filename})
		},
		InputPath:  job.ChannelName.AbsoluteChannelPath(),
		Filename:   job.Filename,
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

	// All good now, save the record.
	recording := &Recording{
		ChannelId:    job.ChannelId,
		ChannelName:  job.ChannelName,
		Filename:     result.Filename,
		PathRelative: job.ChannelName.ChannelPath(result.Filename),
	}
	// Also, when fails, destroy it, some reason it is foul.
	if err := recording.Create(); err != nil {
		if errRemove := os.Remove(result.Filepath); errRemove != nil {
			log.Errorf("Error deleting file '%s': %s", result.Filepath, errRemove)
			return errRemove
		}
	} else {
		if _, errJob := recording.EnqueuePreviewJob(); errJob != nil {
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
	job, err := GetNextJob(StatusCut)
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
	filename := fmt.Sprintf("%s_cut_%s.mp4", job.ChannelName, stamp)
	inputPath := job.ChannelName.AbsoluteChannelFilePath(job.Filename)
	outputFile := job.ChannelName.AbsoluteChannelFilePath(filename)
	segFiles := make([]string, len(cutArgs.Starts))
	mergeFileContent := make([]string, len(cutArgs.Starts))

	// Cut
	segmentFilename := fmt.Sprintf("%s_cut_%s", job.ChannelName, stamp)
	for i, start := range cutArgs.Starts {
		segFiles[i] = job.ChannelName.AbsoluteChannelFilePath(fmt.Sprintf("%s_%04d.mp4", segmentFilename, i))
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
	mergeTextFile := job.ChannelName.AbsoluteChannelFilePath(fmt.Sprintf("%s.txt", segmentFilename))
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

	err = helpers.MergeVideos(func(s string) { log.Errorf("[MergeVideos] %s\n", s) }, mergeTextFile, outputFile)
	if err != nil {
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
	info, err := outputVideo.GetVideoInfo()
	if err != nil {
		log.Errorf("[Job] Error reading video information for file '%s': %s", filename, err)
	}

	// Cutting written to dist, add record to models
	newRec := Recording{
		RecordingId:  job.RecordingId,
		ChannelName:  job.ChannelName,
		Filename:     filename,
		PathRelative: job.ChannelName.ChannelPath(filename),
		Duration:     info.Duration,
		Width:        info.Width,
		Height:       info.Height,
		Size:         info.Size,
		BitRate:      info.BitRate,
		Packets:      info.PacketCount,
		CreatedAt:    time.Now(),
		Bookmark:     false,
		VideoType:    "cut",
	}

	err = newRec.Create()
	if err != nil {
		log.Errorf("[Job] Error creating: %s\n", err)
		return err
	}

	// Successfully added cut record, enqueue preview job
	_, err = newRec.EnqueuePreviewJob()
	if err != nil {
		log.Errorf("[Job] Error adding preview for cutting job %d: %s", job.JobId, err)
		return err
	}

	// Finished, destroy job
	err = job.Destroy()
	if err != nil {
		log.Errorf("[Job] Error deleteing job: %s", err)
		return err
	}

	log.Infof("[Job] Cutting job complete for '%s'", job.Filepath)
	return nil
}

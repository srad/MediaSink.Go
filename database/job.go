package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/srad/streamsink/network"
	"time"

	"gorm.io/gorm/clause"

	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/helpers"
	"gorm.io/gorm"
)

const (
	TaskConvert        JobTask   = "convert"
	TaskPreviewCover   JobTask   = "preview-cover"
	TaskPreviewStrip   JobTask   = "preview-stripe"
	TaskPreviewVideo   JobTask   = "preview-video"
	TaskCut            JobTask   = "cut"
	StatusJobCompleted JobStatus = "completed"
	StatusJobOpen      JobStatus = "open"
	StatusJobError     JobStatus = "error"
	StatusJobCanceled  JobStatus = "canceled"
	JobOrderASC        JobOrder  = "ASC"
	JobOrderDESC       JobOrder  = "DESC"
)

type JobTask string
type JobStatus string
type JobOrder string

type Job struct {
	Channel   Channel   `json:"-" gorm:"foreignKey:channel_id;references:channel_id;"`
	Recording Recording `json:"-" gorm:"foreignKey:recording_id;references:recording_id"`

	JobID uint `json:"jobId" gorm:"autoIncrement;primaryKey" extensions:"!x-nullable"`

	ChannelID   ChannelID   `json:"channelId" gorm:"column:channel_id;not null;default:null" extensions:"!x-nullable"`
	RecordingID RecordingID `json:"recordingId" gorm:"column:recording_id;not null;default:null" extensions:"!x-nullable"`

	// Unique entry, this is the actual primary key
	ChannelName ChannelName       `json:"channelName" gorm:"not null;default:null" extensions:"!x-nullable"`
	Filename    RecordingFileName `json:"filename" gorm:"not null;default:null" extensions:"!x-nullable"`

	// Default values only not to break migrations.
	Task   JobTask   `json:"task" gorm:"not null;default:preview" extensions:"!x-nullable"`
	Status JobStatus `json:"status" gorm:"not null;default:completed" extensions:"!x-nullable"`

	Filepath    string     `json:"filepath" gorm:"not null;default:null;" extensions:"!x-nullable"`
	Active      bool       `json:"active" gorm:"not null;default:false" extensions:"!x-nullable"`
	CreatedAt   time.Time  `json:"createdAt" gorm:"not null;default:current_timestamp;index:idx_create_at" extensions:"!x-nullable"`
	StartedAt   *time.Time `json:"startedAt" gorm:"default:null" extensions:"!x-nullable"`
	CompletedAt *time.Time `json:"completedAt" gorm:"default:null" extensions:"!x-nullable"`

	// Additional information
	Pid      *int    `json:"pid" gorm:"default:null"`
	Command  *string `json:"command" gorm:"default:null"`
	Progress *string `json:"progress" gorm:"default:null"`
	Info     *string `json:"info" gorm:"default:null"`
	Args     *string `json:"args" gorm:"default:null"`
}

func (job *Job) CreateJob() error {
	return DB.Create(job).Error
}

func JobList(skip, take int, status []JobStatus, order JobOrder) ([]*Job, int64, error) {
	var count int64 = 0
	if err := DB.Model(&Job{}).
		Where("status IN (?)", status).
		Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var jobs []*Job
	if err := DB.
		Model(&Job{}).
		Where("status IN (?)", status).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "created_at"}, Desc: order == JobOrderDESC}).
		Offset(skip).
		Limit(take).
		Find(&jobs).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, count, err
	}

	return jobs, count, nil
}

func (channel *Channel) Jobs() ([]*Job, error) {
	var jobs []*Job
	if err := DB.Model(&Job{}).
		Where("channel_id = ?", channel.ChannelID).
		Find(&jobs).Error; err != nil {
		return nil, err
	}

	return jobs, nil
}

func (job *Job) Cancel(reason string) error {
	return job.updateStatus(StatusJobCanceled, &reason)
}

func (job *Job) Completed() error {
	err1 := job.updateStatus(StatusJobCompleted, nil)
	err2 := DB.Model(&Job{}).Where("job_id = ?", job.JobID).
		Update("completed_at", time.Now()).Error

	return errors.Join(err1, err2)
}

func (job *Job) Error(reason error) error {
	err := reason.Error()
	return job.updateStatus(StatusJobError, &err)
}

func (job *Job) updateStatus(status JobStatus, reason *string) error {
	if job.JobID == 0 {
		return errors.New("invalid job id")
	}

	if job.Pid != nil {
		if err := helpers.Interrupt(*job.Pid); err != nil {
			log.Errorf("[Destroy] Error interrupting process: %s", err)
			return err
		}
	}

	return DB.Model(&Job{}).Where("job_id = ?", job.JobID).
		Updates(map[string]interface{}{"status": status, "info": reason, "active": false}).Error
}

func JobExists(recordingID RecordingID, task JobTask) (*Job, bool, error) {
	if recordingID == 0 {
		return nil, false, errors.New("recording id is 0")
	}

	var job *Job
	result := DB.Model(&Job{}).Where("recording_id = ? AND task = ? AND status = ?", recordingID, task, StatusJobOpen).First(&job)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if result.Error != nil {
		return nil, false, result.Error
	}

	return job, result.RowsAffected > 0, nil
}

// DeleteJob If an existing PID is assigned to the job,
// first the process is try-killed and then the job deleted.
func DeleteJob(id uint) error {
	if id == 0 {
		return fmt.Errorf("invalid job id: %d", id)
	}

	var job *Job
	if err := DB.Where("job_id = ?", id).First(&job).Error; err != nil {
		return err
	}

	if job.Pid != nil {
		if err := helpers.Interrupt(*job.Pid); err != nil {
			log.Errorf("[Destroy] Error interrupting process: %s", err)
			return err
		}
	}

	if err := DB.Model(&Job{}).
		Where("job_id = ?", id).
		Delete(Job{}).Error; err != nil {
		return err
	}

	return nil
}

// GetNextJob Any job is attached to a recording which it will process.
// The caller must know which type the JSON serialized argument originally had.
func GetNextJob() (*Job, error) {
	var job *Job
	err := DB.Where("status = ? AND active = ?", StatusJobOpen, false).
		Preload("Channel").
		Preload("Recording").
		Order("jobs.created_at ASC").
		First(&job).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return job, err
}

func UnmarshalJobArg[T any](job *Job) (*T, error) {
	// Deserialize the arguments, if existent.
	if job.Args != nil && *job.Args != "" {
		var data *T
		if err := json.Unmarshal([]byte(*job.Args), &data); err != nil {
			log.Errorf("[Job] Error parsing cutting job arguments: %s", err)
			if errDestroy := job.Error(err); errDestroy != nil {
				log.Errorf("[Job] Error destroying job: %s", errDestroy)
			}
			return nil, err
		}
		return data, nil
	}

	return nil, errors.New("job arg nil or empty")
}

// GetNextJobTask Any job is attached to a recording which it will process.
// The caller must know which type the JSON serialized argument originally had.
func GetNextJobTask[T any](task JobTask) (*Job, *T, error) {
	var job *Job
	err := DB.Where("task = ? AND status = ? AND active = ?", task, StatusJobOpen, false).
		Order("jobs.created_at asc").
		First(&job).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}

	// Deserialize the arguments, if existent.
	if job.Args != nil && *job.Args != "" {
		var data *T
		if err := json.Unmarshal([]byte(*job.Args), &data); err != nil {
			log.Errorf("[Job] Error parsing cutting job arguments: %s", err)
			if errDestroy := job.Error(err); errDestroy != nil {
				log.Errorf("[Job] Error destroying job: %s", errDestroy)
			}
			return job, nil, err
		}
		return job, data, err
	}

	return job, nil, err
}

func (job *Job) UpdateInfo(pid int, command string) error {
	if job.JobID == 0 {
		return errors.New("invalid job id")
	}

	return DB.Model(&Job{}).Where("job_id = ?", job.JobID).
		Update("pid", pid).
		Update("command", command).Error
}

func (job *Job) UpdateProgress(progress string) error {
	if job.JobID == 0 {
		return errors.New("invalid job id")
	}

	return DB.Model(&Job{}).Where("job_id = ?", job.JobID).
		Update("progress", progress).Error
}

func (job *Job) Activate() error {
	if job.JobID == 0 {
		return errors.New("invalid job id")
	}

	return DB.Model(&Job{}).Where("job_id = ?", job.JobID).Updates(map[string]interface{}{"started_at": time.Now(), "active": true}).Error
}

func (job *Job) Deactivate() error {
	if job.JobID == 0 {
		return errors.New("invalid job id")
	}

	return DB.Model(&Job{}).Where("job_id = ?", job.JobID).Update("active", false).Error
}

func CreateJob[T any](recording *Recording, task JobTask, args *T) (*Job, error) {
	data := ""
	if args != nil {
		bytes, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		data = string(bytes)
	}

	job := &Job{
		ChannelID:   recording.ChannelID,
		ChannelName: recording.ChannelName,
		RecordingID: recording.RecordingID,
		Filename:    recording.Filename,
		Filepath:    recording.ChannelName.AbsoluteChannelFilePath(recording.Filename),
		Status:      StatusJobOpen,
		Task:        task,
		Args:        &data,
		Active:      false,
		CreatedAt:   time.Now(),
	}

	err := job.CreateJob()

	return job, err
}

func (recording *Recording) EnqueueConversionJob(mediaType string) (*Job, error) {
	return enqueueJob[string](recording, TaskConvert, &mediaType)
}

func (recording *Recording) EnqueuePreviewsJob() (*Job, *Job, *Job, error) {
	job1, err1 := recording.EnqueuePreviewCoverJob()
	job2, err2 := recording.EnqueuePreviewStripeJob()
	job3, err3 := recording.EnqueuePreviewVideoJob()

	return job1, job2, job3, errors.Join(err1, err2, err3)
}

func (recording *Recording) EnqueuePreviewStripeJob() (*Job, error) {
	job, exists, err := JobExists(recording.RecordingID, TaskPreviewStrip)
	if err != nil {
		return job, err
	}
	if exists {
		return job, nil
	}
	return enqueueJob[*any](recording, TaskPreviewStrip, nil)
}

func (recording *Recording) EnqueuePreviewCoverJob() (*Job, error) {
	job, exists, err := JobExists(recording.RecordingID, TaskPreviewCover)
	if err != nil {
		return job, err
	}
	if exists {
		return job, nil
	}
	return enqueueJob[*any](recording, TaskPreviewCover, nil)
}

func (recording *Recording) EnqueuePreviewVideoJob() (*Job, error) {
	job, exists, err := JobExists(recording.RecordingID, TaskPreviewVideo)
	if err != nil {
		return job, err
	}
	if exists {
		return job, nil
	}
	return enqueueJob[*any](recording, TaskPreviewVideo, nil)
}

func (recording *Recording) EnqueueCuttingJob(args *helpers.CutArgs) (*Job, error) {
	return enqueueJob(recording, TaskCut, args)
}

func enqueueJob[T any](recording *Recording, task JobTask, args *T) (*Job, error) {
	if job, err := CreateJob(recording, task, args); err != nil {
		return nil, err
	} else {
		network.BroadCastClients(network.JobCreateEvent, job)
		return job, nil
	}
}

func EnqueueCuttingJob(id uint, args *helpers.CutArgs) (*Job, error) {
	if rec, err := RecordingID(id).FindRecordingByID(); err != nil {
		return nil, err
	} else {
		if job, err := rec.EnqueueCuttingJob(args); err != nil {
			return nil, err
		} else {
			return job, nil
		}
	}
}

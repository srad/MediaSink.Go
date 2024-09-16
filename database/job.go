package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/helpers"
	"gorm.io/gorm"
)

const (
	TaskConvert        JobTask   = "convert"
	TaskPreview        JobTask   = "preview"
	TaskCut            JobTask   = "cut"
	StatusJobCompleted JobStatus = "completed"
	StatusOpen         JobStatus = "open"
	StatusError        JobStatus = "error"
	StatusJobCanceled  JobStatus = "canceled"
)

type Job struct {
	Channel   Channel   `json:"-" gorm:"foreignKey:channel_id;references:channel_id;"`
	Recording Recording `json:"-" gorm:"foreignKey:recording_id;references:recording_id"`

	JobId uint `json:"jobId" gorm:"autoIncrement;primaryKey" extensions:"!x-nullable"`

	ChannelId   ChannelId   `json:"channelId" gorm:"column:channel_id;not null;default:null" extensions:"!x-nullable"`
	RecordingId RecordingId `json:"recordingId" gorm:"column:recording_id;not null;default:null" extensions:"!x-nullable"`

	// Unique entry, this is the actual primary key
	ChannelName ChannelName       `json:"channelName" gorm:"not null;default:null" extensions:"!x-nullable"`
	Filename    RecordingFileName `json:"filename" gorm:"not null;default:null" extensions:"!x-nullable"`

	// Default values only not to break migrations.
	Task   JobTask   `json:"task" gorm:"not null;default:preview" extensions:"!x-nullable"`
	Status JobStatus `json:"status" gorm:"not null;default:completed" extensions:"!x-nullable"`

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

type JobTask string
type JobStatus string

func (job *Job) CreateJob() error {
	return Db.Create(job).Error
}

func JobList(skip, take int) ([]*Job, int64, error) {
	var count int64 = 0
	if err := Db.Model(&Job{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}

	var jobs []*Job
	if err := Db.Order("jobs.created_at ASC").
		Offset(skip).
		Limit(take).
		Find(&jobs).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, count, err
	}

	return jobs, count, nil
}

func (channel *Channel) Jobs() ([]*Job, error) {
	var jobs []*Job
	if err := Db.Model(&Job{}).
		Where("channel_id = ?", channel.ChannelId).
		Find(&jobs).Error; err != nil {
		return nil, err
	}

	return jobs, nil
}

func (job *Job) Cancel(reason string) error {
	return job.updateStatus(StatusJobCanceled, &reason)
}

func (job *Job) Completed() error {
	return job.updateStatus(StatusJobCompleted, nil)
}

func (job *Job) Error(reason error) error {
	err := reason.Error()
	return job.updateStatus(StatusError, &err)
}

func (job *Job) updateStatus(status JobStatus, reason *string) error {
	if job.JobId == 0 {
		return errors.New("invalid job id")
	}

	if job.Pid != nil {
		if err := helpers.Interrupt(*job.Pid); err != nil {
			log.Errorf("[Destroy] Error interrupting process: %s", err)
			return err
		}
	}

	return Db.Model(&Job{}).Where("job_id = ?", job.JobId).
		Updates(map[string]interface{}{"status": status, "info": reason, "active": false}).Error
}

// DeleteJob If an existing PID is assigned to the job,
// first the process is try-killed and then the job deleted.
func DeleteJob(id uint) error {
	if id == 0 {
		return fmt.Errorf("invalid job id: %d", id)
	}

	var job *Job
	if err := Db.Where("job_id = ?", id).First(&job).Error; err != nil {
		return err
	}

	if job.Pid != nil {
		if err := helpers.Interrupt(*job.Pid); err != nil {
			log.Errorf("[Destroy] Error interrupting process: %s", err)
			return err
		}
	}

	if err := Db.Model(&Job{}).
		Where("job_id = ?", id).
		Delete(Job{}).Error; err != nil {
		return err
	}

	return nil
}

// GetNextJob Any job is attached to a recording which it will process.
// The caller must know which type the JSON serialized argument originally had.
func GetNextJob[T any](task JobTask) (*Job, *T, error) {
	var job *Job
	err := Db.Where("task = ? AND status = ?", task, StatusOpen).
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
	if job.JobId == 0 {
		return errors.New("invalid job id")
	}

	return Db.Model(&Job{}).Where("job_id = ?", job.JobId).
		Update("pid", pid).
		Update("command", command).Error
}

func (job *Job) UpdateProgress(progress string) error {
	if job.JobId == 0 {
		return errors.New("invalid job id")
	}

	return Db.Model(&Job{}).Where("job_id = ?", job.JobId).
		Update("progress", progress).Error
}

func (job *Job) Activate() error {
	if job.JobId == 0 {
		return errors.New("invalid job id")
	}

	return Db.Model(&Job{}).Where("job_id = ?", job.JobId).Update("active", true).Error
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
		ChannelId:   recording.ChannelId,
		ChannelName: recording.ChannelName,
		RecordingId: recording.RecordingId,
		Filename:    recording.Filename,
		Filepath:    recording.ChannelName.AbsoluteChannelFilePath(recording.Filename),
		Status:      StatusOpen,
		Task:        task,
		Args:        &data,
		Active:      false,
		CreatedAt:   time.Now(),
	}

	err := job.CreateJob()

	return job, err
}

package database

import (
	"encoding/json"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/helpers"
	"gorm.io/gorm"
)

type JobVideoInfo struct {
	Total   uint64 `json:"total"`
	Current uint64 `json:"current"`
	Task    string `json:"task"`
}

type Job struct {
	Channel   Channel   `json:"-" gorm:"foreignKey:channel_id;references:channel_id;"`
	Recording Recording `json:"-" gorm:"foreignKey:recording_id;references:recording_id"`

	JobId uint `json:"jobId" gorm:"autoIncrement" extensions:"!x-nullable"`

	ChannelId   ChannelId   `json:"channelId" gorm:"column:channel_id;not null;default:null" extensions:"!x-nullable"`
	RecordingId RecordingId `json:"recordingId" gorm:"column:recording_id;not null;default:null" extensions:"!x-nullable"`

	// Unique entry, this is the actual primary key
	ChannelName ChannelName       `json:"channelName" gorm:"not null;default:null" extensions:"!x-nullable"`
	Filename    RecordingFileName `json:"filename" gorm:"not null;default:null" extensions:"!x-nullable"`
	Status      string            `json:"status" gorm:"not null;default:null" extensions:"!x-nullable"`

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

func (job *Job) CreateJob() error {
	return Db.Create(&job).Error
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

	return nil
}

func FindJobById(id uint) (*Job, error) {
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

// GetNextJob Any job is attached to a recording which it will process.
func GetNextJob[T any](status string) (*Job, *T, error) {
	var job *Job
	err := Db.Where("status = ?", status).
		Order("jobs.created_at asc").First(&job).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}

	// Deserialize the arguments, if existent.
	if job.Args != nil {
		var data *T
		if err := json.Unmarshal([]byte(*job.Args), &data); err != nil {
			log.Errorf("[Job] Error parsing cutting job arguments: %s", err)
			if errDestroy := job.Destroy(); errDestroy != nil {
				log.Errorf("[Job] Error destroying job: %s", errDestroy)
			}
			return job, nil, err
		}
		return job, data, err
	}

	return job, nil, err
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
	return Db.Model(&Job{}).Where("job_id = ?", job.JobId).Update("active", true).Error
}

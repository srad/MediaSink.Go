package database

import (
	"errors"
	"log"
	"time"

	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/entities"
	"github.com/srad/streamsink/utils"
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
	JobChannel = make(chan entities.EventMessage)
)

type JobMessage struct {
	JobId       uint        `json:"jobId,omitempty" extensions:"!x-nullable"`
	ChannelName string      `json:"channelName,omitempty" extensions:"!x-nullable"`
	Filename    string      `json:"filename,omitempty"`
	Type        string      `json:"type,omitempty"`
	Data        interface{} `json:"data,omitempty"`
}

type Job struct {
	Recording Recording `json:"-" gorm:"foreignKey:ChannelName,Filename;references:channel_name,Filename;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	JobId     uint      `json:"jobId" gorm:"autoIncrement"`
	Channel   Channel   `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:channel_name;references:channel_name"`

	// Unique entry, this is the actual primary key
	ChannelName string `json:"channelName" gorm:"not null"`
	Filename    string `json:"filename" gorm:"not null"`
	Status      string `json:"status" gorm:"not null"`

	Filepath  string    `json:"pathRelative" gorm:"not null;"`
	Active    bool      `json:"active" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"createdAt" gorm:"not null;;index:idx_create_at"`

	// Additional information
	Pid      int     `json:"pid" gorm:"default:null"`
	Command  *string `json:"command" gorm:"default:null"`
	Progress *string `json:"progress" gorm:"default:null"`
	Info     *string `json:"info" gorm:"default:null"`
	Args     *string `json:"args" gorm:"default:null"`
}

func EnqueueRecordingJob(channelName, filename, filepath string) (*Job, error) {
	return addJob(channelName, filename, filepath, StatusRecording, nil)
}

func EnqueueConversionJob(channelName, filename, filepath, mediaType string) (*Job, error) {
	return addJob(channelName, filename, filepath, StatusConvert, &mediaType)
}

func EnqueuePreviewJob(channelName, filename string) (*Job, error) {
	return addJob(channelName, filename, conf.AbsoluteFilepath(channelName, filename), StatusPreview, nil)
}

func EnqueueCuttingJob(channelName, filename, filepath, intervals string) (*Job, error) {
	return addJob(channelName, filename, filepath, StatusCut, &intervals)
}

func (job *Job) FindRecording() (*Recording, error) {
	recording := Recording{}

	err := Db.Model(&Recording{}).
		Where("channel_name = ? AND filename = ?", job.ChannelName, job.Filename).
		First(&recording).Error

	return &recording, err
}

func addJob(channelName, filename, filepath, status string, args *string) (*Job, error) {
	job := Job{
		ChannelName: channelName,
		Filename:    filename,
		Filepath:    filepath,
		Status:      status,
		Args:        args,
		Active:      false,
		CreatedAt:   time.Now(),
	}

	if err := Db.Create(&job).Error; err != nil {
		log.Printf("[Job] Error enqueing job: '%s/%s' -> %s: %v", channelName, filename, status, err)
		return &job, err
	}
	log.Printf("[Job] Enqueued job: '%s/%s' -> %s", channelName, filename, status)

	JobChannel <- entities.EventMessage{Name: "job:create", Message: JobMessage{JobId: job.JobId, Type: status, ChannelName: job.ChannelName, Filename: job.Filename}}

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
	if err := Db.Where("channel_name = ?", channel.ChannelName).Find(&jobs).Error; err != nil {
		return nil, err
	}

	return jobs, nil
}

func (job *Job) Destroy() error {
	if job.Pid != 0 {
		if err := utils.Interrupt(job.Pid); err != nil {
			log.Printf("[Destroy] Error interrupting process: %s", err.Error())
			return err
		}
	}

	if err := Db.Table("jobs").Where("job_id = ?", job.JobId).Delete(Job{}).Error; err != nil {
		return err
	}
	log.Printf("[Job] Job id delete %d", job.JobId)

	JobChannel <- entities.EventMessage{Name: "job:destroy", Message: JobMessage{JobId: job.JobId, ChannelName: job.ChannelName, Filename: job.Filename}}

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
	if err != nil {
		return nil, err
	}

	return job, nil
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
	return Db.Model(&Job{}).Where("job_id = ?", job.JobId).
		Update("active", true).Error
}

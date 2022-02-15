package models

import (
	"github.com/srad/streamsink/conf"
	"gorm.io/gorm"
	"log"
	"time"
)

const (
	// StatusRecording Recording in progress
	StatusRecording = "recording"
	// StatusPreview Generating preview currently
	StatusPreview = "preview"
	StatusCut     = "cut"
)

var (
	dispatch = dispatcher{}
)

type dispatcher struct {
	listeners []func(JobMessage)
}

type JobMessage struct {
	Event       string
	ChannelName string
}

func ObserveJobs(f func(JobMessage)) {
	dispatch.listeners = append(dispatch.listeners, f)
}

func notify(msg JobMessage) {
	for _, f := range dispatch.listeners {
		f(msg)
	}
}

type Job struct {
	Recording Recording `json:"-" gorm:"foreignKey:ChannelName,Filename;References:ChannelName,Filename;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	JobId     uint      `json:"jobId" gorm:"primaryKey;AUTO_INCREMENT"`
	Channel   Channel   `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:ChannelName"`

	// Unique entry, this is the actual primary key
	ChannelName string `json:"channelName" gorm:"not null;default:null;index:unique_entry,unique"`
	Filename    string `json:"filename" gorm:"not null;default:null;index:unique_entry,unique"`
	Status      string `json:"status" gorm:"not null;default:null;index:idx_status;index:unique_entry,unique"`

	Filepath  string    `json:"pathRelative" gorm:"not null;default:null"`
	Active    bool      `json:"active" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"createdAt" gorm:"not null;default:null;index:idx_create_at"`

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

	notify(JobMessage{ChannelName: channelName, Event: status})

	return &job, nil
}

func JobList() ([]*Job, error) {
	var jobs []*Job
	if err := Db.Find(&jobs).Error; err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return jobs, nil
}

func (job *Job) Destroy() error {
	err := Db.Table("jobs").Where("job_id = ?", job.JobId).Delete(Job{}).Error
	if err != nil {
		return err
	}
	log.Printf("[Job] Job id delete %d", job.JobId)

	notify(JobMessage{ChannelName: job.ChannelName, Event: "destroyJob"})

	return nil
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
		Order("created_at asc").First(&job).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
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

func ActiveJob(jobId uint) error {
	return Db.Model(&Job{}).Where("job_id = ?", jobId).
		Update("active", true).Error
}

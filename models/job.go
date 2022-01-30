package models

import (
	"time"

	"github.com/srad/streamsink/conf"
	"gorm.io/gorm"
)

const (
	// StatusRecording Recording in progress
	StatusRecording = "recording"
	// StatusPreview Generating preview currently
	StatusPreview = "preview"
	StatusCut     = "cut"
)

type Job struct {
	Recording   Recording `json:"-" gorm:"foreignKey:ChannelName,Filename;References:ChannelName,Filename;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	JobId       uint      `json:"jobId" gorm:"primaryKey;AUTO_INCREMENT"`
	Channel     Channel   `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:ChannelName"`
	ChannelName string    `json:"channelName" gorm:"not null;default:null"`
	Filename    string    `json:"filename" gorm:"not null;default:null"`
	Filepath    string    `json:"pathRelative" gorm:"not null;default:null"`
	Status      string    `json:"status" gorm:"not null;default:null;index:idx_status"`
	Active      bool      `json:"active" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"createdAt" gorm:"not null;default:null;index:idx_create_at"`
	Info        *string   `json:"info" gorm:"default:null"`
	Args        *string   `json:"args" gorm:"default:null"`
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
		return nil, err
	}

	return &job, nil
}

func GetJobs() ([]*Job, error) {
	var jobs []*Job
	err := Db.Find(&jobs).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return jobs, nil
}

func DeleteJob(jobId uint) error {
	err := Db.Table("jobs").Where("job_id = ?", jobId).Delete(Job{}).Error
	if err != nil {
		return err
	}

	return nil
}

func GetJobsByStatus(status string) ([]*Job, error) {
	var jobs []*Job
	err := Db.Where("status = ?", status).Find(&jobs).Error
	if err != nil {
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

func UpdateJobInfo(jobId uint, info string) error {
	return Db.Model(&Job{}).Where("job_id = ?", jobId).Update("info", info).Error
}

func ActiveJob(jobId uint) error {
	return Db.Model(&Job{}).Where("job_id = ?", jobId).Update("active", true).Error
}

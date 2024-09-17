package database

import (
	"errors"
	"fmt"
	"github.com/astaxie/beego/utils"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"os/exec"
	"path"
	"strings"
	"time"
)

// Channel Represent a single stream, that shall be recorded. It can also serve as a folder for videos.
type Channel struct {
	ChannelId   ChannelId   `json:"channelId" gorm:"autoIncrement;primaryKey;column:channel_id" extensions:"!x-nullable"`
	ChannelName ChannelName `json:"channelName" gorm:"unique;not null;" extensions:"!x-nullable"`
	DisplayName string      `json:"displayName" gorm:"not null;default:''" extensions:"!x-nullable"`
	SkipStart   uint        `json:"skipStart" gorm:"not null;default:0" extensions:"!x-nullable"`
	MinDuration uint        `json:"minDuration" gorm:"not null;default:0" extensions:"!x-nullable"`
	Url         string      `json:"url" gorm:"not null;default:''" extensions:"!x-nullable"`
	Tags        *Tags       `json:"tags" gorm:"type:text;default:null"`
	Fav         bool        `json:"fav" gorm:"index:idx_fav,not null" extensions:"!x-nullable"`
	IsPaused    bool        `json:"isPaused" gorm:"not null,default:false" extensions:"!x-nullable"`
	Deleted     bool        `json:"deleted" gorm:"not null,default:false" extensions:"!x-nullable"`
	CreatedAt   time.Time   `json:"createdAt" gorm:"not null;default:current_timestamp" extensions:"!x-nullable"`

	// Only for query result.
	RecordingsCount uint `json:"recordingsCount" gorm:"<-:false;-:migration" extensions:"!x-nullable"`
	RecordingsSize  uint `json:"recordingsSize" gorm:"<-:false;-:migration" extensions:"!x-nullable"`

	// 1:n
	Recordings []Recording `json:"recordings" gorm:"foreignKey:channel_id;constraint:OnDelete:CASCADE"`
}

func CreateChannel(channelName ChannelName, displayName, url string) (*Channel, error) {
	var channel *Channel
	if err := Db.Model(Channel{}).Where("channel_name = ?", channelName).First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newChannel := newChannel(channelName, displayName, url)
			if err := Db.Create(&newChannel).Error; err != nil {
				return nil, err
			} else {
				return &newChannel, nil
			}
		}
	}

	return channel, nil
}

func DestroyChannelRecordings(channelId ChannelId) error {
	if channelId == 0 {
		return errors.New("invalid channel id")
	}

	channel, err := GetChannelById(channelId)
	if err != nil {
		return err
	}

	// 1. Terminate and delete all jobs.
	if jobs, err := channel.Jobs(); err != nil {
		log.Errorln(err)
	} else {
		for _, job := range jobs {
			if err := DeleteJob(job.JobId); err != nil {
				log.Errorf("Error destroying job: %s", err)
			}
		}
	}

	// 2. Delete records.
	var recordings []*Recording
	if err := Db.Model(&Recording{}).
		Where("channel_id = ?", channelId).
		Find(&recordings).Error; err != nil {
		log.Errorf("No recordings found to destroy for channel %s", channel.ChannelName)
		return err
	}

	for _, recording := range recordings {
		if err := DestroyJobs(recording.RecordingId); err != nil {
			log.Errorf("Error deleting recording %s: %s", recording.Filename, err)
		}
	}

	return nil
}

func CreateChannelDetail(channel Channel) (*Channel, error) {
	if err := Db.Create(&channel).Error; err != nil {
		return nil, err
	}

	if err := channel.ChannelName.MkDir(); err != nil {
		return nil, err
	}
	//channel.WriteJson()

	return &channel, nil
}

func (channel *Channel) ExistsJson() bool {
	return utils.FileExists(channel.jsonPath())
}

func (channel *Channel) FolderExists() bool {
	return utils.FileExists(channel.ChannelName.AbsoluteChannelPath())
}

func (channel *Channel) jsonPath() string {
	return path.Join(channel.ChannelName.AbsoluteChannelPath(), "channel.json")
}

func (channel *Channel) Update() error {
	// Validation
	url := strings.TrimSpace(channel.Url)
	displayName := strings.TrimSpace(channel.DisplayName)

	if len(url) == 0 || len(displayName) == 0 {
		return fmt.Errorf("invalid parameters: %v", channel)
	}

	err := Db.Save(&channel).Error

	return err
}

func (channel *Channel) QueryStreamUrl() (string, error) {
	// We only want to extract the URL, disable all additional text output
	cmd := exec.Command("youtube-dl", "--force-ipv4", "--ignore-errors", "--no-warnings", "--youtube-skip-dash-manifest", "-f best/bestvideo", "--get-url", channel.Url)
	stdout, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(stdout))

	if err != nil {
		return "", err
	}

	return output, nil
}

func ChannelList() ([]*Channel, error) {
	var channels []*Channel

	err := Db.Model(&Channel{}).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_id = channels.channel_id) recordings_count", "(SELECT SUM(size) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_size").
		Find(&channels).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return channels, nil
}

func ChannelListNotDeleted() ([]*Channel, error) {
	var result []*Channel

	err := Db.Model(&Channel{}).
		Where("channels.deleted = ?", false).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_id = channels.channel_id) recordings_count", "(SELECT SUM(size) FROM recordings WHERE recordings.channel_id = channels.channel_id) recordings_size").
		Find(&result).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return result, nil
}

func EnabledChannelList() ([]*Channel, error) {
	var channels []*Channel

	// Query favourites first
	err := Db.Model(&Channel{}).
		Where("deleted = ?", false).
		Where("is_paused = ?", false).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_id = channels.channel_id) recordings_count").
		Order("fav desc").
		Find(&channels).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return channels, nil
}

func newChannel(channelName ChannelName, displayName, url string) Channel {
	return Channel{
		ChannelName: channelName,
		DisplayName: displayName,
		SkipStart:   0,
		MinDuration: 10,
		Url:         strings.TrimSpace(url),
		Tags:        nil,
		Fav:         false,
		IsPaused:    false,
		Deleted:     false,
		CreatedAt:   time.Now(),
	}
}

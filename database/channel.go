package database

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/astaxie/beego/utils"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Channel Represent a single stream, that shall be recorded. It can also serve as a folder for videos.
type Channel struct {
	ChannelID   ChannelID   `json:"channelId" gorm:"autoIncrement;primaryKey;column:channel_id" extensions:"!x-nullable"`
	ChannelName ChannelName `json:"channelName" gorm:"unique;not null;" extensions:"!x-nullable"`
	DisplayName string      `json:"displayName" gorm:"not null;default:''" extensions:"!x-nullable"`
	SkipStart   uint        `json:"skipStart" gorm:"not null;default:0" extensions:"!x-nullable"`
	MinDuration uint        `json:"minDuration" gorm:"not null;default:0" extensions:"!x-nullable"`
	URL         string      `json:"url" gorm:"not null;default:''" extensions:"!x-nullable"`
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
	if err := DB.Model(Channel{}).Where("channel_name = ?", channelName).First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newChannel := newChannel(channelName, displayName, url)
			if err := DB.Create(&newChannel).Error; err != nil {
				return nil, err
			}
			return &newChannel, nil
		}
	}

	return channel, nil
}

func DestroyChannelRecordings(channelID ChannelID) error {
	if channelID == 0 {
		return errors.New("invalid channel id")
	}

	channel, errChannel := GetChannelByID(channelID)
	if errChannel != nil {
		return errChannel
	}

	// 1. Terminate and delete all jobs.
	if jobs, err := channel.Jobs(); err != nil {
		log.Errorln(err)
	} else {
		for _, job := range jobs {
			if err := DeleteJob(job.JobID); err != nil {
				log.Errorf("Error destroying job: %s", err)
			}
		}
	}

	// 2. Delete records.
	var recordings []*Recording
	if err := DB.Model(&Recording{}).
		Where("channel_id = ?", channelID).
		Find(&recordings).Error; err != nil {
		log.Errorf("No recordings found to destroy for channel %s", channel.ChannelName)
		return err
	}

	for _, recording := range recordings {
		if err := recording.DestroyRecording(); err != nil {
			log.Errorf("Error deleting recording %s: %s", recording.Filename, err)
		}
	}

	return nil
}

func CreateChannelDetail(channel Channel) (*Channel, error) {
	if err := DB.Create(&channel).Error; err != nil {
		return nil, err
	}

	if err := channel.ChannelName.MkDir(); err != nil {
		return nil, err
	}
	//channel.WriteJson()

	return &channel, nil
}

func (channel *Channel) ExistsJSON() bool {
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
	url := strings.TrimSpace(channel.URL)
	displayName := strings.TrimSpace(channel.DisplayName)

	if len(url) == 0 || len(displayName) == 0 {
		return fmt.Errorf("invalid parameters: %v", channel)
	}

	err := DB.Save(&channel).Error

	return err
}

func (channel *Channel) QueryStreamURL() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 30-second timeout
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--force-ipv4",
		// "--ignore-errors", // Removed for better error handling
		"--no-warnings",
		"--youtube-skip-dash-manifest",
		"-f", "best", // Or "best/bestvideo" if you have a strong reason for that specific fallback
		"--get-url",
		channel.URL,
	)

	outputBytes, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(outputBytes))

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("yt-dlp command timed out for URL %s", channel.URL)
	}

	if err != nil {
		// err from exec.CommandContext will be non-nil if youtube-dl exits with a non-zero status
		// output will contain stderr from youtube-dl, which is useful context
		return "", fmt.Errorf("yt-dlp failed for URL %s: %v\nOutput: %s", channel.URL, err, output)
	}

	// Basic validation: Does the output look like a URL?
	// This is especially important if you were to re-add --ignore-errors.
	// Even without it, youtube-dl might succeed (exit 0) but return multiple lines or an unexpected string.
	// A more robust check might involve parsing the URL or checking for multiple lines.
	if output == "" || (!strings.HasPrefix(output, "http://") && !strings.HasPrefix(output, "https://") && !strings.HasPrefix(output, "rtmp://")) {
		// Consider if output might contain multiple URLs (one per line)
		// For now, assume a single URL or an error string if it doesn't look like a URL
		lines := strings.Split(output, "\n")
		if len(lines) > 0 && (strings.HasPrefix(lines[0], "http://") || strings.HasPrefix(lines[0], "https://") || strings.HasPrefix(lines[0], "rtmp://")) {
			// If the first line looks like a URL, use it (e.g. some extractors print metadata then the URL)
			return lines[0], nil
		}
		return "", fmt.Errorf("yt-dlp returned empty or invalid output for URL %s: %s", channel.URL, output)
	}

	// If output contains multiple URLs (e.g. from a playlist if -g is used without --no-playlist),
	// this will return all of them, separated by newlines.
	// Your application needs to handle this (e.g., pick the first one).
	// For a single video, it should be one URL.
	// If you expect only one URL, you might want to split by newline and take lines[0].
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		return lines[0], nil // Return the first URL if multiple are given
	}

	// This part should ideally not be reached if the previous checks are robust.
	return "", fmt.Errorf("yt-dlp returned unexpected data for URL %s: %s", channel.URL, output)
}

func ChannelList() ([]*Channel, error) {
	var channels []*Channel

	err := DB.Model(&Channel{}).
		Select("channels.*", "(SELECT COUNT(*) FROM recordings WHERE recordings.channel_id = channels.channel_id) recordings_count", "(SELECT SUM(size) FROM recordings WHERE recordings.channel_name = channels.channel_name) recordings_size").
		Find(&channels).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	return channels, nil
}

func ChannelListNotDeleted() ([]*Channel, error) {
	var result []*Channel

	err := DB.Model(&Channel{}).
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
	err := DB.Model(&Channel{}).
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
		URL:         strings.TrimSpace(url),
		Tags:        nil,
		Fav:         false,
		IsPaused:    false,
		Deleted:     false,
		CreatedAt:   time.Now(),
	}
}

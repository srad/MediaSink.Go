package database

import (
    "errors"
    log "github.com/sirupsen/logrus"
    "gorm.io/gorm"
    "os"
    "path/filepath"
)

type ChannelId uint

func (channelId ChannelId) TagChannel(tags *Tags) error {
    return Db.Table("channels").
        Where("channel_id = ?", channelId).
        Update("tags", tags).Error
}

func (channelId ChannelId) GetChannelById() (*Channel, error) {
    var channel Channel

    err := Db.Model(&Channel{}).
        Where("channels.channel_id = ?", channelId).
        Select("*").
        Preload("Recordings").
        Find(&channel).Error

    if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, err
    }

    return &channel, nil
}

func (channelId ChannelId) FavChannel() error {
    return Db.Table("channels").
        Where("channel_id = ?", channelId).
        Update("fav", true).Error
}

func (channelId ChannelId) UnFavChannel() error {
    return Db.Table("channels").
        Where("channel_id = ?", channelId).
        Update("fav", false).Error
}

// SoftDestroyChannel Delete all recordings and mark channel to delete.
// Often the folder is locked for multiple reasons and can only be deleted on restart.
func (channelId ChannelId) SoftDestroyChannel() error {
    channel, err := channelId.GetChannelById()
    if err != nil {
        return err
    }

    if err := channelId.DestroyAllRecordings(); err != nil {
        log.Errorf("Error deleting recordings of channel '%s': %s", channel.ChannelName, err)
        return err
    }
    if err := os.RemoveAll(channel.ChannelName.AbsoluteChannelPath()); err != nil && !os.IsNotExist(err) {
        log.Errorf("Error deleting channel folder: %s", err)
        return err
    }

    if err := Db.Model(&Channel{}).
        Where("channel_id = ?", channel.ChannelId).
        Update("deleted", true).Error; err != nil {
        log.Errorf("[SoftDestroy] Error updating channels table: %s", err)
        return err
    }

    return nil
}

func (channelId ChannelId) DestroyChannel() error {
    channel, err := channelId.GetChannelById()
    if err != nil {
        return err
    }

    // Channel folder
    if err := os.RemoveAll(channel.ChannelName.AbsoluteChannelPath()); err != nil && !os.IsNotExist(err) {
        log.Infof("Error deleting channel folder: %s", err)
        return err
    }
    if err := Db.Where("channel_id = ?", channel.ChannelId).Delete(Channel{}).Error; err != nil {
        return err
    }
    return nil
}

func (channelId ChannelId) DestroyAllRecordings() error {
    channel, err := channelId.GetChannelById()
    if err != nil {
        return err
    }

    // Terminate and delete all jobs
    if jobs, err := channel.Jobs(); err != nil {
        log.Errorln(err)
    } else {
        for _, job := range jobs {
            if err := DeleteJob(job.JobId); err != nil {
                log.Errorf("Error destroying job: %s", err)
            }
        }
    }

    var recordings []*Recording
    if err := Db.Where("channel_id = ?", channel.ChannelId).Find(&recordings).Error; err != nil {
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

func (channelId ChannelId) PauseChannel(pauseVal bool) error {
    if err := Db.Table("channels").
        Where("channel_id = ?", channelId).
        Update("is_paused", pauseVal).Error; err != nil {
        return err
    }

    return nil
}

func (channelId ChannelId) NewRecording(videoType string) (*Recording, string, error) {
    channel, err := channelId.GetChannelById()
    if err != nil {
        return nil, "", err
    }

    filename, timestamp := channel.ChannelName.MakeRecordingFilename()
    relativePath := filepath.Join(channel.ChannelName.String(), filename.String())
    filePath := channel.ChannelName.AbsoluteChannelFilePath(filename)

    return &Recording{
            ChannelId:     channel.ChannelId,
            ChannelName:   channel.ChannelName,
            Filename:      filename,
            Bookmark:      false,
            CreatedAt:     timestamp,
            VideoType:     videoType,
            Packets:       0,
            Duration:      0,
            Size:          0,
            BitRate:       0,
            Width:         0,
            Height:        0,
            PathRelative:  relativePath,
            PreviewStripe: nil,
            PreviewVideo:  nil,
            PreviewCover:  nil,
        },
        filePath,
        nil
}


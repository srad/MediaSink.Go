package models

import (
	"fmt"
	"github.com/srad/streamsink/utils"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/srad/streamsink/conf"
	"gorm.io/gorm"
)

var (
	UpdatingVideoInfo bool = false
)

type Recording struct {
	Channel     Channel   `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:channel_name;references:channel_name"`
	ChannelName string    `json:"channelName" gorm:"primaryKey;"`
	Filename    string    `json:"filename" gorm:"primaryKey;"`
	Bookmark    bool      `json:"bookmark" gorm:"index:idx_bookmark,not null"`
	CreatedAt   time.Time `json:"createdAt" gorm:"not null"`
	LastAccess  time.Time `json:"lastAccess"`
	VideoType   string    `json:"videoType"`

	Packets  uint64  `json:"packets" gorm:"default:0;not null"`
	Duration float64 `json:"duration" gorm:"default:0;not null"`
	Size     uint64  `json:"size" gorm:"default:0;not null"`
	BitRate  uint64  `json:"bitRate" gorm:"default:0;not null"`
	Width    uint    `json:"width" gorm:"default:0"`
	Height   uint    `json:"height" gorm:"default:0"`

	PathRelative  string `json:"pathRelative" gorm:"not null;"`
	PreviewStripe string `json:"previewStripe" gorm:"default:null"`
	PreviewVideo  string `json:"previewVideo" gorm:"default:null"`
	PreviewCover  string `json:"previewCover" gorm:"default:null"`
}

func FindByName(channelName string) ([]*Recording, error) {
	var recordings []*Recording
	err := Db.Model(Recording{}).
		Select("recordings.*").
		Where("recordings.channel_name = ?", channelName).
		Order("recordings.created_at DESC").
		Find(&recordings).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return recordings, nil
}

func FavRecording(channelName, filename string, fav bool) error {
	return Db.Model(Recording{}).
		Where("channel_name = ? AND filename = ?", channelName, filename).
		Update("bookmark", fav).Error
}

func ExitsRecord(channelName, filename string) (bool, error) {
	var exists bool
	if err := Db.Model(Recording{}).
		Select("count(*) > 0").
		Where("channel_name = ? AND filename = ?", channelName, filename).
		Find(&exists).
		Error; err != nil {
		return false, err
	}

	return exists, nil
}

func SortBy(column, order string, limit int) ([]*Recording, error) {
	var recordings []*Recording

	err := Db.Model(Recording{}).
		Order(fmt.Sprintf("recordings.%s %s", column, order)).
		Limit(limit).
		Find(&recordings).Error

	if err != nil {
		return nil, err
	}

	return recordings, nil
}

func FindRandom(limit int) ([]*Recording, error) {
	var recordings []*Recording

	err := Db.Model(Recording{}).
		Order("RANDOM()").
		Limit(limit).
		Find(&recordings).Error

	if err != nil {
		return nil, err
	}

	return recordings, nil
}

func RecordingsList() ([]*Recording, error) {
	var recordings []*Recording

	err := Db.Model(Recording{}).
		Select("recordings.*").
		Find(&recordings).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return recordings, nil
}

func BookmarkList() ([]*Recording, error) {
	var recordings []*Recording
	err := Db.Model(Recording{}).
		Where("bookmark = ?", true).
		Select("recordings.*").Order("recordings.channel_name asc").
		Find(&recordings).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return recordings, nil
}

func (recording *Recording) Save(videoType string) error {
	info, err := utils.GetVideoInfo(conf.AbsoluteFilepath(recording.ChannelName, recording.Filename))
	if err != nil {
		log.Printf("[AddRecord] GetVideoInfo() error '%s' for '%s'", err.Error(), conf.AbsoluteFilepath(recording.ChannelName, recording.Filename))
		return err
	}

	recording.Duration = info.Duration
	recording.Size = info.Size
	recording.BitRate = info.BitRate
	recording.Width = info.Width
	recording.Height = info.Height
	recording.VideoType = videoType
	recording.Packets = info.PacketCount

	log.Printf("[AddRecord] Creating %v", recording)
	if err := Db.Create(&recording).Error; err != nil {
		log.Printf("[AddRecord] Error creating record: %v\n", err)
		return err
	}

	//EnqueuePreviewJob(recording.ChannelName, recording.Filename)

	return nil
}

func (recording *Recording) Destroy() error {
	if err := Db.Delete(&Recording{}, "channel_name = ? AND filename = ?", recording.ChannelName, recording.Filename).Error; err != nil {
		log.Println(fmt.Sprintf("Error deleting recordings of file '%s' from channel '%s': %v", recording.Filename, recording.ChannelName, err))
		return err
	}

	// TODO: Also Cancel running jobs from this channel
	// Remove associated jobs
	if err := Db.Delete(&Job{}, "channel_name = ? AND filename = ?", recording.ChannelName, recording.Filename).Error; err != nil && err != gorm.ErrRecordNotFound {
		log.Println(fmt.Sprintf("Error job for recording of file '%s' from channel '%s': %v", recording.Filename, recording.ChannelName, err))
		return err
	}

	paths := conf.GetRecordingsPaths(recording.ChannelName, recording.Filename)

	if err := os.Remove(paths.Filepath); err != nil && !os.IsNotExist(err) {
		log.Println(fmt.Sprintf("Error deleting recording: %v", err))
	}

	return nil
}

func (channel *Channel) Filename() string {
	return recInfo[channel.ChannelName].Filename
}

func (channel *Channel) DeleteRecordingsFile(filename string) error {
	paths := conf.GetRecordingsPaths(channel.ChannelName, filename)
	log.Printf("[Info] Deleting file")

	if err := os.Remove(paths.Filepath); err != nil && !os.IsNotExist(err) {
		log.Println(fmt.Sprintf("Error deleting recording: %v", err))
		return err
	}

	return nil
}

func FindRecording(channelName, filename string) (*Recording, error) {
	var recording *Recording
	err := Db.Table("recordings").
		Where("channel_name = ? AND filename = ?", channelName, filename).
		First(&recording).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return recording, nil
}

func (recording *Recording) FindJobs() (*[]Job, error) {
	var jobs *[]Job
	err := Db.Model(&Job{}).
		Where("channel_name = ? AND filename = ?", recording.ChannelName, recording.Filename).
		Find(&jobs).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	return jobs, nil
}

func (recording *Recording) DestroyJobs() error {
	return Db.Delete(&Job{}, "channel_name = ? AND filename = ?", recording.ChannelName, recording.Filename).Error
}

func AddIfNotExistsRecording(channelName, filename string) (*Recording, error) {
	rec := Db.First(&Recording{ChannelName: channelName, Filename: filename})
	if rec.Error != nil && rec.Error != gorm.ErrRecordNotFound {
		log.Printf("[AddIfNotExistsRecording] Error %v", rec.Error)
		return nil, rec.Error
	}

	if rec.RowsAffected > 0 {
		log.Printf("[AddIfNotExistsRecording] Info '%s' already in database", filename)

		return nil, nil
	}

	newRec := &Recording{
		ChannelName:  channelName,
		Filename:     filename,
		PathRelative: conf.GetRelativeRecordingsPath(channelName, filename),
		CreatedAt:    time.Now(),
		Bookmark:     false,
	}

	if err := newRec.Save("recording"); err != nil {
		log.Printf("[AddIfNotExistsRecording] Error creating: %v", rec.Error)
		return nil, err
	}

	return newRec, nil
}

func (recording *Recording) GetVideoInfo() (*utils.FFProbeInfo, error) {
	return utils.GetVideoInfo(conf.AbsoluteFilepath(recording.ChannelName, recording.Filename))
}

func (recording *Recording) UpdateInfo(info *utils.FFProbeInfo) error {
	return Db.Updates(&Recording{ChannelName: recording.ChannelName, Filename: recording.Filename, Duration: info.Duration, BitRate: info.BitRate, Size: info.Size, Width: info.Width, Height: info.Height, Packets: info.PacketCount}).Error
}

func (recording *Recording) FilePath() string {
	return conf.AbsoluteFilepath(recording.ChannelName, recording.Filename)
}

func (recording *Recording) DataFolder() string {
	return conf.AbsoluteDataPath(recording.ChannelName)
}

func UpdatePreview(channelName, filename string) (*Recording, error) {
	rec, err := FindRecording(channelName, filename)
	if err != nil {
		return nil, err
	}

	paths := conf.GetRecordingsPaths(channelName, filename)

	rec.PreviewVideo = filepath.Join("recordings", channelName, conf.AppCfg.DataPath, "videos", paths.MP4)
	rec.PreviewStripe = filepath.Join("recordings", channelName, conf.AppCfg.DataPath, "stripes", paths.JPG)

	if err := Db.
		Table("recordings").
		Where("channel_name = ? AND filename = ?", channelName, filename).
		Save(&rec).Error; err != nil {
		return nil, err
	}

	return rec, nil
}

func DestroyPreviews(channelName, filename string) error {
	paths := conf.GetRecordingsPaths(channelName, filename)

	if err := os.Remove(paths.VideosPath); err != nil && !os.IsNotExist(err) {
		log.Println(fmt.Sprintf("Error deleting '%s' from channel '%s': %v", paths.VideosPath, channelName, err))
	}
	if err := os.Remove(paths.StripePath); err != nil && !os.IsNotExist(err) {
		log.Println(fmt.Sprintf("Error deleting '%s' from channel '%s': %v", paths.StripePath, channelName, err))
	}
	if err := os.Remove(paths.CoverPath); err != nil && !os.IsNotExist(err) {
		log.Println(fmt.Sprintf("Error deleting '%s' from channel '%s': %v", paths.StripePath, channelName, err))
	}

	rec, err := FindRecording(channelName, filename)
	if err != nil {
		return err
	}

	rec.PreviewVideo = ""
	rec.PreviewStripe = ""
	rec.PreviewCover = ""

	if err := Db.Model(&rec).Save(&rec).Error; err != nil {
		return err
	}

	return nil
}

func UpdateVideoInfo() error {
	log.Println("[Recorder] Updating all recordings info")
	recordings, err := RecordingsList()
	if err != nil {
		log.Printf("Error %v", err)
		return err
	}
	UpdatingVideoInfo = true
	count := len(recordings)

	i := 1
	for _, rec := range recordings {
		info, err := rec.GetVideoInfo()
		if err != nil {
			log.Printf("[UpdateVideoInfo] Error updating video info: %v", err)
			continue
		}

		if err := rec.UpdateInfo(info); err != nil {
			log.Printf("[Recorder] Error updating video info: %v", err.Error())
			continue
		}
		log.Printf("[Recorder] Updated %s (%d/%d)", rec.Filename, i, count)
		i++
	}

	UpdatingVideoInfo = false

	return nil
}

package models

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/srad/streamsink/conf"
)

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

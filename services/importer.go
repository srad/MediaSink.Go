package services

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/models"
	"os"
	"path/filepath"
)

var (
	ctx, cancelImport = context.WithCancel(context.Background())
	importing         = false
)

func StartImport() {
	go runImport()
}

func StopImport() {
	cancelImport()
}

func IsImporting() bool {
	return importing
}

func runImport() {
	importing = true

	if err := ImportChannels(ctx); err != nil {
		log.Errorln(err)
	}

	importing = false
}

// ImportChannels Imports folders and videos found on disk.
//
// 1. Import all folders as channels found in the recording path.
// 2. If the folder contains the channel.json backup file, then reconstruct the channel information from this file.
// 3. Then search on each folder for media files to import as recordings.
// 4. If the recordings do not contain previews, their creation will be scheduled.
func ImportChannels(context.Context) error {
	cfg := conf.Read()

	log.Infoln("------------------------------------------------------------------------------------------")
	log.Infof("Scanning file system for media: %s", cfg.RecordingsAbsolutePath)
	log.Infoln("------------------------------------------------------------------------------------------")

	recordingFolder, err := os.Open(cfg.RecordingsAbsolutePath)
	if err != nil {
		return fmt.Errorf("failed opening directory '%s': %s", cfg.RecordingsAbsolutePath, err)
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			log.Errorf("error closing folder %s", file.Name())
		}
	}(recordingFolder)

	// ---------------------------------------------------------------------------------
	// Traverse folders (channels)
	// ---------------------------------------------------------------------------------
	channelFolders, _ := recordingFolder.Readdirnames(0)
	var i = 0
	for _, name := range channelFolders {
		i++
		channelName := models.ChannelName(name)
		log.Infof("Import/%s (%d/%d)] Scanning folder", channelName, i, len(channelFolders))
		// Is no directory, skip
		if dir, err := os.Stat(channelName.AbsoluteChannelPath()); err != nil || !dir.IsDir() {
			continue
		}

		log.Infof("[Import/%s (%d/%d)] Reading folder", channelName, i, len(channelFolders))

		channel := models.NewChannel(channelName, channelName.String(), "https://"+channelName.String())

		// Import from JSON file, if found.
		//if channel.ExistsJson() {
		//	if json, err2 := channel.ReadJson(); err2 == nil {
		//		log.Infof("[Import/%s] json: %v", channelName, json)
		//		channel.ChannelId = json.ChannelId
		//		channel.ChannelName = json.ChannelName
		//		channel.DisplayName = json.DisplayName
		//		channel.SkipStart = json.SkipStart
		//		channel.Url = json.Url
		//		channel.Tags = json.Tags
		//		channel.Fav = json.Fav
		//	}
		//}

		if err4 := channel.Create(); err4 != nil {
			log.Errorf("[Import/%s (%d/%d)] Error adding  channel: %s", channelName, err4, i, len(channelFolders))
		}

		// ---------------------------------------------------------------------------------
		// Import individual files
		// ---------------------------------------------------------------------------------
		files, err2 := os.ReadDir(channelName.AbsoluteChannelPath())
		if err2 != nil {
			log.Errorf("[Import/%s] Error reading: %s", channelName, err2)
			continue
		}

		// ---------------------------------------------------------------------------------
		// Traverse all mp4 files and add to models if not existent
		// ---------------------------------------------------------------------------------
		var j = 0
		for _, file := range files {
			j++
			mp4File := !file.IsDir() && filepath.Ext(file.Name()) == ".mp4"
			if !mp4File {
				continue
			}

			recording := models.Recording{ChannelId: channel.ChannelId, ChannelName: channelName, Filename: file.Name()}

			log.Infof("[Import/%s (%d/%d) (%d/%d)] Checking file: %s", channelName, i, len(channelFolders), j, len(files), file.Name())

			video := &helpers.Video{FilePath: channelName.AbsoluteChannelFilePath(file.Name())}

			if _, errVideoInfo := video.GetVideoInfo(); errVideoInfo != nil {
				log.Errorf("[Import/%s] File '%s' seems corrupted, deleting: %s", channelName, file.Name(), errVideoInfo)
				if errDestroy := recording.Destroy(); errDestroy != nil {
					log.Errorf("[Import/%s] Error deleting: %s: %s", channelName, file.Name(), errDestroy)
				} else {
					log.Infof("[Import/%s] Deleted: %s", channelName, file.Name())
				}
				continue
			}

			// File seems ok, try to add.
			if errAdd := recording.AddIfNotExists(); errAdd != nil {
				log.Errorf("[Import/%s] Error: %s", channelName, errAdd)
				continue
			}

			// ---------------------------------------------------------------------------------
			// Not new record inserted and therefore not automatically new previews generated.
			// So check if the files exist and if not generate them.
			// Create preview if any not existent
			// ---------------------------------------------------------------------------------
			if recording.PreviewsExist() {
				log.Infof("[Import/%s] Preview files exist", channelName)
				recording.AddPreviews()
			} else {
				log.Infof("[Import/%s] Adding job for: %s", channelName, file.Name())
				recording.EnqueuePreviewJob()
			}
		}
	}

	return nil
}

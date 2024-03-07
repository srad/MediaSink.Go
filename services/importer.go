package services

import (
	"context"
	"fmt"
	"github.com/astaxie/beego/utils"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/helpers"
	"log"
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
	ImportChannels(ctx)
	importing = false
}

// ImportChannels Imports folders and videos found on disk.
//
// 1. Import all folders as channels found in the recording path.
// 2. If the folder contains the channel.json backup file, then reconstruct the channel information from this file.
// 3. Then search on each folder for media files to import as recordings.
// 4. If the recordings do not contain previews, their creation will be scheduled.
func ImportChannels(context.Context) error {
	log.Println("################################## ImportRecordings ##################################")
	log.Printf("[Import] Importing files from recordingFolder system: %s", conf.AppCfg.RecordingsAbsolutePath)

	recordingFolder, err := os.Open(conf.AppCfg.RecordingsAbsolutePath)
	if err != nil {
		log.Printf("->[Import] Failed opening directory '%s': %v\n", conf.AppCfg.RecordingsAbsolutePath, err)
		return err
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			log.Printf("Error closing folder %s", file.Name())
		}
	}(recordingFolder)

	// Traverse folders (channels)
	channelFolders, _ := recordingFolder.Readdirnames(0)
	for _, channelName := range channelFolders {
		// Is no directory, skip
		if dir, err := os.Stat(conf.AbsoluteChannelPath(channelName)); err != nil || !dir.IsDir() {
			continue
		}

		log.Printf("[Import] Reading folder: %s\n", channelName)

		channel := &database.Channel{
			ChannelName: channelName,
			DisplayName: channelName,
			SkipStart:   0,
			Url:         "https://" + channelName,
			Tags:        "",
			Fav:         false,
			IsPaused:    false,
			Deleted:     false,
		}

		// Import from JSON file, if found.
		if channel.ExistsJson() {
			if json, err2 := channel.ReadJson(); err2 == nil {
				fmt.Printf("json: %v", json)
				channel.ChannelName = json.ChannelName
				channel.DisplayName = json.DisplayName
				channel.SkipStart = json.SkipStart
				channel.Url = json.Url
				channel.Tags = json.Tags
				channel.Fav = json.Fav
			}
		}

		if _, err := channel.Create(nil); err != nil {
			log.Printf(" + Error adding channel channel '%s': %v", channelName, err)
		}

		// Import individual files
		files, err := os.ReadDir(conf.AbsoluteChannelPath(channelName))
		if err != nil {
			log.Printf("[Import] Error reading '%s': %v", channelName, err)
			continue
		}
		// Traverse all mp4 files and add to database if not existent
		for _, file := range files {
			mp4File := !file.IsDir() && filepath.Ext(file.Name()) == ".mp4"
			if !mp4File {
				continue
			}

			log.Printf("  [Import] Checking recordingFolder: %s, %s", channelName, file.Name())

			if _, err := helpers.GetVideoInfo(conf.AbsoluteChannelFilePath(channelName, file.Name())); err != nil {
				log.Printf("    [Import] File '%s' seems corrupted, deleting ...", file.Name())
				if err := channel.DeleteRecordingsFile(file.Name()); err != nil {
					log.Printf("    [Import] Error deleting '%s'", file.Name())
				} else {
					database.DestroyPreviews(channelName, file.Name())
					log.Printf("    [Import] Deleted: %s", file.Name())
				}
				continue
			}
			if _, err := database.AddIfNotExistsRecording(channelName, file.Name()); err != nil {
				log.Printf("    [Import] Error: %s\n", err.Error())
				continue
			}

			// Not new record inserted and therefore not automatically new previews generated.
			// So check if the files exist and if not generate them.
			// Create preview if any not existent
			paths := conf.GetRecordingsPaths(channelName, file.Name())
			videoExists := utils.FileExists(paths.AbsoluteVideosPath)
			stripeExists := utils.FileExists(paths.AbsoluteStripePath)
			posterExists := utils.FileExists(paths.AbsolutePosterPath)
			screensExist := utils.FileExists(paths.ScreensPath)

			if videoExists && stripeExists && posterExists && screensExist {
				log.Println("    [Import] Preview files exist")
				database.UpdatePreview(channelName, file.Name())
			} else {
				log.Printf("    [Import] Adding job for %s\n", file.Name())
				database.EnqueuePreviewJob(channelName, file.Name())
			}
		}
	}

	return nil
}

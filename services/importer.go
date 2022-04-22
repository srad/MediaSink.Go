package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/models"
	"github.com/srad/streamsink/utils"
	"log"
	"os"
	"path/filepath"
)

func StartImport() {
	ctx, c := context.WithCancel(context.Background())
	cancelImport = c
	go runImport(ctx)
}

func StopImport() {
	cancelImport()
}

func IsImporting() bool {
	return importing
}

func runImport(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			importing = false
			log.Println("[runImport] Stopped import")
			return
		default:
			importing = true
			importRecordings()
			importing = false
			cancelImport()
		}
	}
}

func importRecordings() error {
	log.Println("################################## ImportRecordings ##################################")
	log.Printf("[Import] Importing files from file system: %s", conf.AppCfg.RecordingsAbsolutePath)

	file, err := os.Open(conf.AppCfg.RecordingsAbsolutePath)
	if err != nil {
		log.Printf("->[Import] Failed opening directory '%s': %v\n", conf.AppCfg.RecordingsAbsolutePath, err)
		return err
	}
	defer file.Close()

	channelFolders, _ := file.Readdirnames(0)
	for _, channelName := range channelFolders {
		if dir, err := os.Stat(conf.AbsoluteRecordingsPath(channelName)); err != nil || !dir.IsDir() {
			continue
		}

		log.Printf("[Import] Reading folder: %s\n", channelName)

		channel := &models.Channel{
			ChannelName: channelName,
			DisplayName: channelName,
			SkipStart:   0,
			Url:         fmt.Sprintf(conf.AppCfg.DefaultImportUrl, channelName),
			Tags:        "",
			Fav:         false,
			IsPaused:    false,
			Deleted:     false,
		}

		if _, err := channel.Create(nil); err != nil {
			log.Printf(" + Error adding channel channel '%s': %v", channelName, err)
		}

		files, err := os.ReadDir(conf.AbsoluteRecordingsPath(channelName))
		if err != nil {
			log.Printf("[Import] Error reading '%s': %v", channelName, err)
			continue
		}
		// Traverse all mp4 files and add to database if not existent
		for _, file := range files {
			isMp4File := !file.IsDir() && filepath.Ext(file.Name()) == ".mp4"

			if isMp4File {
				log.Printf(" + [Import] Checking file: %s, %s", channelName, file.Name())

				if _, err := utils.GetVideoInfo(conf.GetAbsoluteRecordingsPath(channelName, file.Name())); err != nil {
					log.Printf(" + [Import] File '%s' seems corrupted, deleting ...", file.Name())
					if err := channel.DeleteRecordingsFile(file.Name()); err != nil {
						log.Printf(" + [Import] Error deleting '%s'", file.Name())
					} else {
						models.DestroyPreviews(channelName, file.Name())
						log.Printf(" + [Import] Deleted file '%s'", file.Name())
					}
					continue
				}
				if _, err := models.AddIfNotExistsRecording(channelName, file.Name()); err != nil {
					log.Printf(" + [Import] Error: %s\n", err.Error())
					continue
				}

				// Not new record inserted and therefore not automatically new previews generated.
				// So check if the files exist and if not generate them.
				// Create preview if any not existent
				paths := conf.GetRecordingsPaths(channelName, file.Name())
				_, err1 := os.Stat(paths.AbsoluteVideosPath)
				_, err2 := os.Stat(paths.AbsoluteStripePath)
				_, err3 := os.Stat(paths.AbsolutePosterPath)

				if err1 == nil && err2 == nil && err3 == nil {
					log.Println(" + [Import] Preview files exist")
					models.UpdatePreview(channelName, file.Name())
					continue
				} else if errors.Is(err1, os.ErrNotExist) || errors.Is(err2, os.ErrNotExist) {
					log.Printf(" + [Import] Adding job for %s\n", file.Name())
					models.EnqueuePreviewJob(channelName, file.Name())
				} else {
					// Schrodinger: file may or may not exist. See err for details.
					// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
					log.Printf(" + [Import] Error: %v, %v", err1, err2)
				}
			}
		}
	}

	log.Println("######################################################################################")

	return nil
}

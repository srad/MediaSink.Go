package models

import (
	"database/sql/driver"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/helpers"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	validChannelName, _ = regexp.Compile("(?i)^[a-z_0-9]+$")
	SnapshotFilename    = "live.jpg"
)

type ChannelName string

type RecordingPaths struct {
	AbsoluteStripePath     string
	AbsoluteRecordingsPath string
	AbsoluteVideosPath     string
	AbsolutePosterPath     string
	Filepath               string
	VideosPath             string
	StripePath             string
	CoverPath              string
	JPG                    string
	MP4                    string
	//ScreensPath            string
}

// Scan Restores the channel type from the database
func (c *ChannelName) Scan(src any) error {
	channelNameString, ok := src.(string)
	if !ok {
		return errors.New("src value cannot cast to string")
	}
	*c = ChannelName(channelNameString)
	return nil
}

// Value Stores the channel name in the database.
func (c *ChannelName) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}

	if err := c.IsValid(); err != nil {
		return nil, err
	}

	channelName := c.normalize()

	if !validChannelName.MatchString(channelName) {
		return nil, fmt.Errorf("invalid channel name %s", channelName)
	}

	return channelName, nil
}

func (c *ChannelName) IsValid() error {
	if c == nil {
		return errors.New("channel name is nil")
	}

	str := c.normalize()
	if !validChannelName.MatchString(str) {
		return fmt.Errorf("invalid normalized channel name %s", str)
	}

	return nil
}

func (c *ChannelName) normalize() string {
	if c != nil {
		return strings.ToLower(strings.TrimSpace(string(*c)))
	}
	return ""
}

func (c *ChannelName) String() string {
	return string(*c)
}

func (c *ChannelName) AbsoluteChannelDataPath() string {
	cfg := conf.Read()
	return filepath.Join(cfg.RecordingsAbsolutePath, c.String(), cfg.DataPath)
}

func (c *ChannelName) AbsoluteChannelPath() string {
	cfg := conf.Read()
	return filepath.Join(cfg.RecordingsAbsolutePath, c.String())
}

func (c *ChannelName) MkDir() error {
	dir := c.AbsoluteChannelPath()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Infoln("Creating folder: " + dir)
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("error creating folder: '%s': %s", dir, err)
		}
	}
	dataPath := c.AbsoluteChannelDataPath()
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		log.Infoln("Creating folder: " + dataPath)
		if err := os.MkdirAll(dataPath, os.ModePerm); err != nil {
			return fmt.Errorf("error creating data path '%s': %s", dataPath, err)
		}
		if err := copyDefaultSnapshotTo(dataPath); err != nil {
			log.Errorln(err)
		}
	}

	return nil
}

func copyDefaultSnapshotTo(dataPath string) error {
	pwd, err := os.Getwd()
	if err != nil {
		log.Errorln(err)
	}

	filePath := filepath.Join(pwd, "assets", "live.jpg")
	srcFile, err := os.Open(filePath)
	check(err)
	defer func(srcFile *os.File) {
		if err := srcFile.Close(); err != nil {
			log.Errorf("Error copying default live.jpg image to folder '%s': %s", filePath, err)
		}
	}(srcFile)

	destFile, err := os.Create(filepath.Join(dataPath, SnapshotFilename)) // creates if file doesn't exist
	check(err)
	defer func(destFile *os.File) {
		if err := destFile.Close(); err != nil {
			log.Errorf("Error creating snapshot file: %s", err)
		}
	}(destFile)

	_, err = io.Copy(destFile, srcFile) // check first var for number of bytes copied
	check(err)

	return destFile.Sync()
}

func check(err error) {
	if err != nil {
		log.Errorf("Error : %s", err)
		os.Exit(1)
	}
}

func (c *ChannelName) GetRecordingsPaths(filename string) RecordingPaths {
	posterJpg := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".jpg"
	stripeJpg := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".jpg"
	mp4 := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".mp4"

	cfg := conf.Read()

	return RecordingPaths{
		AbsoluteRecordingsPath: cfg.RecordingsAbsolutePath,

		Filepath:   c.AbsoluteChannelFilePath(filename),
		VideosPath: filepath.Join(c.RelativeDataPath(), helpers.VideosFolder, mp4),
		StripePath: filepath.Join(c.RelativeDataPath(), helpers.StripesFolder, stripeJpg),
		CoverPath:  filepath.Join(c.RelativeDataPath(), helpers.PostersFolder, posterJpg),
		//ScreensPath:        filepath.Join(c.RelativeDataPath(), ScreensFolder, filename),
		AbsoluteVideosPath: filepath.Join(c.AbsoluteChannelDataPath(), helpers.VideosFolder, mp4),
		AbsoluteStripePath: filepath.Join(c.AbsoluteChannelDataPath(), helpers.StripesFolder, stripeJpg),
		AbsolutePosterPath: filepath.Join(c.AbsoluteChannelDataPath(), helpers.PostersFolder, posterJpg),
		JPG:                stripeJpg,
		MP4:                mp4,
	}
}

func (c *ChannelName) RelativeDataPath() string {
	cfg := conf.Read()
	return filepath.Join(c.String(), cfg.DataPath)
}

func (c *ChannelName) ChannelPath(filename string) string {
	return filepath.Join(c.String(), filename)
}

func (c *ChannelName) AbsoluteChannelFilePath(filename string) string {
	cfg := conf.Read()
	return filepath.Join(cfg.RecordingsAbsolutePath, c.String(), filename)
}

func (c *ChannelName) MakeRecordingFilename() (string, time.Time) {
	now := time.Now()
	stamp := now.Format("2006_01_02_15_04_05")
	return fmt.Sprintf("%s_%s.mp4", c.String(), stamp), now
}

func (c *ChannelName) MakeMp3Filename() (string, time.Time) {
	now := time.Now()
	stamp := now.Format("2006_01_02_15_04_05")
	return fmt.Sprintf("%s_%s.mp3", c.String(), stamp), now
}

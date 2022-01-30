package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Cfg struct {
	DbFileName             string
	RecordingsFolder       string
	RecordingsAbsolutePath string
	DataDisk               string
	NetworkDev             string
	DataPath               string
	PublicPath             string
	ScriptPath             string
	Default                struct {
		ImportUrl string
	}
}

var AppCfg = &Cfg{}

type VideoPaths struct {
	Filepath string
}

func DataPath(channelName string) string {
	return filepath.Join(AppCfg.RecordingsFolder, channelName, AppCfg.DataPath)
}

func AbsoluteDataPath(channelName string) string {
	return filepath.Join(AppCfg.RecordingsAbsolutePath, channelName, AppCfg.DataPath)
}

func AbsoluteRecordingsPath(channelName string) string {
	return filepath.Join(AppCfg.RecordingsAbsolutePath, channelName)
}

func AbsoluteFilepath(channelName, filename string) string {
	return filepath.Join(AppCfg.RecordingsAbsolutePath, channelName, filename)
}

type RecordingPaths struct {
	AbsoluteStripePath     string
	AbsoluteRecordingsPath string
	AbsoluteVideosPath     string
	Filepath               string
	VideosPath             string
	StripePath             string
	JPG                    string
	MP4                    string
}

func GetRecordingsPaths(channelName, filename string) RecordingPaths {
	jpg := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".jpg"
	mp4 := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".mp4"

	return RecordingPaths{
		AbsoluteRecordingsPath: AppCfg.RecordingsAbsolutePath,

		Filepath:           AbsoluteFilepath(channelName, filename),
		VideosPath:         filepath.Join(DataPath(channelName), "videos", mp4),
		StripePath:         filepath.Join(DataPath(channelName), "stripes", jpg),
		AbsoluteVideosPath: filepath.Join(AbsoluteDataPath(channelName), "videos", mp4),
		AbsoluteStripePath: filepath.Join(AbsoluteDataPath(channelName), "stripes", jpg),
		JPG:                jpg,
		MP4:                mp4,
	}
}

func GetRelativeRecordingsPath(channelName, filename string) string {
	return filepath.Join(AppCfg.RecordingsFolder, channelName, filename)
}

func GetAbsoluteRecordingsPath(channelName, filename string) string {
	return filepath.Join(AppCfg.RecordingsAbsolutePath, channelName, filename)
}

func Read() {
	viper.SetConfigName("conf/app") // name of config file (without extension)
	viper.AddConfigPath("./")       // path to look for the config file in
	err := viper.ReadInConfig()     // Find and read the config file
	if err != nil {                 // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %w \n", err))
	}

	AppCfg.RecordingsAbsolutePath = viper.GetString("dirs.recordingsfolder")
	AppCfg.DbFileName = viper.GetString("db.filename")
	AppCfg.RecordingsFolder = viper.GetString("dirs.recordings")
	AppCfg.DataPath = viper.GetString("dirs.data")
	AppCfg.PublicPath = viper.GetString("dirs.public")
	AppCfg.ScriptPath = viper.GetString("dirs.scripts")
	AppCfg.DataDisk = viper.GetString("sys.disk")
	AppCfg.NetworkDev = viper.GetString("sys.network")
	AppCfg.Default.ImportUrl = viper.GetString("default.importurl")
}

func MakeChannelFolders(channelName string) {
	dir := AbsoluteRecordingsPath(channelName)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Println("Creating folder: " + dir)
		os.MkdirAll(dir, os.ModePerm)
	}
	dataPath := AbsoluteDataPath(channelName)
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		fmt.Println("Creating folder: " + dataPath)
		os.MkdirAll(dataPath, os.ModePerm)
	}
}

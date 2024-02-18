package conf

import (
	"fmt"
	"github.com/spf13/viper"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Cfg struct {
	DbFileName             string
	RecordingsFolder       string
	RecordingsAbsolutePath string
	DataDisk               string
	NetworkDev             string
	DataPath               string
	//PublicPath             string
	//ScriptPath             string
	DefaultImportUrl string
	MinRecMin        int
}

const (
	VideosFolder  = "videos"
	StripesFolder = "stripes"
	PostersFolder = "posters"
	winFont       = "C\\\\:/Windows/Fonts/DMMono-Regular.ttf"
	linuxFont     = "/usr/share/fonts/truetype/DMMono-Regular.ttf"
	// FrameCount Number of extracted frames or timeline/preview
	FrameCount       = 96
	FrameWidth       = "480"
	SnapshotFilename = "live.jpg"
)

var (
	AppCfg      = &Cfg{}
	ThreadCount = uint(float32(runtime.NumCPU() / 2))
)

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
	AbsolutePosterPath     string
	Filepath               string
	VideosPath             string
	StripePath             string
	CoverPath              string
	JPG                    string
	MP4                    string
}

func GetRecordingsPaths(channelName, filename string) RecordingPaths {
	posterJpg := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".jpg"
	stripeJpg := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".jpg"
	mp4 := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".mp4"

	return RecordingPaths{
		AbsoluteRecordingsPath: AppCfg.RecordingsAbsolutePath,

		Filepath:           AbsoluteFilepath(channelName, filename),
		VideosPath:         filepath.Join(DataPath(channelName), VideosFolder, mp4),
		StripePath:         filepath.Join(DataPath(channelName), StripesFolder, stripeJpg),
		CoverPath:          filepath.Join(DataPath(channelName), PostersFolder, posterJpg),
		AbsoluteVideosPath: filepath.Join(AbsoluteDataPath(channelName), VideosFolder, mp4),
		AbsoluteStripePath: filepath.Join(AbsoluteDataPath(channelName), StripesFolder, stripeJpg),
		AbsolutePosterPath: filepath.Join(AbsoluteDataPath(channelName), PostersFolder, posterJpg),
		JPG:                stripeJpg,
		MP4:                mp4,
	}
}

func GetRelativeRecordingsPath(channelName, filename string) string {
	return filepath.Join(AppCfg.RecordingsFolder, channelName, filename)
}

func GetAbsoluteRecordingsPath(channelName, filename string) string {
	return filepath.Join(AppCfg.RecordingsAbsolutePath, channelName, filename)
}

func getConfInt(key, envKey string) int {
	val := os.Getenv(envKey)
	if val == "" {
		return viper.GetInt(key)
	}

	n, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("[getConfInt] Error parsing env variable '%s': %v", envKey, err)
	}

	return n
}

func getConfString(key, envKey string) string {
	val := os.Getenv(envKey)
	if val == "" {
		val = viper.GetString(key)
	}
	if val == "" {
		log.Panicf("Missing config file value for key %s", key)
	}
	return val
}

func Read() {
	viper.SetConfigName("conf/app") // name of config file (without extension)
	viper.AddConfigPath("./")       // path to look for the config file in
	err := viper.ReadInConfig()     // Find and read the config file
	if err != nil {                 // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %w \n", err))
	}

	AppCfg.DbFileName = getConfString("db.filename", "DB_FILENAME")

	AppCfg.RecordingsAbsolutePath = getConfString("dirs.recordingsfolder", "REC_PATH")
	AppCfg.RecordingsFolder = getConfString("dirs.recordings", "REC_FOLDERNAME")
	AppCfg.DataPath = getConfString("dirs.data", "DATA_DIR")
	//AppCfg.PublicPath = getConfString("dirs.public", "PUBLIC_PATH")

	AppCfg.DataDisk = getConfString("sys.disk", "DATA_DISK")
	AppCfg.NetworkDev = getConfString("sys.network", "NET_ADAPTER")

	AppCfg.DefaultImportUrl = getConfString("default.importurl", "DEFAULT_IMPORT_URL")
	AppCfg.MinRecMin = getConfInt("settings.minrecmin", "MIN_REC_MIN")
}

func MakeChannelFolders(channelName string) {
	dir := AbsoluteRecordingsPath(channelName)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Println("Creating folder: " + dir)
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			log.Printf("Error creating folder: '%s': %s", dir, err.Error())
		}
	}
	dataPath := AbsoluteDataPath(channelName)
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		fmt.Println("Creating folder: " + dataPath)
		if err := os.MkdirAll(dataPath, os.ModePerm); err != nil {
			log.Printf("Error creating data path '%s': %s", dataPath, err.Error())
		}
		if err := copyDefaultSnapshotTo(dataPath); err != nil {
			log.Println(err)
		}
	}
}

func copyDefaultSnapshotTo(dataPath string) error {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
	}

	filePath := filepath.Join(pwd, "assets", "live.jpg")
	srcFile, err := os.Open(filePath)
	check(err)
	defer func(srcFile *os.File) {
		if err := srcFile.Close(); err != nil {
			log.Printf("Error copying default live.jpg image to folder '%s': %s", filePath, err.Error())
		}
	}(srcFile)

	destFile, err := os.Create(filepath.Join(dataPath, SnapshotFilename)) // creates if file doesn't exist
	check(err)
	defer func(destFile *os.File) {
		if err := destFile.Close(); err != nil {
			log.Printf("Error creating snapshot file: %s", err.Error())
		}
	}(destFile)

	_, err = io.Copy(destFile, srcFile) // check first var for number of bytes copied
	check(err)

	return destFile.Sync()
}

func check(err error) {
	if err != nil {
		fmt.Printf("Error : %s\n", err.Error())
		os.Exit(1)
	}
}

func GetFontPath() string {
	if runtime.GOOS == "windows" {
		return winFont
	}
	return linuxFont
}

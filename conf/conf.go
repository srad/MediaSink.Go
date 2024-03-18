package conf

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

const (
	VideosFolder  = "videos"
	StripesFolder = "stripes"
	PostersFolder = "posters"
	ScreensFolder = "screens"
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

type Cfg struct {
	DbFileName             string
	RecordingsAbsolutePath string
	DataDisk               string
	NetworkDev             string
	DataPath               string
	// PublicPath             string
	// ScriptPath             string
	MinRecMin int
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
	//ScreensPath            string
}

type VideoPaths struct {
	Filepath string
}

func RelativeDataPath(channelName string) string {
	return filepath.Join(channelName, AppCfg.DataPath)
}

func AbsoluteDataPath(channelName string) string {
	return filepath.Join(AppCfg.RecordingsAbsolutePath, channelName, AppCfg.DataPath)
}

func ChannelPath(channelName, filename string) string {
	return filepath.Join(channelName, filename)
}

func AbsoluteChannelFilePath(channelName, filename string) string {
	return filepath.Join(AppCfg.RecordingsAbsolutePath, channelName, filename)
}

func AbsoluteChannelPath(channelName string) string {

	return filepath.Join(AppCfg.RecordingsAbsolutePath, channelName)
}

func GetRecordingsPaths(channelName, filename string) RecordingPaths {
	posterJpg := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".jpg"
	stripeJpg := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".jpg"
	mp4 := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".mp4"

	return RecordingPaths{
		AbsoluteRecordingsPath: AppCfg.RecordingsAbsolutePath,

		Filepath:   AbsoluteChannelFilePath(channelName, filename),
		VideosPath: filepath.Join(RelativeDataPath(channelName), VideosFolder, mp4),
		StripePath: filepath.Join(RelativeDataPath(channelName), StripesFolder, stripeJpg),
		CoverPath:  filepath.Join(RelativeDataPath(channelName), PostersFolder, posterJpg),
		//ScreensPath:        filepath.Join(RelativeDataPath(channelName), ScreensFolder, filename),
		AbsoluteVideosPath: filepath.Join(AbsoluteDataPath(channelName), VideosFolder, mp4),
		AbsoluteStripePath: filepath.Join(AbsoluteDataPath(channelName), StripesFolder, stripeJpg),
		AbsolutePosterPath: filepath.Join(AbsoluteDataPath(channelName), PostersFolder, posterJpg),
		JPG:                stripeJpg,
		MP4:                mp4,
	}
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
		panic(fmt.Errorf("fatal error config file: %w \n", err))
	}

	AppCfg.DbFileName = getConfString("db.filename", "DB_FILENAME")

	AppCfg.RecordingsAbsolutePath = getConfString("dirs.recordings", "REC_PATH")
	AppCfg.DataPath = getConfString("dirs.data", "DATA_DIR")

	AppCfg.DataDisk = getConfString("sys.disk", "DATA_DISK")
	AppCfg.NetworkDev = getConfString("sys.network", "NET_ADAPTER")

	AppCfg.MinRecMin = getConfInt("settings.minrecmin", "MIN_REC_MIN")
}

func MakeChannelFolders(channelName string) {
	dir := AbsoluteChannelPath(channelName)
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

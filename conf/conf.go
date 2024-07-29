package conf

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"runtime"
	"strconv"
)

const (
	//ScreensFolder = "screens"
	winFont   = "C\\\\:/Windows/Fonts/DMMono-Regular.ttf"
	linuxFont = "/usr/share/fonts/truetype/DMMono-Regular.ttf"
	// FrameCount Number of extracted frames or timeline/preview
	FrameCount = 96
	FrameWidth = "480"
)

var (
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
}

type VideoPaths struct {
	Filepath string
}

func getConfInt(key, envKey string) int {
	val := os.Getenv(envKey)
	if val == "" {
		return viper.GetInt(key)
	}

	n, err := strconv.Atoi(val)
	if err != nil {
		log.Errorf("[getConfInt] Error parsing env variable '%s': %s", envKey, err)
	}

	return n
}

func getConfString(key, envKey string) string {
	val := os.Getenv(envKey)
	if val == "" {
		val = viper.GetString(key)
	}
	if val == "" {
		log.Errorf("Missing config file value for key %s", key)
	}
	return val
}

func Read() *Cfg {
	viper.SetConfigName("conf/app") // name of config file (without extension)
	viper.AddConfigPath("./")       // path to look for the config file in
	err := viper.ReadInConfig()     // Find and read the config file
	if err != nil {                 // Handle errors reading the config file
		log.Warnf("config file not found, will try to find env varibles: %s", err)
	}

	return &Cfg{
		DbFileName:             getConfString("db.filename", "DB_FILENAME"),
		RecordingsAbsolutePath: getConfString("dirs.recordings", "REC_PATH"),
		DataPath:               getConfString("dirs.data", "DATA_DIR"),
		DataDisk:               getConfString("sys.disk", "DATA_DISK"),
		NetworkDev:             getConfString("sys.network", "NET_ADAPTER"),
	}
}

func GetFontPath() string {
	if runtime.GOOS == "windows" {
		return winFont
	}
	return linuxFont
}

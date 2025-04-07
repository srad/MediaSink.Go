package conf

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
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
	ThreadCount   = uint(float32(runtime.NumCPU() / 2))
	configMissing = false
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

func getConfInt(key, envKey string) (int, error) {
	val := os.Getenv(envKey)
	if val == "" {
		return 0, fmt.Errorf("%s env variable is empty", envKey)
	}

	n, err := strconv.Atoi(val)
	if err != nil {
		log.Errorf("[getConfInt] Error parsing env variable '%s': %s", envKey, err)
	}

	return n, nil
}

func getConfString(key, envKey string) (string, error) {
	val := os.Getenv(envKey)
	if val == "" {
		val = viper.GetString(key)
	}
	if val == "" {
		return "", fmt.Errorf("%s env variable is empty", envKey)
	}
	return val, nil
}

func Read() Cfg {
	viper.SetConfigName("conf/app") // name of config file (without extension)
	viper.AddConfigPath("./")       // path to look for the config file in
	err := viper.ReadInConfig()     // Find and read the config file
	// Only log once, that not yml file has been found
	if err != nil && !configMissing {
		configMissing = true
		log.Warnf("config file not found, will try to find env varibles: %s", err)
	}

	// If any needed configuration is missing, panic.
	db, err := getConfString("db.filename", "DB_FILENAME")
	if err != nil {
		log.Panicln(err)
	}

	path, err := getConfString("dirs.recordings", "REC_PATH")
	if err != nil {
		log.Panicln(err)
	}

	dataPath, err := getConfString("dirs.data", "DATA_DIR")
	if err != nil {
		log.Panicln(err)
	}

	dataDisk, err := getConfString("sys.disk", "DATA_DISK")
	if err != nil {
		log.Panicln(err)
	}

	network, err := getConfString("sys.network", "NET_ADAPTER")
	if err != nil {
		log.Panicln(err)
	}

	return Cfg{
		DbFileName:             db,
		RecordingsAbsolutePath: path,
		DataPath:               dataPath,
		DataDisk:               dataDisk,
		NetworkDev:             network,
	}
}

func GetFontPath() string {
	if runtime.GOOS == "windows" {
		return winFont
	}
	return linuxFont
}

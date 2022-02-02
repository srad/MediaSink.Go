package v1

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/models"
)

var (
	rChannel, _ = regexp.Compile("(?i)^[a-z_0-9]+$")
)

type ChannelResponse struct {
	models.Channel
	IsRecording  bool    `json:"isRecording"`
	IsOnline     bool    `json:"isOnline"`
	Preview      string  `json:"preview"`
	MinRecording float64 `json:"minRecording"`
}

func GetChannels(c *gin.Context) {
	appG := app.Gin{C: c}
	channels, err := models.ChannelList()
	response := make([]ChannelResponse, len(channels))

	for index, channel := range channels {
		// Add to each channel current system information
		response[index] = ChannelResponse{Channel: *channel,
			Preview:      filepath.Join(conf.AppCfg.RecordingsFolder, channel.ChannelName, conf.AppCfg.DataPath, "live.jpg"),
			IsOnline:     channel.IsOnline(),
			IsRecording:  channel.IsRecording(),
			MinRecording: channel.RecordingMinutes()}
	}

	if err != nil {
		appG.Response(http.StatusInternalServerError, nil)
		return
	}

	appG.Response(http.StatusOK, &response)
}

type RequeestAddChannel struct {
	ChannelName string `json:"channelName"`
	Url         string `json:"url"`
}

func AddChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	data := &RequeestAddChannel{}
	if err := c.BindJSON(&data); err != nil {
		log.Printf("[AddChannel] Error parsing request: %v", err)
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	url := strings.ToLower(strings.TrimSpace(data.Url))
	if !rChannel.MatchString(data.ChannelName) || len(url) == 0 {
		log.Printf("[AddChannel] Error validating: %s, %s", data.ChannelName, data.Url)
		appG.Response(http.StatusBadRequest, fmt.Sprintf("Parameters wrong, Channel: '%s', Url: '%s'", data.ChannelName, data.Url))
		return
	}

	channelName := strings.ToLower(strings.TrimSpace(data.ChannelName))
	channel := models.Channel{ChannelName: channelName, Url: url, IsPaused: false, CreatedAt: time.Now()}

	if err := models.Db.Create(&channel).Error; err != nil {
		log.Printf("[AddChannel] Error creating record: %v", err)
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, &channel)
}

func DeleteChannel(c *gin.Context) {
	appG := app.Gin{C: c}
	channel, err := models.GetChannelByName(c.Param("channelName"))
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Deleting channel '%s'\n", channel.ChannelName)

	if err := channel.Destroy(); err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, channel)
}

func ResumeChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := strings.ToLower(strings.TrimSpace(c.Param("channelName")))

	if len(channelName) == 0 {
		appG.Response(http.StatusBadRequest, fmt.Sprintf("Invalid channel name '%s'", channelName))
		return
	}

	channel, err := models.GetChannelByName(channelName)
	if err != nil {
		log.Printf("[ResumeChannel] Error getting channel '%s': %v", channelName, err.Error())
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	if err := channel.Start(); err != nil {
		log.Printf("[ResumeChannel] Error resuming channel '%s': %v", channelName, err.Error())
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}
	log.Println("Resuming channel " + channelName)
	appG.Response(http.StatusOK, nil)
}

func PauseChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := strings.ToLower(strings.TrimSpace(c.Param("channelName")))

	if len(channelName) == 0 {
		appG.Response(http.StatusBadRequest, "invalid channel name")
		return
	}

	log.Println("Pausing channel " + channelName)
	channel, err := models.GetChannelByName(channelName)
	if err != nil {
		appG.Response(http.StatusBadRequest, err.Error())
		return
	}
	channel.Stop(true)

	appG.Response(http.StatusOK, nil)
}

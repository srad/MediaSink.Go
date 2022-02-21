package v1

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/model"
)

var (
	rChannel, _ = regexp.Compile("(?i)^[a-z_0-9]+$")
)

type ChannelResponse struct {
	model.Channel
	IsRecording  bool    `json:"isRecording"`
	IsOnline     bool    `json:"isOnline"`
	Preview      string  `json:"preview"`
	MinRecording float64 `json:"minRecording"`
}

func GetChannels(c *gin.Context) {
	appG := app.Gin{C: c}
	channels, err := model.ChannelList()
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

type ReqAddChannel struct {
	ChannelName string    `json:"channelName"`
	Url         string    `json:"url"`
	Tags        *[]string `json:"tags"`
}

type ReqTagChannel struct {
	Tags []string `json:"tags"`
}

func AddChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	data := &ReqAddChannel{}
	if err := c.BindJSON(&data); err != nil {
		log.Printf("[AddChannel] Error parsing request: %v", err)
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	url := strings.TrimSpace(data.Url)
	if !rChannel.MatchString(data.ChannelName) || len(url) == 0 {
		log.Printf("[AddChannel] Error validating: %s, %s", data.ChannelName, data.Url)
		appG.Response(http.StatusBadRequest, fmt.Sprintf("Parameters wrong, Channel: '%s', Url: '%s'", data.ChannelName, data.Url))
		return
	}

	channel := model.Channel{ChannelName: data.ChannelName, Url: url, IsPaused: false, CreatedAt: time.Now()}

	if err := channel.Create(data.Tags); err != nil {
		log.Printf("[AddChannel] Error creating record: %v", err)
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, &channel)
}

func DeleteChannel(c *gin.Context) {
	appG := app.Gin{C: c}
	channel, err := model.GetChannelByName(c.Param("channelName"))
	if err != nil {
		appG.Response(http.StatusNotFound, fmt.Sprintf("Channel not found: %v", err))
		return
	}

	log.Printf("Deleting channel '%s'\n", channel.ChannelName)

	if err := channel.Stop(false); err != nil {
		appG.Response(http.StatusInternalServerError, fmt.Sprintf("Process cound not be terminated: %v", err))
		return
	}

	if err := channel.Destroy(); err != nil {
		appG.Response(http.StatusInternalServerError, fmt.Sprintf("Channel could not be deleted: %v", err))
		return
	}

	appG.Response(http.StatusOK, channel)
}

func TagChannel(c *gin.Context) {
	appG := app.Gin{C: c}
	channelName := c.Param("channelName")

	data := &ReqTagChannel{}
	if err := c.BindJSON(&data); err != nil {
		log.Printf("[TagChannel] Error parsing request: %v", err)
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	if err := model.TagChannel(channelName, data.Tags); err != nil {
		log.Println(err)
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, nil)
}

func ResumeChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := strings.ToLower(strings.TrimSpace(c.Param("channelName")))

	if len(channelName) == 0 {
		appG.Response(http.StatusBadRequest, fmt.Sprintf("Invalid channel name '%s'", channelName))
		return
	}

	channel, err := model.GetChannelByName(channelName)
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

func FavChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	channel := model.Channel{ChannelName: c.Param("channelName"), Fav: true}

	if err := channel.FavChannel(); err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, nil)
}

func UnFavChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	channel := model.Channel{ChannelName: c.Param("channelName"), Fav: false}
	if err := channel.UnFavChannel(); err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, nil)
}

func UploadChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		appG.Response(http.StatusBadRequest, err.Error())
		return
	}

	channel := model.Channel{ChannelName: c.Param("channelName"), Fav: false}
	recording, outputPath := channel.NewRecording()

	out, err := os.Create(outputPath)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	recording.Save("recording")
	model.EnqueuePreviewJob(recording.ChannelName, recording.Filename)

	appG.Response(http.StatusOK, recording)
}

func PauseChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := c.Param("channelName")

	if len(channelName) == 0 {
		appG.Response(http.StatusBadRequest, "invalid channel name")
		return
	}

	log.Println("Pausing channel " + channelName)
	channel, err := model.GetChannelByName(channelName)
	if err != nil {
		appG.Response(http.StatusBadRequest, err.Error())
		return
	}
	channel.Stop(true)

	appG.Response(http.StatusOK, nil)
}

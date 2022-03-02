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

type ChannelRequest struct {
	ChannelId   *uint     `json:"channelId"`
	ChannelName string    `json:"channelName"`
	DisplayName string    `json:"displayName"`
	SkipStart   uint      `json:"skipStart"`
	Url         string    `json:"url"`
	Tags        *[]string `json:"tags"`
}

type TagChannelRequest struct {
	Tags []string `json:"tags"`
}

// GetChannels godoc
// @Summary     Return a list of channels
// @Schemes
// @Description Return a list of channels
// @Tags        channels
// @Accept      json
// @Produce     json
// @Success     200 {object} []ChannelResponse
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /channels [get]
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

// AddChannel godoc
// @Summary     Add a new channel
// @Description Add a new channel
// @Tags        channels
// @Param       ChannelRequest body ChannelRequest true "Channel data"
// @Accept      json
// @Produce     json
// @Success     200 {object} models.Channel
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels [post]
func AddChannel(c *gin.Context) {
	appG := app.Gin{C: c}
	data := &ChannelRequest{}
	if err := c.BindJSON(&data); err != nil {
		log.Printf("[AddChannel] Error parsing request: %s", err.Error())
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}
	url := strings.TrimSpace(data.Url)
	if !rChannel.MatchString(data.ChannelName) || len(url) == 0 {
		log.Printf("[AddChannel] Error validating: %s, %s", data.ChannelName, data.Url)
		appG.Response(http.StatusBadRequest, fmt.Sprintf("Parameters wrong, Channel: '%s', Url: '%s'", data.ChannelName, data.Url))
		return
	}

	channel := models.Channel{
		ChannelName:     data.ChannelName,
		DisplayName:     data.DisplayName,
		SkipStart:       data.SkipStart,
		Url:             url,
		Fav:             false,
		RecordingsCount: 0,
		RecordingsSize:  0,
		Tags:            "",
		IsPaused:        false,
		CreatedAt:       time.Now()}

	newChannel, err := channel.Create(data.Tags)
	if err != nil {
		log.Printf("[AddChannel] Error creating record: %s", err.Error())
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	res := &ChannelResponse{
		Channel:      *newChannel,
		IsRecording:  false,
		IsOnline:     false,
		Preview:      "",
		MinRecording: 0,
	}

	log.Printf("New channel: %v", res)

	appG.Response(http.StatusOK, &res)
}

// UpdateChannel godoc
// @Summary     Add a new channel
// @Description Add a new channel
// @Tags        channels
// @Param       ChannelRequest body ChannelRequest true "Channel data"
// @Accept      json
// @Produce     json
// @Success     200 {object} models.Channel
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{channelName} [post]
func UpdateChannel(c *gin.Context) {
	appG := app.Gin{C: c}
	data := &ChannelRequest{}
	if err := c.BindJSON(&data); err != nil {
		log.Printf("[UpdateChannel] Error parsing request: %s", err.Error())
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	url := strings.TrimSpace(data.Url)
	displayName := strings.TrimSpace(data.DisplayName)

	if !rChannel.MatchString(data.ChannelName) || len(url) == 0 || len(displayName) == 0 {
		log.Printf("[UpdateChannel] Error validating: %v", data)
		appG.Response(http.StatusBadRequest, fmt.Sprintf("Invalid parameters: %v", data))
		return
	}

	channel := models.Channel{ChannelId: *data.ChannelId, ChannelName: data.ChannelName, DisplayName: data.DisplayName, SkipStart: data.SkipStart, Url: url}
	if err := channel.Update(); err != nil {
		log.Printf("[UpdateChannel] Error creating record: %s", err.Error())
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, &channel)
}

// DeleteChannel godoc
// @Summary     Delete channel
// @Description Delete channel with all recordings
// @Tags        channels
// @Accept      json
// @Produce     json
// @Param       channelName path string true  "List of tags"
// @Success     200 {object} models.Channel
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /channels/{channelName} [delete]
func DeleteChannel(c *gin.Context) {
	appG := app.Gin{C: c}
	channel, err := models.GetChannelByName(c.Param("channelName"))
	if err != nil {
		appG.Response(http.StatusNotFound, fmt.Sprintf("Channel not found: %s", err.Error()))
		return
	}

	log.Printf("Deleting channel '%s'\n", channel.ChannelName)

	if err := channel.Terminate(false); err != nil {
		appG.Response(http.StatusInternalServerError, fmt.Sprintf("Process cound not be terminated: %s", err.Error()))
		return
	}

	if err := channel.Destroy(); err != nil {
		appG.Response(http.StatusInternalServerError, fmt.Sprintf("Channel could not be deleted: %s", err.Error()))
		return
	}

	appG.Response(http.StatusOK, &channel)
}

// TagChannel godoc
// @Summary     Tag a channel
// @Description Delete channel with all recordings
// @Tags        channels
// @Accept      json
// @Produce     json
// @Param       TagChannelRequest body TagChannelRequest true "Channel data"
// @Param       channelName path string true "Channel name"
// @Success     200 {object} models.Channel
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /channels/{channelName}/tags [post]
func TagChannel(c *gin.Context) {
	appG := app.Gin{C: c}
	channelName := c.Param("channelName")

	data := &TagChannelRequest{}
	if err := c.BindJSON(&data); err != nil {
		log.Printf("[TagChannel] Error parsing request: %s", err.Error())
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	if err := models.TagChannel(channelName, data.Tags); err != nil {
		log.Println(err)
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, nil)
}

// ResumeChannel godoc
// @Summary     Tag a channel
// @Description Delete channel with all recordings
// @Tags        channels
// @Accept      json
// @Produce     json
// @Param       channelName path string true "Channel name"
// @Success     200 {} http.StatusOK
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{channelName}/resume [post]
func ResumeChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := c.Param("channelName")

	if len(channelName) == 0 {
		appG.Response(http.StatusBadRequest, fmt.Sprintf("Invalid channel name '%s'", channelName))
		return
	}

	channel, err := models.GetChannelByName(channelName)
	if err != nil {
		log.Printf("[ResumeChannel] Error getting channel '%s': %s", channelName, err.Error())
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	if err := channel.Start(); err != nil {
		log.Printf("[ResumeChannel] Error resuming channel '%s': %s", channelName, err.Error())
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}
	log.Println("Resuming channel " + channelName)
	appG.Response(http.StatusOK, nil)
}

// FavChannel godoc
// @Summary     Mark channel as one of favorites
// @Description Mark channel as one of favorites
// @Tags        channels
// @Accept      json
// @Produce     json
// @Param       channelName path string true "Channel name"
// @Success     200 {} http.StatusOK
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{channelName}/fav [post]
func FavChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	channel := models.Channel{ChannelName: c.Param("channelName"), Fav: true}

	if err := channel.FavChannel(); err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, nil)
}

// UnFavChannel godoc
// @Summary     Remove channel as one of favorites
// @Description Remove channel as one of favorites
// @Tags        channels
// @Accept      json
// @Produce     json
// @Param       channelName path string true "Channel name"
// @Success     200 {} http.StatusOK
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{channelName}/unfav [post]
func UnFavChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	channel := models.Channel{ChannelName: c.Param("channelName"), Fav: false}
	if err := channel.UnFavChannel(); err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, nil)
}

// UploadChannel godoc
// @Summary     Add a new channel
// @Description Add a new channel
// @Tags        channels
// @Param       []byte body []byte true "Uploaded file chunk"
// @Param       channelName path string true "Channel name"
// @Accept      json
// @Produce     json
// @Success     200 {object} models.Recording
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{channelName}/upload [post]
func UploadChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		appG.Response(http.StatusBadRequest, err.Error())
		return
	}

	channel := models.Channel{ChannelName: c.Param("channelName"), Fav: false}
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
	models.EnqueuePreviewJob(recording.ChannelName, recording.Filename)

	appG.Response(http.StatusOK, recording)
}

// PauseChannel godoc
// @Summary     Pause channel for recording
// @Description Pause channel for recording
// @Tags        channels
// @Accept      json
// @Produce     json
// @Param       channelName path string true "Channel name"
// @Success     200 {} http.StatusOK
// @Failure     400 {} http.StatusBadRequest
// @Router      /channels/{channelName}/pause [post]
func PauseChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := c.Param("channelName")

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
	channel.Terminate(true)

	appG.Response(http.StatusOK, nil)
}

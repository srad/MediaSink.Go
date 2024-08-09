package v1

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/models"
	"gorm.io/gorm"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type ChannelRequest struct {
	ChannelName string       `json:"channelName" extensions:"!x-nullable"`
	DisplayName string       `json:"displayName" extensions:"!x-nullable"`
	SkipStart   uint         `json:"skipStart" extensions:"!x-nullable"`
	MinDuration uint         `json:"minDuration" extensions:"!x-nullable"`
	Url         string       `json:"url" extensions:"!x-nullable"`
	IsPaused    bool         `json:"isPaused" extensions:"!x-nullable"`
	Tags        *models.Tags `json:"tags"`
	Fav         bool         `json:"fav"`
	Deleted     bool         `json:"deleted"`
}

type ChannelResponse struct {
	models.Channel
	IsRecording   bool    `json:"isRecording" extensions:"!x-nullable"`
	IsOnline      bool    `json:"isOnline" extensions:"!x-nullable"`
	IsTerminating bool    `json:"isTerminating" extensions:"!x-nullable"`
	Preview       string  `json:"preview" extensions:"!x-nullable"`
	MinRecording  float64 `json:"minRecording" extensions:"!x-nullable"`
}

type ChannelTagsUpdateRequest struct {
	Tags *models.Tags `json:"tags"`
}

// https://github.com/swaggo/swag/blob/master/README.md#declarative-comments-format
// Parameters that separated by spaces: | param name | param type | data type | is mandatory? | comment attribute(optional) |
//
// Param Type
// ----------------------------
// query
// path
// header
// body
// formData
//
// Data Type
// ----------------------------
// string (string)
// integer (int, uint, uint32, uint64)
// number (float32)
// boolean (bool)
// file (param data type when uploading)
// user defined struct

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
	channels, err := models.ChannelListNotDeleted()
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	response := make([]ChannelResponse, len(channels))

	cfg := conf.Read()

	for index, channel := range channels {
		// Add to each channel current system information
		response[index] = ChannelResponse{
			Channel:       *channel,
			Preview:       filepath.Join(channel.ChannelName.String(), cfg.DataPath, models.SnapshotFilename),
			IsOnline:      channel.ChannelId.IsOnline(),
			IsTerminating: channel.ChannelId.IsTerminating(),
			IsRecording:   channel.ChannelId.IsRecording(),
			MinRecording:  channel.ChannelId.GetRecordingMinutes()}
	}

	appG.Response(http.StatusOK, &response)
}

// GetChannel godoc
// @Summary     Return the data of one channel
// @Schemes
// @Description Return the data of one channel
// @Param       id path uint true  "Channel id"
// @Tags        channels
// @Produce     json
// @Success     200 {object} ChannelResponse
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /channels/{id} [get]
func GetChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	channelId := models.ChannelId(id)
	if channel, err := channelId.GetChannelById(); err != nil {
		log.Errorf("[GetChannel] Error getting channel: %s", err)
		appG.Response(http.StatusInternalServerError, err)
	} else {
		res := &ChannelResponse{
			Channel:       *channel,
			IsOnline:      channelId.IsOnline(),
			IsTerminating: channelId.IsTerminating(),
			IsRecording:   channelId.IsRecording(),
			MinRecording:  channelId.GetRecordingMinutes()}

		appG.Response(http.StatusOK, &res)
	}
}

// CreateChannel godoc
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
func CreateChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	data := &ChannelRequest{}
	if err := c.BindJSON(&data); err != nil {
		errReq := fmt.Errorf("error parsing request: %s", err)
		log.Errorln(errReq)
		appG.Response(http.StatusInternalServerError, errReq)
		return
	}

	channel := models.Channel{
		ChannelName: models.ChannelName(data.ChannelName),
		DisplayName: data.DisplayName,
		SkipStart:   data.SkipStart,
		MinDuration: data.MinDuration,
		CreatedAt:   time.Now(),
		Url:         data.Url,
		Fav:         data.Fav,
		Tags:        data.Tags,
		IsPaused:    data.IsPaused}

	if _, err := channel.CreateChannelDetail(); errors.Is(err, gorm.ErrDuplicatedKey) {
		errDuplicate := fmt.Errorf("error creating record: %s", err)
		log.Errorln(errDuplicate)
		appG.Response(http.StatusInternalServerError, errDuplicate)
		return
	} else if err != nil {
		errCreate := fmt.Errorf("error creating record: %s", err)
		log.Errorln(errCreate)
		appG.Response(http.StatusInternalServerError, errCreate)
		return
	}

	cfg := conf.Read()

	res := &ChannelResponse{
		Channel:      channel,
		IsRecording:  false,
		IsOnline:     false,
		Preview:      filepath.Join(channel.ChannelName.AbsoluteChannelPath(), cfg.DataPath, models.SnapshotFilename),
		MinRecording: 0,
	}

	log.Infof("New channel: %v", res)

	appG.Response(http.StatusOK, &res)
}

// UpdateChannel godoc
// @Summary     Update channel data
// @Description Update channel data
// @Tags        channels
// @Param       id path uint true "Channel id"
// @Param       ChannelRequestRequest body ChannelRequest true "Channel data"
// @Accept      json
// @Produce     json
// @Success     200 {object} models.Channel
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{id} [patch]
func UpdateChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	// Id
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	// Body
	data := &ChannelRequest{}
	if err := c.BindJSON(&data); err != nil {
		log.Errorf("[UpdateChannel] Error parsing request: %s", err)
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	channel := models.Channel{
		ChannelId:   models.ChannelId(id),
		ChannelName: models.ChannelName(data.ChannelName),
		DisplayName: data.DisplayName,
		SkipStart:   data.SkipStart,
		MinDuration: data.MinDuration,
		Url:         data.Url,
		Tags:        data.Tags,
		Fav:         data.Fav,
		IsPaused:    data.IsPaused,
		Deleted:     data.Deleted,
	}

	if err := channel.Update(); err != nil {
		message := fmt.Errorf("error creating record: %s", err)
		log.Errorln(message)
		appG.Response(http.StatusInternalServerError, message)
		return
	}

	if channel.IsPaused == true {
		if err := channel.ChannelId.TerminateProcess(); err != nil {
			message := fmt.Errorf("error stopping stream: %s", err)
			log.Errorln(message)
			appG.Response(http.StatusInternalServerError, message)
			return
		}
	}

	appG.Response(http.StatusOK, &channel)
}

// DeleteChannel godoc
// @Summary     Delete channel
// @Description Delete channel with all recordings
// @Tags        channels
// @Accept      json
// @Produce     json
// @Param       id path uint true  "List of tags"
// @Success     200 {} models.Channel
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /channels/{id} [delete]
func DeleteChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	channelId := models.ChannelId(id)

	if err := channelId.TerminateProcess(); err != nil {
		appG.Response(http.StatusInternalServerError, fmt.Sprintf("Process cound not be terminated: %s", err))
		return
	}

	if err := channelId.SoftDestroyChannel(); err != nil {
		appG.Response(http.StatusInternalServerError, fmt.Sprintf("Channel could not be deleted: %s", err))
		return
	}

	appG.Response(http.StatusOK, nil)
}

// TagChannel godoc
// @Summary     Tag a channel
// @Description Tag a channel
// @Tags        channels
// @Accept      json
// @Param       ChannelTagsUpdateRequest body ChannelTagsUpdateRequest true "Channel data"
// @Param       id path uint true "Channel id"
// @Success     200 {} nil
// @Failure     500 {}  http.StatusInternalServerError
// @Failure     400 {}  http.StatusBadRequest
// @Router      /channels/{id}/tags [patch]
func TagChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	request := &ChannelTagsUpdateRequest{}
	if err := c.BindJSON(&request); err != nil {
		log.Errorf("[TagChannel] Error parsing request: %s", err)
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	update := &models.ChannelTagsUpdate{
		ChannelId: models.ChannelId(id),
		Tags:      request.Tags,
	}

	if err := update.TagChannel(); err != nil {
		log.Errorln(err)
		appG.Response(http.StatusInternalServerError, err)
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
// @Param       id path uint true "Channel id"
// @Success     200 {} http.StatusOK
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{id}/resume [post]
func ResumeChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	channelId := models.ChannelId(id)

	if err := channelId.Start(); err != nil {
		log.Errorf("[ResumeChannel] Error resuming channel-id %d: %s", channelId, err)
		appG.Response(http.StatusInternalServerError, err)
		return
	}
	log.Infof("Resuming channel %d", id)
	appG.Response(http.StatusOK, nil)
}

// FavChannel godoc
// @Summary     Mark channel as one of favorites
// @Description Mark channel as one of favorites
// @Tags        channels
// @Accept      json
// @Produce     json
// @Param       id path uint true "Channel id"
// @Success     200 {} http.StatusOK
// @Failure     500 {} http.StatusInternalServerError
// @Router       /channels/{id}/fav [patch]
func FavChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	channelId := models.ChannelId(id)

	if err := channelId.FavChannel(); err != nil {
		appG.Response(http.StatusInternalServerError, err)
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
// @Param       id path uint true "Channel id"
// @Success     200 {} http.StatusOK
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{id}/unfav [patch]
func UnFavChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	channelId := models.ChannelId(id)

	if err := channelId.UnFavChannel(); err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

// Parameters that separated by spaces: | param name | param type | data type | is mandatory? | comment attribute(optional) |

// UploadChannel godoc
// @Summary     Add a new channel
// @Description Add a new channel
// @Tags        channels
// @Param       id path uint true "Channel id"
// @Param       file formData []byte true "Uploaded file chunk"
// @Accept      json
// @Produce     json
// @Success     200 {object} models.Recording
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{id}/upload [post]
func UploadChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	channelId := models.ChannelId(id)
	recording, outputPath, err := channelId.NewRecording("recording")
	models.CreateRecording(recording.ChannelId, recording.Filename, "recording")

	out, err := os.Create(outputPath)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	if err := recording.Save(); err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	recording.RecordingId.EnqueuePreviewJob()

	appG.Response(http.StatusOK, recording)
}

// PauseChannel godoc
// @Summary     Pause channel for recording
// @Description Pause channel for recording
// @Tags        channels
// @Accept      json
// @Produce     json
// @Param       id path uint true "Channel id"
// @Success     200 {} http.StatusOK
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{id}/pause [post]
func PauseChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	channelId := models.ChannelId(id)

	if err := channelId.TerminateProcess(); err != nil {
		log.Errorf("Error teminating process: %s", err)
	}

	if err := channelId.PauseChannel(true); err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

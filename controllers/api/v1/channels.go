package v1

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/models/requests"
	"github.com/srad/streamsink/services"
)

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
// @Success     200 {object} []services.ChannelInfo
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /channels [get]
func GetChannels(c *gin.Context) {
	appG := app.Gin{C: c}
	if response, err := services.GetChannels(); err != nil {
		appG.Error(http.StatusInternalServerError, err)
	} else {
		appG.Response(http.StatusOK, &response)
	}
}

// GetProcesses godoc
// @Summary     Return a list of streaming processes
// @Schemes
// @Description Return a list of streaming processes
// @Tags        processes
// @Accept      json
// @Produce     json
// @Success     200 {object} []services.ProcessInfo
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /processes [get]
func GetProcesses(c *gin.Context) {
	appG := app.Gin{C: c}
	appG.Response(http.StatusOK, services.ProcessList())
}

// GetChannel godoc
// @Summary     Return the data of one channel
// @Schemes
// @Description Return the data of one channel
// @Param       id path uint true  "Channel id"
// @Tags        channels
// @Produce     json
// @Success     200 {object} services.ChannelInfo
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /channels/{id} [get]
func GetChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	if id, err := strconv.ParseUint(c.Param("id"), 10, 32); err != nil {
		appG.Error(http.StatusBadRequest, fmt.Errorf("invalid id type: %s", err))
		return
	} else {
		if response, err2 := services.GetChannel(uint(id)); err2 != nil {
			appG.Error(http.StatusInternalServerError, err2)
			return
		} else {
			appG.Response(http.StatusOK, response)
		}
	}
}

// CreateChannel godoc
// @Summary     Add a new channel
// @Description Add a new channel
// @Tags        channels
// @Param       ChannelRequest body requests.ChannelRequest true "Channel data"
// @Accept      json
// @Produce     json
// @Success     200 {object} services.ChannelInfo
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels [post]
func CreateChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	// Parse JSON
	data := &requests.ChannelRequest{}
	if err := c.BindJSON(&data); err != nil {
		errReq := fmt.Errorf("error parsing request: %s", err)
		log.Errorln(errReq)
		appG.Error(http.StatusBadRequest, errReq)
		return
	}

	if newChannel, err := services.CreateChannel(data.ChannelName, data.DisplayName, data.SkipStart, data.MinDuration, data.Url, data.Fav, data.Tags, data.IsPaused); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	} else {
		appG.Response(http.StatusOK, newChannel)
	}
}

// UpdateChannel godoc
// @Summary     Update channel data
// @Description Update channel data
// @Tags        channels
// @Param       id path uint true "Channel id"
// @Param       ChannelRequest body requests.ChannelRequest true "Channel data"
// @Accept      json
// @Produce     json
// @Success     200 {object} database.Channel
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{id} [patch]
func UpdateChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	// ID
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	// Body
	data := &requests.ChannelRequest{}
	if err := c.BindJSON(&data); err != nil {
		log.Errorf("[UpdateChannel] Error parsing request: %s", err)
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	channel := database.Channel{
		ChannelID:   database.ChannelID(id),
		ChannelName: database.ChannelName(data.ChannelName),
		DisplayName: data.DisplayName,
		SkipStart:   data.SkipStart,
		MinDuration: data.MinDuration,
		URL:         data.Url,
		Tags:        data.Tags,
		Fav:         data.Fav,
		IsPaused:    data.IsPaused,
		Deleted:     data.Deleted,
	}

	if err := channel.Update(); err != nil {
		message := fmt.Errorf("error creating record: %s", err)
		log.Errorln(message)
		appG.Error(http.StatusInternalServerError, message)
		return
	}

	if channel.IsPaused {
		if err := services.TerminateProcess(channel.ChannelID); err != nil {
			message := fmt.Errorf("error stopping stream: %s", err)
			log.Errorln(message)
			appG.Error(http.StatusInternalServerError, message)
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
// @Success     200 {} database.Channel
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /channels/{id} [delete]
func DeleteChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	if err := services.DeleteChannel(database.ChannelID(id)); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

// TagChannel godoc
// @Summary     Tag a channel
// @Description Tag a channel
// @Tags        channels
// @Accept      json
// @Param       ChannelTagsUpdateRequest body requests.ChannelTagsUpdateRequest true "Channel data"
// @Param       id path uint true "Channel id"
// @Success     200 {} nil
// @Failure     500 {}  http.StatusInternalServerError
// @Failure     400 {}  http.StatusBadRequest
// @Router      /channels/{id}/tags [patch]
func TagChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	request := &requests.ChannelTagsUpdateRequest{}
	if err := c.BindJSON(&request); err != nil {
		log.Errorf("[TagChannel] Error parsing request: %s", err)
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	channelID := database.ChannelID(id)
	if err := channelID.TagChannel(request.Tags); err != nil {
		log.Errorln(err)
		appG.Error(http.StatusInternalServerError, err)
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

	id, errParse := strconv.ParseUint(c.Param("id"), 10, 32)
	if errParse != nil {
		appG.Error(http.StatusBadRequest, errParse)
		return
	}

	channelID := database.ChannelID(id)
	if err := channelID.PauseChannel(false); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	if err := services.Start(channelID); err != nil {
		log.Errorf("[ResumeChannel] Error resuming channel-id %d: %s", channelID, err)
		appG.Error(http.StatusInternalServerError, err)
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
		appG.Error(http.StatusBadRequest, err)
		return
	}

	channelID := database.ChannelID(id)

	if err := channelID.FavChannel(); err != nil {
		appG.Error(http.StatusInternalServerError, err)
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
		appG.Error(http.StatusBadRequest, err)
		return
	}

	channelID := database.ChannelID(id)

	if err := channelID.UnFavChannel(); err != nil {
		appG.Error(http.StatusInternalServerError, err)
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
// @Success     200 {object} database.Recording
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /channels/{id}/upload [post]
func UploadChannel(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	channelID := database.ChannelID(id)
	recording, outputPath, err := database.NewRecording(channelID, "recording")

	out, err := os.Create(outputPath)
	if err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	if err := recording.Save(); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	if _, _, _, err := recording.EnqueuePreviewsJob(); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}

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
		appG.Error(http.StatusBadRequest, err)
		return
	}

	channelID := database.ChannelID(id)

	if err := services.TerminateProcess(channelID); err != nil {
		log.Errorf("Error teminating process: %s", err)
	}

	if err := channelID.PauseChannel(true); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

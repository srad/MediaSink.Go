package v1

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/database"
	"github.com/srad/streamsink/services"
)

type CutRequest struct {
	Starts []string `json:"starts"`
	Ends   []string `json:"ends"`
}

// GetRecordings godoc
// @Summary     Return a list of recordings
// @Description Return a list of recordings.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Success     200 {object} []database.Recording
// @Failure     500 {} string "Error message"
// @Router      /recordings [get]
func GetRecordings(c *gin.Context) {
	appG := app.Gin{C: c}
	recordings, err := database.RecordingsList()

	if err != nil {
		appG.Response(http.StatusInternalServerError, nil)
		return
	}

	appG.Response(http.StatusOK, recordings)
}

// GeneratePosters godoc
// @Summary     Return a list of recordings
// @Description Return a list of recordings.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Success     200
// @Failure     500 {} string "Error message"
// @Router      /recordings/generate/posters [post]
func GeneratePosters(c *gin.Context) {
	appG := app.Gin{C: c}

	if err := services.GeneratePosters(); err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

// UpdateVideoInfo godoc
// @Summary     Return a list of recordings
// @Description Return a list of recordings.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Success     200
// @Failure     500 {} string "Error message"
// @Router      /recordings/updateinfo [post]
func UpdateVideoInfo(c *gin.Context) {
	appG := app.Gin{C: c}
	// TODO Make into a cancelable job
	if err := database.UpdateVideoInfo(); err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}
	appG.Response(http.StatusOK, nil)
}

// IsUpdatingVideoInfo godoc
// @Summary     Returns if current the videos are updated.
// @Description Returns if current the videos are updated.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Success     200
// @Failure     500 {} string "Error message"
// @Router      /recordings/isupdating [get]
func IsUpdatingVideoInfo(c *gin.Context) {
	appG := app.Gin{C: c}
	// TODO: do it
	appG.Response(http.StatusOK, true)
}

// GetRecording godoc
// @Summary     Return a list of recordings for a particular channel
// @Description Return a list of recordings for a particular channel.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       id path uint true "Recording item id"
// @Success     200 {object} database.Recording
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{id} [get]
func GetRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	recording, err := database.RecordingId(id).FindById()
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, &recording)
}

// GetBookmarks godoc
// @Summary     Returns all bookmarked recordings
// @Description Returns all bookmarked recordings.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Success     200 {object} []database.Recording
// @Failure     500 {} string "Error message"
// @Router      /recordings/bookmarks [get]
func GetBookmarks(c *gin.Context) {
	appG := app.Gin{C: c}
	recordings, err := database.BookmarkList()

	if err != nil {
		appG.Response(http.StatusInternalServerError, nil)
		return
	}

	appG.Response(http.StatusOK, recordings)
}

// GeneratePreview godoc
// @Summary     Generate preview for a certain video in a channel
// @Description Generate preview for a certain video in a channel.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       id path uint true "Recording item id"
// @Success     200 {object} database.Job
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{id}/preview [post]
func GeneratePreview(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	recordingId := database.RecordingId(id)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
	}

	job, err := services.EnqueuePreviewJob(recordingId)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, job)
}

// FavRecording godoc
// @Summary     Bookmark a certain video in a channel
// @Description Bookmark a certain video in a channel.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       id path uint true "Recording item id"
// @Success     200
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{id}/fav [patch]
func FavRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	if err := database.FavRecording(uint(id), true); err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

// UnfavRecording godoc
// @Summary     Bookmark a certain video in a channel
// @Description Bookmark a certain video in a channel.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       id path uint true "Recording item id"
// @Success     200
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{id}/unfav [patch]
func UnfavRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	if err := database.FavRecording(uint(id), false); err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

// CutRecording godoc
// @Summary     Cut a video and merge all defined segments
// @Description Cut a video and merge all defined segments
// @Tags        recordings
// @Param       id path uint true "Recording item id"
// @Param       CutRequest body CutRequest true "Start and end timestamp of cutting sequences."
// @Accept      json
// @Produce     json
// @Success     200 {object} database.Job
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{id}/cut [post]
func CutRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	cutRequest := CutRequest{}
	if err := c.BindJSON(&cutRequest); err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	cut, err := json.Marshal(cutRequest)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	job, err := services.EnqueueCuttingJob(database.RecordingId(id), string(cut))
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, job)
}

// Convert godoc
// @Summary     Cut a video and merge all defined segments
// @Description Cut a video and merge all defined segments
// @Tags        recordings
// @Param       id path uint true "Recording item id"
// @Param       mediaType path string true "Media type to convert to: 720, 1080, mp3"
// @Accept      json
// @Produce     json
// @Success     200 {object} database.Job
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{id}/{mediaType}/convert [post]
func Convert(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}
	mediaType := c.Param("mediaType")

	job, err := services.EnqueueConversionJob(database.RecordingId(id), mediaType)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, job)
}

// FilterRecordings godoc
// @Summary     Get the top N the latest recordings
// @Description Get the top N the latest recordings.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       limit path string int "How many recordings"
// @Success     200 {object} []database.Recording
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/filter/{column}/{order}/{limit} [get]
func FilterRecordings(c *gin.Context) {
	appG := app.Gin{C: c}

	limit, err := strconv.Atoi(c.Param("limit"))
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	column := c.Param("column")
	order := c.Param("order")

	recordings, err := database.SortBy(column, order, limit)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, recordings)
}

// GetRandomRecordings godoc
// @Summary     Get random videos
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       limit path string int "How many recordings"
// @Success     200 {object} []database.Recording
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/random/{limit} [get]
func GetRandomRecordings(c *gin.Context) {
	appG := app.Gin{C: c}

	limit, err := strconv.Atoi(c.Param("limit"))
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	recordings, err := database.FindRandom(limit)

	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, recordings)
}

// DownloadRecording godoc
// @Summary     Download a file from a channel
// @Description Download a file from a channel.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       channelName path string true "Channel name"
// @Param       filename    path string true "Filename to generate the preview for"
// @Success     200
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{channelName}/{filename}/download [get]
func DownloadRecording(c *gin.Context) {
	channelName := database.ChannelName(c.Param("channelName"))
	c.FileAttachment(channelName.AbsoluteChannelFilePath(database.RecordingFileName(c.Param("filename"))), c.Param("filename"))
}

// DeleteRecording godoc
// @Summary     Delete recording
// @Description Delete recording
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       id path uint true "Recording item id"
// @Success     200
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{id} [delete]
func DeleteRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	rec, err := database.RecordingId(id).FindRecordingById()
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	if rec != nil {
		if err2 := rec.Destroy(); err2 != nil {
			appG.Response(http.StatusInternalServerError, err2)
			return
		}
	}

	appG.Response(http.StatusOK, nil)
}

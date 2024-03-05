package v1

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/conf"
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
		appG.Response(http.StatusInternalServerError, err.Error())
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
	database.UpdateVideoInfo()
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
// @Param       channelName path string true "Channel name"
// @Success     200 {object} []database.Recording
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{channelName} [get]
func GetRecording(c *gin.Context) {
	appG := app.Gin{C: c}
	channelName := c.Param("channelName")

	if channelName == "" {
		appG.Response(http.StatusBadRequest, nil)
		return
	}

	recordings, err := database.FindByName(channelName)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, &recordings)
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
// @Param       channelName path string true "Channel name"
// @Param       filename    path string true "Filename to generate the preview for"
// @Success     200 {object} database.Job
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{channelName}/{filename}/preview [post]
func GeneratePreview(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := c.Param("channelName")
	filename := c.Param("filename")

	job, err := database.EnqueuePreviewJob(channelName, filename)
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
// @Param       channelName path string true "Channel name"
// @Param       filename    path string true "Filename to generate the preview for"
// @Success     200
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{channelName}/{filename}/fav [post]
func FavRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	if err := database.FavRecording(c.Param("channelName"), c.Param("filename"), true); err != nil {
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
// @Param       channelName path string true "Channel name"
// @Param       filename    path string true "Filename to generate the preview for"
// @Success     200
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{channelName}/{filename}/unfav [post]
func UnfavRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	if err := database.FavRecording(c.Param("channelName"), c.Param("filename"), false); err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

// CutRecording godoc
// @Summary     Cut a video and merge all defined segments
// @Description Cut a video and merge all defined segments
// @Tags        recordings
// @Param       CutRequest body CutRequest true "Start and end timestamp of cutting sequences."
// @Accept      json
// @Produce     json
// @Success     200 {object} database.Job
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{channelName}/{filename}/cut [post]
func CutRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	cutRequest := CutRequest{}
	if err := c.BindJSON(&cutRequest); err != nil {
		appG.Response(http.StatusBadRequest, err.Error())
		return
	}

	channelName := c.Param("channelName")
	filename := c.Param("filename")

	cut, err := json.Marshal(cutRequest)
	if err != nil {
		appG.Response(http.StatusBadRequest, err.Error())
		return
	}

	job, err := database.EnqueueCuttingJob(channelName, filename, conf.AbsoluteFilepath(channelName, filename), string(cut))
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, job)
}

// Convert godoc
// @Summary     Cut a video and merge all defined segments
// @Description Cut a video and merge all defined segments
// @Tags        recordings
// @Param       channelName path string true "Channel name"
// @Param       filename path string true "Filename in channel"
// @Param       mediaType path string true "Media type to convert to: 720, 1080, mp3"
// @Accept      json
// @Produce     json
// @Success     200 {object} database.Job
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{channelName}/{filename}/{mediaType}/convert [post]
func Convert(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := c.Param("channelName")
	filename := c.Param("filename")
	mediaType := c.Param("mediaType")

	job, err := database.EnqueueConversionJob(channelName, filename, conf.AbsoluteFilepath(channelName, filename), mediaType)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
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
		appG.Response(http.StatusBadRequest, err.Error())
		return
	}

	column := c.Param("column")
	order := c.Param("order")

	recordings, err := database.SortBy(column, order, limit)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
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
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	recordings, err := database.FindRandom(limit)

	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
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
	c.FileAttachment(conf.AbsoluteFilepath(c.Param("channelName"), c.Param("filename")), c.Param("filename"))
}

// DeleteRecording godoc
// @Summary     Delete recording
// @Description Delete recording
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       channelName path string true "Channel name"
// @Param       filename    path string true "Filename to generate the preview for"
// @Success     200
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{channelName}/{filename} [delete]
func DeleteRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := c.Param("channelName")
	filename := c.Param("filename")

	if channelName == "" || filename == "" {
		appG.Response(http.StatusBadRequest, "invalid params")
		return
	}

	rec, err := database.FindRecording(channelName, filename)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	if err := rec.Destroy(); err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, nil)
}

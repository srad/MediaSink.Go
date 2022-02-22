package v1

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/models"
	"net/http"
	"strconv"
	"strings"
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
// @Success     200 {object} []models.Recording
// @Failure     500 {} string "Error message"
// @Router      /recordings [get]
func GetRecordings(c *gin.Context) {
	appG := app.Gin{C: c}
	recordings, err := models.RecordingList()

	if err != nil {
		appG.Response(http.StatusInternalServerError, nil)
		return
	}

	appG.Response(http.StatusOK, recordings)
}

// GetRecording godoc
// @Summary     Return a list of recordings for a particular channel
// @Description Return a list of recordings for a particular channel.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       channelName path string true "Channel name"
// @Success     200 {object} []models.Recording
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

	recordings, err := models.FindByName(channelName)
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
// @Success     200 {object} []models.Recording
// @Failure     500 {} string "Error message"
// @Router      /recordings/bookmarks [get]
func GetBookmarks(c *gin.Context) {
	appG := app.Gin{C: c}
	recordings, err := models.BookmarkList()

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
// @Success     200 {object} models.Job
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{channelName}/{filename}/preview [post]
func GeneratePreview(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := strings.ToLower(strings.TrimSpace(c.Param("channelName")))
	filename := strings.ToLower(strings.TrimSpace(c.Param("filename")))

	if channelName == "" || filename == "" {
		appG.Response(http.StatusBadRequest, "invalid params")
		return
	}

	job, err := models.EnqueuePreviewJob(channelName, filename)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, job)
}

//func UpdateVideoInfo(c *gin.Context) {
//	appG := app.Gin{C: c}
//
//	log.Println("Starting updating video durations ...")
//	go service.UpdateVideoInfo()
//
//	appG.Response(http.StatusOK, nil)
//}

// Bookmark godoc
// @Summary     Bookmark a certain video in a channel
// @Description Bookmark a certain video in a channel.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       bookmark path int true "1 or 0 for bookmark or remove bookmark"
// @Param       channelName path string true "Channel name"
// @Param       filename    path string true "Filename to generate the preview for"
// @Success     200
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{channelName}/{filename}/bookmark/{bookmark} [post]
func Bookmark(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := strings.ToLower(strings.TrimSpace(c.Param("channelName")))
	filename := strings.ToLower(strings.TrimSpace(c.Param("filename")))
	bookmark, err := strconv.Atoi(c.Param("bookmark"))

	if err != nil || channelName == "" || filename == "" {
		appG.Response(http.StatusBadRequest, "invalid params")
		return
	}

	errUpdate := models.Db.Table("recordings").Where("channel_name = ? AND filename = ?", channelName, filename).Update("bookmark", bookmark).Error
	if errUpdate != nil {
		appG.Response(http.StatusInternalServerError, errUpdate)
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
// @Success     200 {object} models.Job
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/:channelName/:filename/cut [post]
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

	job, err := models.EnqueueCuttingJob(channelName, filename, conf.AbsoluteFilepath(channelName, filename), string(cut))
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, job)
}

// GetLatestRecordings godoc
// @Summary     Get the top N the latest recordings
// @Description Get the top N the latest recordings.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       limit path string int "How many recordings"
// @Success     200 {object} []models.Recording
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/latest/{limit} [get]
func GetLatestRecordings(c *gin.Context) {
	appG := app.Gin{C: c}

	limit, err := strconv.Atoi(c.Param("limit"))
	if err != nil {
		appG.Response(http.StatusBadRequest, err.Error())
		return
	}

	recordings, err := models.LatestList(limit)

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
// @Success     200 {object} []models.Recording
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

	recordings, err := models.FindRandom(limit)

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

	rec, err := models.FindRecording(channelName, filename)
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

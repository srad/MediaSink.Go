package v1

import (
	"net/http"
	"strconv"

	"github.com/srad/mediasink/helpers"
	"github.com/srad/mediasink/models/requests"

	"github.com/gin-gonic/gin"
	"github.com/srad/mediasink/app"
	"github.com/srad/mediasink/database"
	"github.com/srad/mediasink/services"
)

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
		appG.Error(http.StatusInternalServerError, nil)
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
		appG.Error(http.StatusInternalServerError, err)
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
	if err := services.UpdateVideoInfo(); err != nil {
		appG.Error(http.StatusInternalServerError, err)
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
	appG.Response(http.StatusOK, services.IsUpdatingRecordings())
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
		appG.Error(http.StatusBadRequest, err)
		return
	}

	recording, err := database.RecordingID(id).FindRecordingByID()
	if err != nil {
		appG.Error(http.StatusInternalServerError, err)
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
		appG.Error(http.StatusInternalServerError, nil)
		return
	}

	appG.Response(http.StatusOK, recordings)
}

// GeneratePreviews godoc
// @Summary     Generate preview for a certain video in a channel
// @Description Generate preview for a certain video in a channel.
// @Tags        recordings
// @Accept      json
// @Produce     json
// @Param       id path uint true "Recording item id"
// @Success     200 {object} []database.Job
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{id}/preview [post]
func GeneratePreviews(c *gin.Context) {
	appG := app.Gin{C: c}

	id, errConvert := strconv.ParseUint(c.Param("id"), 10, 32)
	if errConvert != nil {
		appG.Error(http.StatusBadRequest, errConvert)
		return
	}

	if recording, err := database.RecordingID(id).FindRecordingByID(); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	} else {
		if job1, job2, err := recording.EnqueuePreviewsJob(); err != nil {
			appG.Error(http.StatusInternalServerError, err)
			return
		} else {
			appG.Response(http.StatusOK, []*database.Job{job1, job2})
		}
	}
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
		appG.Error(http.StatusBadRequest, err)
		return
	}

	if err := database.FavRecording(uint(id), true); err != nil {
		appG.Error(http.StatusInternalServerError, err)
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
		appG.Error(http.StatusBadRequest, err)
		return
	}

	if err := database.FavRecording(uint(id), false); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

// CutRecording godoc
// @Summary     Cut a video and merge all defined segments
// @Description Cut a video and merge all defined segments
// @Tags        recordings
// @Param       id path uint true "Recording item id"
// @Param       CutRequest body requests.CutRequest true "Start and end timestamp of cutting sequences."
// @Accept      json
// @Produce     json
// @Success     200 {object} database.Job
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /recordings/{id}/cut [post]
func CutRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	cutRequest := &requests.CutRequest{}
	if err := c.BindJSON(cutRequest); err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	args := &helpers.CutArgs{
		Starts:                cutRequest.Starts,
		Ends:                  cutRequest.Ends,
		DeleteAfterCompletion: cutRequest.DeleteAfterCompletion,
	}
	if job, err := database.EnqueueCuttingJob(uint(id), args); err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	} else {
		appG.Response(http.StatusOK, job)
	}
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
		appG.Error(http.StatusBadRequest, err)
		return
	}
	mediaType := c.Param("mediaType")

	if rec, err := database.RecordingID(id).FindRecordingByID(); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	} else {
		if job, err := rec.EnqueueConversionJob(mediaType); err != nil {
			appG.Error(http.StatusInternalServerError, err)
			return
		} else {
			appG.Response(http.StatusOK, job)
		}
	}
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
		appG.Error(http.StatusBadRequest, err)
		return
	}

	column := c.Param("column")
	order := c.Param("order")

	recordings, err := database.SortBy(column, order, limit)
	if err != nil {
		appG.Error(http.StatusInternalServerError, err)
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
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	recordings, err := database.FindRandom(limit)

	if err != nil {
		appG.Error(http.StatusInternalServerError, err)
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
		appG.Error(http.StatusBadRequest, err)
		return
	}

	rec, err := database.RecordingID(id).FindRecordingByID()
	if err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	if rec != nil {
		if err2 := rec.DestroyRecording(); err2 != nil {
			appG.Error(http.StatusInternalServerError, err2)
			return
		}
	}

	appG.Response(http.StatusOK, nil)
}

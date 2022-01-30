package v1

import (
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/models"
	"github.com/srad/streamsink/services"
)

func GetRecordings(c *gin.Context) {
	appG := app.Gin{C: c}
	recordings, err := models.FindAll()

	if err != nil {
		appG.Response(http.StatusInternalServerError, nil)
		return
	}

	appG.Response(http.StatusOK, recordings)
}

func GetBookmarks(c *gin.Context) {
	appG := app.Gin{C: c}
	recordings, err := models.FindBookmarks()

	if err != nil {
		appG.Response(http.StatusInternalServerError, nil)
		return
	}

	appG.Response(http.StatusOK, recordings)
}

func GeneratePreview(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := strings.ToLower(strings.TrimSpace(c.Param("channelName")))
	filename := strings.ToLower(strings.TrimSpace(c.Param("filename")))

	if channelName == "" || filename == "" {
		appG.Response(http.StatusBadRequest, "invalid params")
		return
	}

	_, err := models.EnqueuePreviewJob(channelName, filename)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

func UpdateVideoInfo(c *gin.Context) {
	appG := app.Gin{C: c}

	log.Println("Starting updating video durations ...")
	go services.UpdateVideoInfo()

	appG.Response(http.StatusOK, nil)
}

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
		appG.Response(http.StatusBadRequest, errUpdate)
		return
	}

	appG.Response(http.StatusOK, nil)
}

func CutRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	body, err := ioutil.ReadAll(c.Request.Body)

	if err != nil {
		appG.Response(http.StatusBadRequest, "invalid body")
		return
	}

	intervals := string(body)

	channelName := strings.ToLower(strings.TrimSpace(c.Param("channelName")))
	filename := strings.ToLower(strings.TrimSpace(c.Param("filename")))

	if channelName == "" || filename == "" {
		appG.Response(http.StatusBadRequest, "invalid params")
		return
	}

	models.EnqueueCuttingJob(channelName, filename, conf.AbsoluteFilepath(channelName, filename), intervals)
}

func IsRecording(c *gin.Context) {
	appG := app.Gin{C: c}
	appG.Response(http.StatusOK, services.Recording())
}

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

func GetLatestRecordings(c *gin.Context) {
	appG := app.Gin{C: c}

	limit, err := strconv.Atoi(c.Param("limit"))
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	recordings, err := models.FindLatest(limit)

	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, recordings)
}

func PauseRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	go services.Pause()

	appG.Response(http.StatusOK, nil)
}

func ResumeRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	services.Resume()
	appG.Response(http.StatusOK, nil)
}

func DownloadRecording(c *gin.Context) {
	c.FileAttachment(conf.AbsoluteFilepath(c.Param("channelName"), c.Param("filename")), c.Param("filename"))
}

func DeleteRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := strings.ToLower(strings.TrimSpace(c.Param("channelName")))
	filename := strings.ToLower(strings.TrimSpace(c.Param("filename")))

	if channelName == "" || filename == "" {
		appG.Response(http.StatusBadRequest, "invalid params")
		return
	}
	if err := models.DeleteRecording(channelName, filename); err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, nil)
}

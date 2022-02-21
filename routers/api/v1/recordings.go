package v1

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/model"
	"github.com/srad/streamsink/service"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func GetRecordings(c *gin.Context) {
	appG := app.Gin{C: c}
	recordings, err := model.RecordingList()

	if err != nil {
		appG.Response(http.StatusInternalServerError, nil)
		return
	}

	appG.Response(http.StatusOK, recordings)
}

func GetRecording(c *gin.Context) {
	appG := app.Gin{C: c}
	channelName := c.Param("channelName")

	if channelName == "" {
		appG.Response(http.StatusBadRequest, nil)
		return
	}

	recordings, err := model.FindByName(channelName)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, &recordings)
}

func GetBookmarks(c *gin.Context) {
	appG := app.Gin{C: c}
	recordings, err := model.BookmarkList()

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

	job, err := model.EnqueuePreviewJob(channelName, filename)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, job)
}

func UpdateVideoInfo(c *gin.Context) {
	appG := app.Gin{C: c}

	log.Println("Starting updating video durations ...")
	go service.UpdateVideoInfo()

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

	errUpdate := model.Db.Table("recordings").Where("channel_name = ? AND filename = ?", channelName, filename).Update("bookmark", bookmark).Error
	if errUpdate != nil {
		appG.Response(http.StatusBadRequest, errUpdate)
		return
	}

	appG.Response(http.StatusOK, nil)
}

type CutRequest struct {
	Starts []string `json:"starts"`
	Ends   []string `json:"ends"`
}

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

	job, err := model.EnqueueCuttingJob(channelName, filename, conf.AbsoluteFilepath(channelName, filename), string(cut))
	if err != nil {
		appG.Response(http.StatusBadRequest, err.Error())
		return
	}

	appG.Response(http.StatusOK, job)
}

func IsRecording(c *gin.Context) {
	appG := app.Gin{C: c}
	appG.Response(http.StatusOK, service.IsRecording())
}

func GetLatestRecordings(c *gin.Context) {
	appG := app.Gin{C: c}

	limit, err := strconv.Atoi(c.Param("limit"))
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	recordings, err := model.LatestList(limit)

	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, recordings)
}

func GetRandomRecordings(c *gin.Context) {
	appG := app.Gin{C: c}

	limit, err := strconv.Atoi(c.Param("limit"))
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	recordings, err := model.FindRandom(limit)

	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, recordings)
}

func PauseRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	go service.Pause()

	appG.Response(http.StatusOK, nil)
}

func ResumeRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	service.Resume()
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

	rec, err := model.FindRecording(channelName, filename)
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

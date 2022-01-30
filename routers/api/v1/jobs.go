package v1

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/models"
)

func AddJob(c *gin.Context) {
	appG := app.Gin{C: c}

	channelName := strings.ToLower(strings.TrimSpace(c.Param("channelName")))
	filename := strings.ToLower(strings.TrimSpace(c.Param("filename")))

	if channelName == "" || filename == "" {
		appG.Response(http.StatusBadRequest, nil)
		return
	}

	job, err := models.EnqueuePreviewJob(channelName, filename)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	appG.Response(http.StatusOK, &job)
}

func GetJobs(c *gin.Context) {
	appG := app.Gin{C: c}
	jobs, err := models.GetJobs()

	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	appG.Response(http.StatusOK, &jobs)
}

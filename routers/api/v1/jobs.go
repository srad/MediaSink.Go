package v1

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/srad/streamsink/utils"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/models"
)

// AddJob godoc
// @Summary     Enqueue a preview job
// @Description Enqueue a preview job for a video in a channel. For now only preview jobs allowed via REST
// @Tags        jobs
// @Param       channelName path string  true  "Channel name"
// @Param       filename    path string  true  "Filename in channel"
// @Accept      json
// @Produce     json
// @Success     200 {object} models.Job
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /jobs/{channelName}/{filename} [post]
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
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, &job)
}

// StopJob godoc
// @Summary     Interrupt job gracefully
// @Description Interrupt job gracefully
// @Tags        jobs
// @Param       pid path int  true  "Channel name"
// @Accept      json
// @Produce     json
// @Success     200
// @Failure     400 {string} http.StatusBadRequest
// @Failure     500 {string} http.StatusInternalServerError
// @Router      /jobs/stop/{pid} [post]
func StopJob(c *gin.Context) {
	appG := app.Gin{C: c}

	pid, err := strconv.Atoi(c.Param("pid"))
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	if utils.Interrupt(pid); err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, pid)
}

// DestroyJob godoc
// @Summary     Interrupt and delete job gracefully
// @Description Interrupt and delete job gracefully
// @Tags        jobs
// @Param       id path int  true  "Job id"
// @Accept      json
// @Produce     json
// @Success     200
// @Failure     400 {string} http.StatusBadRequest
// @Failure     500 {string} http.StatusInternalServerError
// @Router      /jobs/{pid} [delete]
func DestroyJob(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	job, err := models.FindJobById(id)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	job.Destroy()

	appG.Response(http.StatusOK, id)
}

// GetJobs godoc
// @Summary     Return a list of jobs
// @Description Return a list of jobs
// @Tags        jobs
// @Accept      json
// @Produce     json
// @Success     200 {object} []models.Job
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /jobs [get]
func GetJobs(c *gin.Context) {
	appG := app.Gin{C: c}
	jobs, err := models.JobList()

	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, &jobs)
}

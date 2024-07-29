package v1

import (
	"github.com/srad/streamsink/helpers"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/models"
)

// AddJob godoc
// @Summary     Enqueue a preview job
// @Description Enqueue a preview job for a video in a channel. For now only preview jobs allowed via REST
// @Tags        jobs
// @Param       id path string  true  "Recording item id"
// @Accept      json
// @Produce     json
// @Success     200 {object} models.Job
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /jobs/{id} [post]
func AddJob(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Response(http.StatusBadRequest, err)
		return
	}

	recording := models.Recording{RecordingId: uint(id)}
	job, err := recording.EnqueuePreviewJob()
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
// @Param       pid path int  true  "Process ID"
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

	if err := helpers.Interrupt(pid); err != nil {
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
// @Router      /jobs/{id} [delete]
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

	if errDestroy := job.Destroy(); errDestroy != nil {
		appG.Response(http.StatusInternalServerError, errDestroy.Error())
		return
	}

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

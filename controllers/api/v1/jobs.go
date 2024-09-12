package v1

import (
	"fmt"
	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/database"
)

type JobResponse struct {
	Jobs       []*database.Job `json:"jobs"`
	TotalCount int64           `json:"totalCount"`
	Skip       int             `json:"skip"`
	Take       int             `json:"take"`
}

// AddJob godoc
// @Summary     Enqueue a preview job
// @Description Enqueue a preview job for a video in a channel. For now only preview jobs allowed via REST
// @Tags        jobs
// @Param       id path string  true  "Recording item id"
// @Accept      json
// @Produce     json
// @Success     200 {object} database.Job
// @Failure     400 {} http.StatusBadRequest
// @Failure     500 {} http.StatusInternalServerError
// @Router      /jobs/{id} [post]
func AddJob(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	recording, err := database.RecordingId(id).FindRecordingById()
	if err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	job, err := services.EnqueuePreviewJob(recording.RecordingId)
	if err != nil {
		appG.Error(http.StatusInternalServerError, err)
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
		appG.Error(http.StatusBadRequest, err)
		return
	}

	if err := helpers.Interrupt(pid); err != nil {
		appG.Error(http.StatusInternalServerError, err)
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
// @Success     200 {} http.StatusOK
// @Failure     400 {string} http.StatusBadRequest
// @Failure     500 {string} http.StatusInternalServerError
// @Router      /jobs/{id} [delete]
func DestroyJob(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	if err := services.DeleteJob(uint(id)); err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

// GetJobs godoc
// @Summary     Return a list of jobs
// @Description Return a list of jobs
// @Tags        jobs
// @Accept      json
// @Produce     json
// @Param       skip path int true "Number of rows to skip"
// @Param       take path int true "Number of rows to take"
// @Success     200 {object} JobResponse
// @Failure     500 {} string http.StatusInternalServerError
// @Router      /jobs/{skip}/{take} [get]
func GetJobs(c *gin.Context) {
	appG := app.Gin{C: c}

	skip, skipErr := strconv.ParseInt(c.Param("skip"), 10, 32)
	take, takeErr := strconv.ParseInt(c.Param("take"), 10, 32)

	if skipErr != nil {
		appG.Error(http.StatusBadRequest, fmt.Errorf("invalid id type: %s", skipErr))
		return
	}

	if takeErr != nil {
		appG.Error(http.StatusBadRequest, fmt.Errorf("invalid id type: %s", takeErr))
		return
	}

	if jobs, totalCount, err := database.JobList(int(skip), int(take)); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	} else {
		appG.Response(http.StatusOK, JobResponse{
			Jobs:       jobs,
			TotalCount: totalCount,
			Skip:       int(skip),
			Take:       int(take),
		})
	}
}

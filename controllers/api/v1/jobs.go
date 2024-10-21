package v1

import (
	"net/http"
	"strconv"

	"github.com/srad/streamsink/helpers"
	"github.com/srad/streamsink/models/requests"
	"github.com/srad/streamsink/models/responses"
	"github.com/srad/streamsink/services"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/database"
)

// AddPreviewJobs godoc
// @Summary     Enqueue a preview job
// @Description Enqueue a preview job for a video in a channel. For now only preview jobs allowed via REST
// @Tags        jobs
// @Param       id path string  true  "Recording item id"
// @Accept      json
// @Produce     json
// @Success     200 {object} []database.Job
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /jobs/{id} [post]
func AddPreviewJobs(c *gin.Context) {
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

	job1, job2, err := recording.EnqueuePreviewsJob()
	if err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, []*database.Job{job1, job2})
}

// StopJob godoc
// @Summary     Interrupt job gracefully
// @Description Interrupt job gracefully
// @Tags        jobs
// @Param       pid path int  true  "Process ID"
// @Accept      json
// @Produce     json
// @Success     200
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
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
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /jobs/{id} [delete]
func DestroyJob(c *gin.Context) {
	appG := app.Gin{C: c}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	if err := services.DeleteJob(uint(id)); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, nil)
}

// JobsList godoc
// @Summary     Jobs pagination
// @Description Allow paging through jobs by providing skip, take, statuses, and sort order.
// @Tags        jobs
// @Accept      json
// @Produce     json
// @Param       JobsRequest body requests.JobsRequest true "Job pagination properties"
// @Success     200 {object} responses.JobsResponse
// @Failure     400 {} string "Error message"
// @Failure     500 {} string "Error message"
// @Router      /jobs/list [post]
func JobsList(c *gin.Context) {
	appG := app.Gin{C: c}

	var request requests.JobsRequest

	if err := c.BindJSON(&request); err != nil {
		appG.Error(http.StatusBadRequest, err)
		return
	}

	if jobs, totalCount, err := database.JobList(request.Skip, request.Take, request.States, request.SortOrder); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	} else {
		appG.Response(http.StatusOK, responses.JobsResponse{
			Jobs:       jobs,
			TotalCount: totalCount,
			Skip:       request.Skip,
			Take:       request.Take,
		})
	}
}

// PauseJobs godoc
// @Summary     Stops the job processing
// @Description Stops the job processing
// @Tags        jobs
// @Success     200 {} nil
// @Router      /jobs/pause [post]
func PauseJobs(c *gin.Context) {
	appG := app.Gin{C: c}
	services.StopJobProcessing()
	appG.Response(http.StatusOK, nil)
}

// ResumeJobs godoc
// @Summary     Start the job processing
// @Description Start the job processing
// @Tags        jobs
// @Success     200 {} nil
// @Router      /jobs/resume [post]
func ResumeJobs(c *gin.Context) {
	appG := app.Gin{C: c}
	services.StartJobProcessing()
	appG.Response(http.StatusOK, nil)
}

// IsProcessing godoc
// @Summary     Job worker status
// @Description Job worker status
// @Produce     json
// @Success     200 {object} responses.JobWorkerStatus
// @Tags        jobs
// @Router      /jobs/worker [get]
func IsProcessing(c *gin.Context) {
	appG := app.Gin{C: c}
	appG.Response(http.StatusOK, &responses.JobWorkerStatus{IsProcessing: services.IsJobProcessing()})
}

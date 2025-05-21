package v1

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/srad/mediasink/models/responses"

	"github.com/gin-gonic/gin"
	"github.com/srad/mediasink/app"
	"github.com/srad/mediasink/services"
)

// IsRecording godoc
// @Summary     Return if server is current recording
// @Description Return if server is current recording.
// @Tags        recorder
// @Produce     json
// @Success     200 {object} responses.RecordingStatusResponse
// @Success     500
// @Router      /recorder [get]
func IsRecording(c *gin.Context) {
	appG := app.Gin{C: c}
	appG.Response(http.StatusOK, &responses.RecordingStatusResponse{IsRecording: services.IsRecorderActive()})
}

// StopRecorder godoc
// @Summary     StopRecorder server recording
// @Tags        recorder
// @Success     200
// @Router      /recorder/pause [post]
func StopRecorder(c *gin.Context) {
	appG := app.Gin{C: c}

	go services.StopRecorder()

	appG.Response(http.StatusOK, nil)
}

// StartRecorder godoc
// @Summary     StartRecorder server recording
// @Tags        recorder
// @Success     200
// @Router      /recorder/resume [post]
func StartRecorder(c *gin.Context) {
	appG := app.Gin{C: c}

	log.Infoln("Resuming recorder")
	services.StartRecorder()
	appG.Response(http.StatusOK, nil)
}

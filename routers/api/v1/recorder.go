package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/services"
	"net/http"
)

// IsRecording godoc
// @Summary     Return if server is current recording
// @Description Return if server is current recording.
// @Tags        recorder
// @Produce     plain
// @Success     200 {} bool "true if currently recording, otherwise false."
// @Router      /recorder [get]
func IsRecording(c *gin.Context) {
	appG := app.Gin{C: c}
	appG.Response(http.StatusOK, services.IsRecording())
}

// PauseRecorder godoc
// @Summary     Pause server recording
// @Tags        recorder
// @Success     200
// @Router      /recorder/pause [post]
func PauseRecorder(c *gin.Context) {
	appG := app.Gin{C: c}

	go services.Pause()

	appG.Response(http.StatusOK, nil)
}

// ResumeRecording godoc
// @Summary     Resume server recording
// @Tags        recorder
// @Success     200
// @Router      /recorder/resume [post]
func ResumeRecording(c *gin.Context) {
	appG := app.Gin{C: c}

	services.Resume()
	appG.Response(http.StatusOK, nil)
}

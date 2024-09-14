package v1

import (
	"github.com/srad/streamsink/models/responses"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/services"
)

// TriggerImport godoc
// @Summary     Run once the import of mp4 files in the recordings folder, which are not yet in the system
// @Schemes
// @Description Return a list of channels
// @Tags        admin
// @Accept      json
// @Produce     json
// @Success     200
// @Failure     500 {} http.StatusInternalServerError
// @Router      /admin/import [post]
func TriggerImport(c *gin.Context) {
	appG := app.Gin{C: c}

	services.StopImport()
	services.StartImport()

	appG.Response(http.StatusOK, nil)
}

// GetImportInfo godoc
// @Summary     Returns server version information
// @Schemes
// @Description version information
// @Tags        admin
// @Accept      json
// @Produce     json
// @Success     200 {object} responses.ImportInfoResponse
// @Failure     500 {} http.StatusInternalServerError
// @Router      /admin/import [get]
func GetImportInfo(c *gin.Context) {
	appG := app.Gin{C: c}

	progress, size := services.GetImportProgress()

	info := responses.ImportInfoResponse{
		IsImporting: services.IsImporting(),
		Progress:    progress,
		Size:        size,
	}

	appG.Response(http.StatusOK, info)
}

// GetVersion godoc
// @Summary     Returns server version information
// @Schemes
// @Description version information
// @Tags        admin
// @Accept      json
// @Produce     json
// @Success     200 {object} responses.ServerInfoResponse
// @Failure     500 {} http.StatusInternalServerError
// @Router      /admin/version [get]
func GetVersion(version, commit string) func(c *gin.Context) {
	return func(c *gin.Context) {
		appG := app.Gin{C: c}

		appG.Response(http.StatusOK, responses.ServerInfoResponse{Commit: commit, Version: version})
	}
}

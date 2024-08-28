package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/database"
)

// GetCpu godoc
// @Summary     Get CPU metrics
// @Description Get CPU metrics
// @Tags        metric
// @Accept      json
// @Produce     json
// @Success     200 {object} database.CPULoad
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /metric/cpu [get]
func GetCpu(c *gin.Context) {
	appG := app.Gin{C: c}
	if response, err := database.GetCpuMeasure(); err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	} else {
		appG.Response(http.StatusOK, &response)
	}
}

// GetNet godoc
// @Summary     Get network metrics
// @Description Get network metrics
// @Tags        metric
// @Accept      json
// @Produce     json
// @Success     200 {object} database.NetInfo
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /metric/net [get]
func GetNet(c *gin.Context) {
	appG := app.Gin{C: c}
	response, err := database.GetNetworkMeasure()
	if err != nil {
		appG.Error(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, &response)
}

package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/models"
	"net/http"
)

// GetCpu godoc
// @Summary     Get CPU metrics
// @Description Get CPU metrics
// @Tags        metric
// @Accept      json
// @Produce     json
// @Success     200 {object} models.CPULoad
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /metric/cpu [get]
func GetCpu(c *gin.Context) {
	appG := app.Gin{C: c}
	response, err := models.GetCpuMeasure()
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, &response)
}

// GetNet godoc
// @Summary     Get network metrics
// @Description Get network metrics
// @Tags        metric
// @Accept      json
// @Produce     json
// @Success     200 {object} models.NetInfo
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /metric/net [get]
func GetNet(c *gin.Context) {
	appG := app.Gin{C: c}
	response, err := models.GetNetworkMeasure()
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, &response)
}

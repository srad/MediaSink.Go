package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/model"
	"net/http"
)

func GetCpu(c *gin.Context) {
	appG := app.Gin{C: c}
	response, err := model.GetCpuMeasure()
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, &response)
}

func GetNet(c *gin.Context) {
	appG := app.Gin{C: c}
	response, err := model.GetNetworkMeasure()
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, &response)
}

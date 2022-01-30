package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/utils"
)

func GetInfo(c *gin.Context) {
	appG := app.Gin{C: c}

	secs := c.Param("seconds")
	val, err := strconv.ParseUint(secs, 10, 64)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
	}

	data, err := utils.Info(conf.AppCfg.DataDisk, conf.AppCfg.NetworkDev, val)

	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, data)
}

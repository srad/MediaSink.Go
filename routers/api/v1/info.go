package v1

import (
	"github.com/srad/streamsink/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/conf"
)

func GetInfo(c *gin.Context) {
	appG := app.Gin{C: c}

	secs := c.Param("seconds")
	val, err := strconv.ParseUint(secs, 10, 64)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
	}

	data, err := model.Info(conf.AppCfg.DataDisk, conf.AppCfg.NetworkDev, val)

	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, data)
}

func GetDiskInfo(c *gin.Context) {
	appG := app.Gin{C: c}

	info, err := model.DiskUsage(conf.AppCfg.DataDisk)

	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, info)
}

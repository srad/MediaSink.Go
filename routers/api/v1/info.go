package v1

import (
	"net/http"
	"strconv"

	"github.com/srad/streamsink/utils"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/app"
	"github.com/srad/streamsink/conf"
)

// GetInfo godoc
// @Summary     Get system metrics
// @Description Get system metrics
// @Tags        info
// @Accept      json
// @Produce     json
// @Param       seconds path int true "Number of seconds to measure"
// @Success     200 {object} utils.SysInfo
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /info/{seconds} [get]
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

// GetDiskInfo godoc
// @Summary     Get disk information
// @Description Get disk information
// @Tags        info
// @Accept      json
// @Produce     json
// @Success     200 {object} utils.DiskInfo
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /info/disk [get]
func GetDiskInfo(c *gin.Context) {
	appG := app.Gin{C: c}

	info, err := utils.DiskUsage(conf.AppCfg.DataDisk)

	if err != nil {
		appG.Response(http.StatusInternalServerError, err.Error())
		return
	}

	appG.Response(http.StatusOK, info)
}

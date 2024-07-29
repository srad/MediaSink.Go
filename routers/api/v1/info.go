package v1

import (
	"net/http"
	"strconv"

	"github.com/srad/streamsink/helpers"

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
// @Success     200 {object} helpers.SysInfo
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /info/{seconds} [get]
func GetInfo(c *gin.Context) {
	appG := app.Gin{C: c}
	cfg := conf.Read()

	secs := c.Param("seconds")
	val, err := strconv.ParseUint(secs, 10, 64)
	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
	}

	data, err := helpers.Info(cfg.DataDisk, cfg.NetworkDev, val)

	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
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
// @Success     200 {object} helpers.DiskInfo
// @Failure     500 {}  http.StatusInternalServerError
// @Router      /info/disk [get]
func GetDiskInfo(c *gin.Context) {
	appG := app.Gin{C: c}

	cfg := conf.Read()

	info, err := helpers.DiskUsage(cfg.DataDisk)

	if err != nil {
		appG.Response(http.StatusInternalServerError, err)
		return
	}

	appG.Response(http.StatusOK, info)
}

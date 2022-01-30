package app

import (
	"github.com/gin-gonic/gin"
)

type Gin struct {
	C *gin.Context
}

// Response setting gin.JSON
func (g *Gin) Response(httpCode int, data interface{}) {
	g.C.JSON(httpCode, data)
	return
}

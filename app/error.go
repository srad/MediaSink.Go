package app

// Response setting gin.JSON
func (g *Gin) Error(httpCode int, err error) {
	g.C.AbortWithStatusJSON(httpCode, err.Error())
}

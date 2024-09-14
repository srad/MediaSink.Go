package app

func (g *Gin) Error(httpCode int, err error) {
	g.C.AbortWithStatusJSON(httpCode, err.Error())
}

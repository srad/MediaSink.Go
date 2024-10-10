package app

import (
	log "github.com/sirupsen/logrus"
)

func (g *Gin) Error(httpCode int, err error) {
	g.C.AbortWithStatusJSON(httpCode, err.Error())
	log.Errorln(err)
}

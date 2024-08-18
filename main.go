package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/models"
	"github.com/srad/streamsink/routers"
	"github.com/srad/streamsink/services"
)

var (
	Version string
	Commit  string
)

func main() {
	log.Infof("Version: %s, Commit: %s", Version, Commit)

	log.SetFormatter(&log.TextFormatter{})

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cleanup()
		os.Exit(1)
	}()

	models.Init()
	// models.StartMetrics(conf.AppCfg.NetworkDev)
	setupFolders()

	services.StartUpJobs()
	services.StartRecorder()
	models.StartWorker()

	gin.SetMode("release")
	endPoint := fmt.Sprintf("0.0.0.0:%d", 3000)

	log.Infof("[main] start http server listening %s", endPoint)

	server := &http.Server{
		Addr:           endPoint,
		Handler:        routers.Setup(Version, Commit),
		ReadTimeout:    12 * time.Hour,
		WriteTimeout:   12 * time.Hour,
		MaxHeaderBytes: 0,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Fatalln(err)
		} else {
			log.Infof("[main] start http server listening %s", endPoint)
		}
	}()

	<-c
}

func cleanup() {
	log.Infoln("cleanup ...")
	models.StopWorker()
	services.StopRecorder()
	log.Infoln("cleanup complete")
}

func setupFolders() {
	channels, err := models.ChannelList()
	if err != nil {
		log.Errorln(err)
		return
	}
	for _, channel := range channels {
		channel.ChannelName.MkDir()
	}
}

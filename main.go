package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/models"
	"github.com/srad/streamsink/routers"
	"github.com/srad/streamsink/services"
	"github.com/srad/streamsink/workers"
)

func main() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cleanup()
		os.Exit(1)
	}()

	conf.Read()
	models.Init()
	// model.StartMetrics(conf.AppCfg.NetworkDev)
	setupFolders()

	services.StartDispatch()
	services.StartUpJobs()
	services.StartRecorder()
	go workers.StartWorker()

	gin.SetMode("release")
	endPoint := fmt.Sprintf("0.0.0.0:%d", 3000)

	log.Printf("[info] start http server listening %s", endPoint)

	server := &http.Server{
		Addr:           endPoint,
		Handler:        routers.Setup(),
		ReadTimeout:    12 * time.Hour,
		WriteTimeout:   12 * time.Hour,
		MaxHeaderBytes: 0,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Fatalln(err)
		} else {
			log.Printf("[info] start http server listening %s", endPoint)
		}
	}()

	<-c
}

func cleanup() {
	log.Println("cleanup ...")
	workers.StopWorker()
	services.StopRecorder()
	services.StopDispatch()
	log.Println("cleanup complete")
}

func setupFolders() {
	channels, err := models.ChannelList()
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, channel := range channels {
		conf.MakeChannelFolders(channel.ChannelName)
	}
}

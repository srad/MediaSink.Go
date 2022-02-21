package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/model"
	"github.com/srad/streamsink/routers"
	v1 "github.com/srad/streamsink/routers/api/v1"
	"github.com/srad/streamsink/service"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
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
	model.Init()
	//model.StartMetrics(conf.AppCfg.NetworkDev)
	setupFolders()

	service.Subscribe(func(message service.SocketMessage) {
		v1.SendMessage(v1.SocketMessage{Event: message.Event, Data: message.Data})
	})
	model.Subscribe(func(message model.SocketMessage) {
		v1.SendMessage(v1.SocketMessage{Event: message.Event, Data: message.Data})
	})

	service.Resume()

	go service.ImportRecordings()
	go service.FixOrphanedRecordings()
	go model.StartWorker()

	gin.SetMode("release")
	endPoint := fmt.Sprintf("0.0.0.0:%d", 3000)

	log.Printf("[info] start http server listening %s", endPoint)

	server := &http.Server{
		Addr:         endPoint,
		Handler:      routers.Setup(),
		ReadTimeout:  12 * time.Hour,
		WriteTimeout: 12 * time.Hour,
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
	model.StopWorker()
	service.Pause()
	log.Println("cleanup complete")
}

func setupFolders() {
	channels, err := model.ChannelList()
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, channel := range channels {
		conf.MakeChannelFolders(channel.ChannelName)
	}
}

package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/models"
	"github.com/srad/streamsink/routers"
	"github.com/srad/streamsink/services"
	"github.com/srad/streamsink/utils"
	"github.com/srad/streamsink/workers"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	//"github.com/srad/streamsink/workers"
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
	go services.ImportRecordings()
	go services.FixOrphanedRecordings()
	go workers.JobWorker()

	gin.SetMode("release")
	endPoint := fmt.Sprintf("0.0.0.0:%d", 3000)
	setupFolders()
	services.Resume()

	log.Printf("[info] start http server listening %s", endPoint)

	go utils.TCPServer()

	server := &http.Server{
		Addr:         endPoint,
		Handler:      routers.Setup(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
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
	workers.Quit <- true
	services.StopAll()
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

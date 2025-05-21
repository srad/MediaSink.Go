package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/srad/mediasink/controllers"
	"github.com/srad/mediasink/database"
	"github.com/srad/mediasink/services"
)

var (
	Version string
	Commit  string
)

func init() {
	// 1. Check for JWT SECRET
	if os.Getenv("SECRET") == "" {
		log.Fatal("FATAL: JWT SECRET environment variable is not set.")
	}
	log.Infoln("OK: JWT SECRET environment variable is set.")

	// 2. Check if needed executable exist
	executables := []string{"ffmpeg", "yt-dlp", "ffprobe"}
	for _, app := range executables {
		path, err := exec.LookPath(app)
		if err != nil {
			log.Fatalf("FATAL: Required executable '%s' not found in PATH: %v", app, err)
		}
		log.Infof("OK: Found executable '%s' at '%s'", app, path)
	}

	log.Infoln("All init checks passed.")
}

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

	database.Init()
	// models.StartMetrics(conf.AppCfg.NetworkDev)
	setupFolders()

	services.StartUpJobs()
	services.StartRecorder()
	services.StartJobProcessing()

	gin.SetMode("release")
	endPoint := fmt.Sprintf("0.0.0.0:%d", 3000)

	log.Infof("[main] start http server listening %s", endPoint)

	server := &http.Server{
		Addr:           endPoint,
		Handler:        controllers.Setup(Version, Commit),
		ReadTimeout:    12 * time.Hour,
		WriteTimeout:   12 * time.Hour,
		MaxHeaderBytes: 0,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Fatalln(err)
		}
		log.Infof("[main] start http server listening %s", endPoint)
	}()

	<-c
}

func cleanup() {
	log.Infoln("cleanup ...")
	services.StopJobProcessing()
	services.StopRecorder()
	log.Infoln("cleanup complete")
}

func setupFolders() {
	channels, err := database.ChannelList()
	if err != nil {
		log.Errorln(err)
		return
	}
	for _, channel := range channels {
		if err := channel.ChannelName.MkDir(); err != nil {
			log.Errorln(err)
		}
	}
}

package routers

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
	"github.com/srad/streamsink/conf"
	v1 "github.com/srad/streamsink/routers/api/v1"

	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"
)

var (
	server   *socketio.Server
	upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		}}
)

// Setup InitRouter initialize routing information
func Setup() http.Handler {
	r := gin.New()
	//r.Use(gin.Logger())
	r.Use(gin.Recovery())
	const rec = "./recordings"
	r.Static("/recordings", conf.AppCfg.RecordingsAbsolutePath)
	r.Static("/public", conf.AppCfg.PublicPath)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"POST", "GET", "DELETE", "PUT", "PATCH"},
		AllowHeaders:     []string{"Origin", "Content-Type, Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		AllowWebSockets:  true,
		AllowWildcard:    true,
	}))

	apiv1 := r.Group("/api/v1")
	apiv1.Use()
	{
		apiv1.GET("/channels", v1.GetChannels)
		apiv1.POST("/channels", v1.AddChannel)
		apiv1.DELETE("/channels/:channelName", v1.DeleteChannel)
		apiv1.POST("/channels/:channelName/resume", v1.ResumeChannel)
		apiv1.POST("/channels/:channelName/pause", v1.PauseChannel)

		apiv1.POST("/jobs/:channelName/:filename", v1.AddJob)
		apiv1.GET("/jobs", v1.GetJobs)

		apiv1.POST("/recordings/updateinfo", v1.UpdateVideoInfo)
		apiv1.POST("/recordings/:channelName/:filename/bookmark/:bookmark", v1.Bookmark)
		apiv1.POST("/recordings/:channelName/:filename/cut", v1.CutRecording)
		apiv1.GET("/recording", v1.IsRecording)
		apiv1.GET("/recordings/latest/:limit", v1.GetLatestRecordings)
		apiv1.GET("/recordings/random/:limit", v1.GetRandomRecordings)
		apiv1.GET("/recordings", v1.GetRecordings)
		apiv1.GET("/recordings/bookmarks", v1.GetBookmarks)
		apiv1.POST("/recordings/:channelName/:filename/preview", v1.GeneratePreview)
		apiv1.DELETE("/recordings/:channelName/:filename", v1.DeleteRecording)
		apiv1.GET("/recordings/:channelName/:filename/download", v1.DownloadRecording)
		apiv1.GET("/recordings/:channelName", v1.GetRecording)
		apiv1.POST("/recordings/resume", v1.ResumeRecording)
		apiv1.POST("/recordings/pause", v1.PauseRecording)

		apiv1.GET("/info/:seconds", v1.GetInfo)

		apiv1.GET("/ws", wshandler)
	}

	return r
}

func wshandler(c *gin.Context) {
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)

	if err != nil {
		log.Println("error get connection")
		log.Fatal(err)
		return
	}

	_, message, err := ws.ReadMessage()
	if err != nil {
		log.Println("error read message")
		log.Fatal(err)
		return
	}
	log.Printf("[Socket] %s", message)
}

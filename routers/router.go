package routers

import (
	"net/http"
	"time"

	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/docs"
	"github.com/srad/streamsink/network"

	"github.com/gin-contrib/cors"
	socketio "github.com/googollee/go-socket.io"
	v1 "github.com/srad/streamsink/routers/api/v1"

	"github.com/gin-gonic/gin"
	"github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           StreamSink API
// @version         1.0
// @description     The rest API of the StreamSink server.
//
// @contact.name   API Support
// @contact.url    https://github.com/srad
//
// @license.name  Dual license, non-commercial, but free for open-source educational uses.
//
// @host      localhost:3000
// @BasePath  /api/v1

var (
	server *socketio.Server
)

// Setup InitRouter initialize routing information
func Setup() http.Handler {
	r := gin.New()
	// r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// You can use the internal static path, but it is recommended that you use a seperate
	// nginx instance or container to serve the static content more efficiently.
	// This is more suited for dev environments.
	r.Static("/recordings", conf.AppCfg.RecordingsAbsolutePath)
	// r.Static("/public", conf.AppCfg.PublicPath)

	docs.SwaggerInfo.BasePath = "/api/v1"
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
	// apiv1 := r.Group("/api/v1", gin.BasicAuth(gin.Accounts{
	//	"user": "barcode",
	// }))
	apiv1.Use()
	{
		// Admin
		apiv1.POST("/admin/import", v1.TriggerImport)
		apiv1.GET("/admin/importing", v1.IsImporting)

		// Channels
		apiv1.GET("/channels", v1.GetChannels)
		apiv1.POST("/channels", v1.CreateChannel)

		apiv1.GET("/channels/:channelName", v1.GetChannel)
		apiv1.DELETE("/channels/:channelName", v1.DeleteChannel)
		apiv1.PATCH("/channels/:channelName", v1.UpdateChannel)

		apiv1.POST("/channels/:channelName/resume", v1.ResumeChannel)
		apiv1.POST("/channels/:channelName/pause", v1.PauseChannel)

		apiv1.POST("/channels/:channelName/fav", v1.FavChannel)
		apiv1.POST("/channels/:channelName/unfav", v1.UnFavChannel)

		apiv1.POST("/channels/:channelName/upload", v1.UploadChannel)

		apiv1.POST("/channels/:channelName/tags", v1.TagChannel)

		// Jobs
		apiv1.POST("/jobs/:channelName/:filename", v1.AddJob)
		apiv1.POST("/jobs/stop/:pid", v1.StopJob)
		apiv1.DELETE("/jobs/:id", v1.DestroyJob)
		apiv1.GET("/jobs", v1.GetJobs)

		// recorder
		apiv1.POST("/recorder/resume", v1.StartRecorder)
		apiv1.POST("/recorder/pause", v1.StopRecorder)
		apiv1.GET("/recorder", v1.IsRecording)

		// Channels
		apiv1.POST("/recordings/updateinfo", v1.UpdateVideoInfo)
		apiv1.POST("/recordings/isupdating", v1.IsUpdatingVideoInfo)
		apiv1.POST("/recordings/generate/posters", v1.GeneratePosters)

		// recordings
		apiv1.GET("/recordings", v1.GetRecordings)
		apiv1.GET("/recordings/filter/:column/:order/:limit", v1.FilterRecordings)
		apiv1.GET("/recordings/random/:limit", v1.GetRandomRecordings)
		apiv1.GET("/recordings/bookmarks", v1.GetBookmarks)
		apiv1.GET("/recordings/:channelName", v1.GetRecording)
		apiv1.GET("/recordings/:channelName/:filename/download", v1.DownloadRecording)

		apiv1.POST("/recordings/:channelName/:filename/fav", v1.FavRecording)
		apiv1.POST("/recordings/:channelName/:filename/unfav", v1.UnfavRecording)

		apiv1.POST("/recordings/:channelName/:filename/:mediaType/convert", v1.Convert)
		apiv1.POST("/recordings/:channelName/:filename/cut", v1.CutRecording)
		apiv1.POST("/recordings/:channelName/:filename/preview", v1.GeneratePreview)

		apiv1.DELETE("/recordings/:channelName/:filename", v1.DeleteRecording)

		apiv1.GET("/info/:seconds", v1.GetInfo)
		apiv1.GET("/info/disk", v1.GetDiskInfo)

		apiv1.GET("/metric/cpu", v1.GetCpu)
		apiv1.GET("/metric/net", v1.GetNet)

		go network.WsListen()
		apiv1.GET("/ws", network.WsHandler)
	}

	return r
}

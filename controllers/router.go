package controllers

import (
	"github.com/srad/streamsink/middlewares"
	"net/http"
	"time"

	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/docs"
	"github.com/srad/streamsink/network"

	"github.com/gin-contrib/cors"
	v1 "github.com/srad/streamsink/controllers/api/v1"

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

// Setup InitRouter initialize routing information
func Setup(version, commit string) http.Handler {
	router := gin.New()
	// r.Use(gin.Logger())
	router.Use(gin.Recovery())

	cfg := conf.Read()

	// This is only for development. User nginx or something to serve the static files.
	router.Static("/recordings", cfg.RecordingsAbsolutePath)

	docs.SwaggerInfo.BasePath = "/api/v1"
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		AllowHeaders:     []string{"*", "Authorization"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		AllowWebSockets:  true,
		AllowWildcard:    true,
	}))

	apiV1 := router.Group("/api/v1")

	apiV1.Use()
	{
		// Auth
		apiV1.POST("/auth/signup", v1.CreateUser)
		apiV1.POST("/auth/login", v1.Login)
		apiV1.GET("/user/profile", middlewares.CheckAuthorizationHeader, v1.GetUserProfile)

		// Admin
		apiV1.GET("/admin/version", middlewares.CheckAuthorizationHeader, v1.GetVersion(version, commit))
		apiV1.POST("/admin/import", middlewares.CheckAuthorizationHeader, v1.TriggerImport)
		apiV1.GET("/admin/import", middlewares.CheckAuthorizationHeader, v1.GetImportInfo)

		// Channels
		apiV1.GET("/channels", middlewares.CheckAuthorizationHeader, v1.GetChannels)
		apiV1.POST("/channels", middlewares.CheckAuthorizationHeader, v1.CreateChannel)

		apiV1.GET("/channels/:id", middlewares.CheckAuthorizationHeader, v1.GetChannel)
		apiV1.DELETE("/channels/:id", middlewares.CheckAuthorizationHeader, v1.DeleteChannel)
		apiV1.PATCH("/channels/:id", middlewares.CheckAuthorizationHeader, v1.UpdateChannel)

		apiV1.POST("/channels/:id/resume", middlewares.CheckAuthorizationHeader, v1.ResumeChannel)
		apiV1.POST("/channels/:id/pause", middlewares.CheckAuthorizationHeader, v1.PauseChannel)

		apiV1.PATCH("/channels/:id/fav", middlewares.CheckAuthorizationHeader, v1.FavChannel)
		apiV1.PATCH("/channels/:id/unfav", middlewares.CheckAuthorizationHeader, v1.UnFavChannel)

		apiV1.POST("/channels/:id/upload", middlewares.CheckAuthorizationHeader, v1.UploadChannel)

		apiV1.PATCH("/channels/:id/tags", middlewares.CheckAuthorizationHeader, v1.TagChannel)

		// Jobs
		apiV1.POST("/jobs/:id", middlewares.CheckAuthorizationHeader, v1.AddJob)
		apiV1.POST("/jobs/stop/:pid", middlewares.CheckAuthorizationHeader, v1.StopJob)
		apiV1.DELETE("/jobs/:id", middlewares.CheckAuthorizationHeader, v1.DestroyJob)
		apiV1.POST("/jobs/list", middlewares.CheckAuthorizationHeader, v1.JobsList)

		// recorder
		apiV1.POST("/recorder/resume", middlewares.CheckAuthorizationHeader, v1.StartRecorder)
		apiV1.POST("/recorder/pause", middlewares.CheckAuthorizationHeader, v1.StopRecorder)
		apiV1.GET("/recorder", middlewares.CheckAuthorizationHeader, v1.IsRecording)

		// Channels
		apiV1.POST("/recordings/updateinfo", middlewares.CheckAuthorizationHeader, v1.UpdateVideoInfo)
		apiV1.POST("/recordings/isupdating", middlewares.CheckAuthorizationHeader, v1.IsUpdatingVideoInfo)
		apiV1.POST("/recordings/generate/posters", middlewares.CheckAuthorizationHeader, v1.GeneratePosters)

		// recordings
		apiV1.GET("/recordings", middlewares.CheckAuthorizationHeader, v1.GetRecordings)
		apiV1.GET("/recordings/filter/:column/:order/:limit", middlewares.CheckAuthorizationHeader, v1.FilterRecordings)
		apiV1.GET("/recordings/random/:limit", middlewares.CheckAuthorizationHeader, v1.GetRandomRecordings)
		apiV1.GET("/recordings/bookmarks", middlewares.CheckAuthorizationHeader, v1.GetBookmarks)
		apiV1.GET("/recordings/:id", middlewares.CheckAuthorizationHeader, v1.GetRecording)
		apiV1.GET("/recordings/:id/download", middlewares.CheckAuthorizationHeader, v1.DownloadRecording)

		apiV1.PATCH("/recordings/:id/fav", middlewares.CheckAuthorizationHeader, v1.FavRecording)
		apiV1.PATCH("/recordings/:id/unfav", middlewares.CheckAuthorizationHeader, v1.UnfavRecording)

		apiV1.POST("/recordings/:id/:mediaType/convert", middlewares.CheckAuthorizationHeader, v1.Convert)
		apiV1.POST("/recordings/:id/cut", middlewares.CheckAuthorizationHeader, v1.CutRecording)
		apiV1.POST("/recordings/:id/preview", middlewares.CheckAuthorizationHeader, v1.GeneratePreview)

		apiV1.DELETE("/recordings/:id", middlewares.CheckAuthorizationHeader, v1.DeleteRecording)

		apiV1.GET("/info/:seconds", middlewares.CheckAuthorizationHeader, v1.GetInfo)
		apiV1.GET("/info/disk", middlewares.CheckAuthorizationHeader, v1.GetDiskInfo)

		apiV1.GET("/processes", middlewares.CheckAuthorizationHeader, v1.GetProcesses)

		go network.WsListen()
		apiV1.GET("/ws", middlewares.CheckAuthorizationHeader, network.WsHandler)
	}

	return router
}

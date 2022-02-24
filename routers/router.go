package routers

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
	"github.com/srad/streamsink/conf"
	v1 "github.com/srad/streamsink/routers/api/v1"

	docs "github.com/srad/streamsink/docs"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"
)

// @title           Swagger Example API
// @version         1.0
// @description     This is a sample server celler server.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:3000
// @BasePath  /api/v1

// @securityDefinitions.basic  BasicAuth

var (
	server *socketio.Server
)

// Setup InitRouter initialize routing information
func Setup() http.Handler {
	r := gin.New()
	//r.Use(gin.Logger())
	r.Use(gin.Recovery())
	const rec = "./recordings"
	r.Static("/recordings", conf.AppCfg.RecordingsAbsolutePath)
	r.Static("/public", conf.AppCfg.PublicPath)

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
	//apiv1 := r.Group("/api/v1", gin.BasicAuth(gin.Accounts{
	//	"user": "barcode",
	//}))
	apiv1.Use()
	{
		apiv1.GET("/channels", v1.GetChannels)
		apiv1.POST("/channels", v1.AddChannel)
		apiv1.POST("/channels/:channelName/tags", v1.TagChannel)
		apiv1.POST("/channels/:channelName/resume", v1.ResumeChannel)
		apiv1.POST("/channels/:channelName/pause", v1.PauseChannel)
		apiv1.POST("/channels/:channelName/fav", v1.FavChannel)
		apiv1.POST("/channels/:channelName/unfav", v1.UnFavChannel)
		apiv1.POST("/channels/:channelName/upload", v1.UploadChannel)
		apiv1.DELETE("/channels/:channelName", v1.DeleteChannel)

		apiv1.POST("/jobs/:channelName/:filename", v1.AddJob)
		apiv1.POST("/jobs/stop/:pid", v1.StopJob)
		apiv1.GET("/jobs", v1.GetJobs)

		//apiv1.POST("/recordings/updateinfo", v1.UpdateVideoInfo)

		apiv1.POST("/recorder/resume", v1.StartRecorder)
		apiv1.POST("/recorder/pause", v1.StopRecorder)
		apiv1.GET("/recorder", v1.IsRecording)

		apiv1.GET("/recordings", v1.GetRecordings)
		apiv1.GET("/recordings/latest/:limit", v1.GetLatestRecordings)
		apiv1.GET("/recordings/random/:limit", v1.GetRandomRecordings)
		apiv1.GET("/recordings/bookmarks", v1.GetBookmarks)
		apiv1.GET("/recordings/:channelName", v1.GetRecording)
		apiv1.GET("/recordings/:channelName/:filename/download", v1.DownloadRecording)

		apiv1.POST("/recordings/:channelName/:filename/bookmark/:bookmark", v1.Bookmark)
		apiv1.POST("/recordings/:channelName/:filename/cut", v1.CutRecording)
		apiv1.POST("/recordings/:channelName/:filename/preview", v1.GeneratePreview)

		apiv1.DELETE("/recordings/:channelName/:filename", v1.DeleteRecording)

		apiv1.GET("/info/:seconds", v1.GetInfo)
		apiv1.GET("/info/disk", v1.GetDiskInfo)

		apiv1.GET("/metric/cpu", v1.GetCpu)
		apiv1.GET("/metric/net", v1.GetNet)

		go v1.WsListen()
		apiv1.GET("/ws", v1.WsHandler)
	}

	return r
}

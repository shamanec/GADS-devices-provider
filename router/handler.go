package router

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func HandleRequests() *gin.Engine {

	router := gin.Default()
	router.Use(cors.Default())
	router.GET("/device/:udid/health", DeviceHealth)
	router.GET("/device/list", GetProviderDevices)
	router.GET("/containers/:containerID/logs", GetContainerLogs)
	router.POST("/device/create-udev-rules", CreateUdevRules)
	router.POST("/device/:udid/tap", DeviceTap)
	router.GET("/logs", GetLogs)

	return router
}

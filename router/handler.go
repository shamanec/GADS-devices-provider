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
	router.POST("/device/:udid/home", DeviceHome)
	router.POST("/device/:udid/lock", DeviceLock)
	router.POST("/device/:udid/unlock", DeviceUnlock)
	router.POST("/device/:udid/screenshot", DeviceScreenshot)
	router.POST("/device/:udid/swipe", DeviceSwipe)
	router.GET("/device/:udid/stream", DeviceStream)
	router.GET("/device/:udid/appiumSource", DeviceAppiumSource)
	router.POST("/device/:udid/typeText", DeviceTypeText)
	router.POST("/device/:udid/clearText", DeviceClearText)
	router.GET("/logs", GetLogs)
	router.Any("/device/:udid/appium/*proxyPath", AppiumReverseProxy)

	return router
}

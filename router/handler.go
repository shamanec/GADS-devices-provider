package router

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func HandleRequests() *gin.Engine {

	router := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"X-Auth-Token", "Content-Type"}
	router.Use(cors.New(config))
	router.GET("/device/:udid/health", DeviceHealth)
	router.POST("/device/:udid/tap", DeviceTap)
	router.POST("/device/:udid/home", DeviceHome)
	router.POST("/device/:udid/lock", DeviceLock)
	router.POST("/device/:udid/unlock", DeviceUnlock)
	router.POST("/device/:udid/screenshot", DeviceScreenshot)
	router.POST("/device/:udid/swipe", DeviceSwipe)
	router.GET("/device/:udid/appiumSource", DeviceAppiumSource)
	router.POST("/device/:udid/typeText", DeviceTypeText)
	router.POST("/device/:udid/clearText", DeviceClearText)
	router.Any("/device/:udid/appium/*proxyPath", AppiumReverseProxy)
	router.GET("/device/:udid/android-stream", AndroidStreamProxy)
	router.GET("/device/:udid/ios-stream", IosStreamProxy)

	router.POST("/provider/uploadFile", UploadFile)
	router.GET("/provider", GetProviderData)

	return router
}

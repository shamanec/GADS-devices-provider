package router

import (
	"github.com/gin-gonic/gin"
)

func HandleRequests() *gin.Engine {

	router := gin.Default()
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

	return router
}

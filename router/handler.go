package router

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func HandleRequests() *gin.Engine {
	// Start sending live provider data
	// to connected clients
	go sendProviderLiveData()

	r := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"X-Auth-Token", "Content-Type"}
	r.Use(cors.New(config))

	r.GET("/info", GetProviderData)
	r.GET("/info-ws", GetProviderDataWS)
	r.GET("/devices", DevicesInfo)
	r.POST("/uploadFile", UploadFile)
	r.POST("/addDevice", AddNewDevice)

	deviceGroup := r.Group("/device")
	deviceGroup.GET("/:udid/info", DeviceInfo)
	deviceGroup.GET("/:udid/health", DeviceHealth)
	deviceGroup.POST("/:udid/tap", DeviceTap)
	deviceGroup.POST("/:udid/touchAndHold", DeviceTouchAndHold)
	deviceGroup.POST("/:udid/home", DeviceHome)
	deviceGroup.POST("/:udid/lock", DeviceLock)
	deviceGroup.POST("/:udid/unlock", DeviceUnlock)
	deviceGroup.POST("/:udid/screenshot", DeviceScreenshot)
	deviceGroup.POST("/:udid/swipe", DeviceSwipe)
	deviceGroup.GET("/:udid/appiumSource", DeviceAppiumSource)
	deviceGroup.POST("/:udid/typeText", DeviceTypeText)
	deviceGroup.POST("/:udid/clearText", DeviceClearText)
	deviceGroup.Any("/:udid/appium/*proxyPath", AppiumReverseProxy)
	deviceGroup.GET("/:udid/android-stream", AndroidStreamProxy)
	deviceGroup.GET("/:udid/ios-stream", IosStreamProxy)
	deviceGroup.POST("/:udid/uninstallApp", UninstallApp)
	deviceGroup.POST("/:udid/installApp", InstallApp)
	deviceGroup.POST("/:udid/reset", ResetDevice)
	deviceGroup.POST("/:udid/uploadFile", UploadFile)

	return r
}

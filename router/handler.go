package router

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/shamanec/GADS-devices-provider/config"
)

func HandleRequests() *gin.Engine {
	// Start sending live provider data
	// to connected clients
	go sendProviderLiveData()

	r := gin.Default()
	rConfig := cors.DefaultConfig()
	rConfig.AllowAllOrigins = true
	rConfig.AllowHeaders = []string{"X-Auth-Token", "Content-Type"}
	r.Use(cors.New(rConfig))

	r.GET("/info", GetProviderData)
	r.GET("/info-ws", GetProviderDataWS)
	r.GET("/devices", DevicesInfo)
	r.POST("/uploadFile", UploadFile)

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
	if config.Config.EnvConfig.UseGadsIosStream {
		deviceGroup.GET("/:udid/ios-stream", IosStreamProxyWDA)
	} else {
		deviceGroup.GET("/:udid/ios-stream", IosStreamProxyGADS)
	}
	deviceGroup.POST("/:udid/uninstallApp", UninstallApp)
	deviceGroup.POST("/:udid/installApp", InstallApp)
	deviceGroup.POST("/:udid/reset", ResetDevice)
	deviceGroup.POST("/:udid/uploadFile", UploadFile)

	return r
}

package router

import (
	"context"
	"fmt"
	"log"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/shamanec/GADS-devices-provider/device"
)

func StreamProxy(c *gin.Context) {
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		fmt.Println(err)
	}

	defer conn.Close()

	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	u := url.URL{Scheme: "ws", Host: "localhost:" + device.Device.ContainerServerPort, Path: "android-stream"}
	destConn, _, _, err := ws.DefaultDialer.Dial(context.Background(), u.String())
	if err != nil {
		log.Println("Destination WebSocket connection error:", err)
		return
	}

	defer destConn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			data, code, err := wsutil.ReadClientData(destConn)
			if err != nil {
				log.Println("Destination read error:", err)
				return
			}
			err = wsutil.WriteServerMessage(conn, code, data)
			if err != nil {
				log.Println("Proxy write error:", err)
				return
			}
		}
	}()

	for {
		data, code, err := wsutil.ReadClientData(conn)
		if err != nil {
			log.Println("Proxy read error:", err)
			return
		}
		err = wsutil.WriteServerMessage(destConn, code, data)
		if err != nil {
			log.Println("Destination write error:", err)
			return
		}
	}
}

func HandleRequests() *gin.Engine {

	router := gin.Default()
	router.GET("/device/:udid/health", DeviceHealth)
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
	router.Any("/device/:udid/appium/*proxyPath", AppiumReverseProxy)
	router.GET("/device/:udid/android-stream", AndroidStreamProxy)
	router.GET("/device/:udid/ios-stream", IosStreamProxy)

	return router
}

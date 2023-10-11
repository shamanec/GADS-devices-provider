package router

import (
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/shamanec/GADS-devices-provider/device"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	CheckOrigin:      func(r *http.Request) bool { return true },
	HandshakeTimeout: time.Duration(time.Second * 5),
}

func StreamProxy(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	u := url.URL{Scheme: "ws", Host: "localhost:" + device.Device.ContainerServerPort, Path: "android-stream"}
	destConn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("Destination WebSocket connection error:", err)
		return
	}
	defer destConn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			messageType, p, err := destConn.ReadMessage()
			if err != nil {
				log.Println("Destination read error:", err)
				return
			}
			err = conn.WriteMessage(messageType, p)
			if err != nil {
				log.Println("Proxy write error:", err)
				return
			}
		}
	}()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Proxy read error:", err)
			return
		}
		err = destConn.WriteMessage(messageType, p)
		if err != nil {
			log.Println("Destination write error:", err)
			return
		}
	}
}

func HandleRequests() *gin.Engine {

	router := gin.Default()
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
	router.GET("/device/:udid/android-stream", AndroidStreamProxy)
	router.GET("/device/:udid/ios-stream", IosStreamProxy)

	return router
}

package router

import (
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/shamanec/GADS-devices-provider/device"
)

func AndroidStreamProxy(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	u := url.URL{Scheme: "ws", Host: "localhost:" + device.Device.StreamPort, Path: ""}
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

func IosStreamProxy(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	url := "http://localhost:" + device.Device.StreamPort

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}
	defer resp.Body.Close()

	// Get the media type and params after connecting to WebDriverAgent stream
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		fmt.Println("Error getting request mediaType and params:", err)
		return
	}

	// Get the boundary string
	// It has leading slashes -- that need to be removed for it to work properly
	boundary := strings.Replace(params["boundary"], "--", "", -1)

	// Should be multipart/x-mixed-replace
	// We know its that one but check just in case
	if strings.HasPrefix(mediaType, "multipart/") {
		// Create a multipart reader from the response using the cleaned boundary
		mr := multipart.NewReader(resp.Body, boundary)

		// Loop and for each part in the multpart reader read the image and send it over the ws
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			jpg, err := io.ReadAll(part)
			if err != nil {
				break
			}
			conn.WriteMessage(websocket.BinaryMessage, jpg)
		}
	}
}

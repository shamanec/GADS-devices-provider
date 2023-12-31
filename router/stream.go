package router

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/shamanec/GADS-devices-provider/devices"
)

func AndroidStreamProxy(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]

	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		fmt.Println(err)
	}

	defer conn.Close()

	u := url.URL{Scheme: "ws", Host: "localhost:" + device.StreamPort, Path: ""}
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
			data, code, err := wsutil.ReadServerData(destConn)
			if err != nil {
				return
			}

			err = wsutil.WriteServerMessage(conn, code, data)
			if err != nil {
				return
			}
		}
	}()

	for {
		data, code, err := wsutil.ReadClientData(conn)
		if err != nil {
			return
		}

		err = wsutil.WriteServerMessage(destConn, code, data)
		if err != nil {
			return
		}
	}
}

func IosStreamProxy(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]

	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		fmt.Println(err)
	}
	defer conn.Close()

	url := "http://localhost:" + device.StreamPort

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
			wsutil.WriteServerBinary(conn, jpg)
		}
	}
}

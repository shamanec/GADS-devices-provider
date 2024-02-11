package router

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
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

func findJPEGMarkers(data []byte) (int, int) {
	start := bytes.Index(data, []byte{0xFF, 0xD8})
	end := bytes.Index(data, []byte{0xFF, 0xD9})
	return start, end
}

func IosStreamProxy2(c *gin.Context) {
	udid := c.Param("udid")
	device := devices.DeviceMap[udid]

	// Create the new conn
	wsConn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		fmt.Println(err)
	}
	defer wsConn.Close()

	// Read data from device
	server := "localhost:" + device.StreamPort
	// Connect to the server
	conn, err := net.Dial("tcp", server)
	if err != nil {
		fmt.Println("Error connecting:", err.Error())
		os.Exit(1)
	}
	defer conn.Close()

	var buffer []byte
	for {
		// Read data from the connection
		tempBuf := make([]byte, 1024)
		n, err := conn.Read(tempBuf)
		if err != nil {
			if err != io.EOF {
				return
			}
			break
		}

		// Append the read bytes to the buffer
		buffer = append(buffer, tempBuf[:n]...)

		// Check if buffer has a complete JPEG image
		start, end := findJPEGMarkers(buffer)
		if start >= 0 && end > start {
			// Process the JPEG image
			jpegImage := buffer[start : end+2] // Include end marker
			// Keep any remaining data in the buffer for the next image
			buffer = buffer[end+2:]
			// Send the jpeg over the websocket
			wsutil.WriteServerBinary(wsConn, jpegImage)
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

	streamUrl := "http://localhost:" + device.StreamPort

	req, err := http.NewRequest("GET", streamUrl, nil)
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
	// We know it's that one but check just in case
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

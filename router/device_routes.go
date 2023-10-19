package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shamanec/GADS-devices-provider/device"
)

// Check the device health by checking Appium and WDA(for iOS)
func DeviceHealth(c *gin.Context) {
	udid := c.Param("udid")
	bool, err := device.GetDeviceHealth(udid)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if bool {
		c.Writer.WriteHeader(200)
		return
	}

	c.Writer.WriteHeader(500)
}

// Call the respective Appium/WDA endpoint to go to Homescreen
func DeviceHome(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	// Send the request
	homeResponse, err := appiumHome(device)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer homeResponse.Body.Close()

	// Read the response body
	homeResponseBody, err := io.ReadAll(homeResponse.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(homeResponse.StatusCode)
	copyHeaders(c.Writer.Header(), homeResponse.Header)
	fmt.Fprintf(c.Writer, string(homeResponseBody))
}

// Call respective Appium/WDA endpoint to lock the device
func DeviceLock(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	lockResponse, err := appiumLockUnlock(device, "lock")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := io.ReadAll(lockResponse.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(lockResponse.StatusCode)
	copyHeaders(c.Writer.Header(), lockResponse.Header)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

// Call the respective Appium/WDA endpoint to unlock the device
func DeviceUnlock(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	lockResponse, err := appiumLockUnlock(device, "unlock")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := io.ReadAll(lockResponse.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(lockResponse.StatusCode)
	copyHeaders(c.Writer.Header(), lockResponse.Header)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

// Call the respective Appium/WDA endpoint to take a screenshot of the device screen
func DeviceScreenshot(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	screenshotResp, err := appiumScreenshot(device)
	defer screenshotResp.Body.Close()

	// Read the response body
	screenshotRespBody, err := io.ReadAll(screenshotResp.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(screenshotResp.StatusCode)
	copyHeaders(c.Writer.Header(), screenshotResp.Header)
	fmt.Fprintf(c.Writer, string(screenshotRespBody))
}

// ================================
// Device screen streaming

// Call the device stream endpoint and proxy it to the respective provider stream endpoint
func DeviceStream(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	deviceStreamURL := ""
	if device.Device.OS == "android" {
		deviceStreamURL = "http://localhost:" + device.Device.ContainerServerPort + "/stream"
	}

	if device.Device.OS == "ios" {
		deviceStreamURL = "http://localhost:" + device.Device.StreamPort
	}
	client := http.Client{}

	// Replace this URL with the actual endpoint URL serving the JPEG stream
	resp, err := client.Get(deviceStreamURL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error connecting to the stream")
		return
	}
	defer resp.Body.Close()

	copyHeaders(c.Writer.Header(), resp.Header)
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		return
	}
}

// Copy the headers from the original endpoint to the proxied endpoint
func copyHeaders(destination, source http.Header) {
	for name, values := range source {
		for _, v := range values {
			destination.Add(name, v)
		}
	}
}

//======================================
// Appium source

func DeviceAppiumSource(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	sourceResp, err := appiumSource(device)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Read the response body
	body, err := io.ReadAll(sourceResp.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer sourceResp.Body.Close()

	c.Writer.WriteHeader(sourceResp.StatusCode)
	copyHeaders(c.Writer.Header(), sourceResp.Header)
	fmt.Fprint(c.Writer, string(body))
}

//=======================================
// ACTIONS

type actionData struct {
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	EndX       float64 `json:"endX,omitempty"`
	EndY       float64 `json:"endY,omitempty"`
	TextToType string  `json:"text,omitempty"`
}

type deviceAction struct {
	Type     string  `json:"type"`
	Duration int     `json:"duration"`
	X        float64 `json:"x,omitempty"`
	Y        float64 `json:"y,omitempty"`
	Button   int     `json:"button"`
	Origin   string  `json:"origin,omitempty"`
}

type deviceActionParameters struct {
	PointerType string `json:"pointerType"`
}

type devicePointerAction struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Parameters deviceActionParameters `json:"parameters"`
	Actions    []deviceAction         `json:"actions"`
}

type devicePointerActions struct {
	Actions []devicePointerAction `json:"actions"`
}

func DeviceTypeText(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	typeResp, err := appiumTypeText(device, requestBody.TextToType)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	body, err := io.ReadAll(typeResp.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer typeResp.Body.Close()

	c.Writer.WriteHeader(typeResp.StatusCode)
	copyHeaders(c.Writer.Header(), typeResp.Header)
	fmt.Fprintf(c.Writer, string(body))
}

func DeviceClearText(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	clearResp, err := appiumClearText(device)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	body, err := io.ReadAll(clearResp.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer clearResp.Body.Close()

	c.Writer.WriteHeader(clearResp.StatusCode)
	copyHeaders(c.Writer.Header(), clearResp.Header)
	fmt.Fprintf(c.Writer, string(body))
}

func DeviceTap(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	tapResp, err := appiumTap(device, requestBody.X, requestBody.Y)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer tapResp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(tapResp.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(tapResp.StatusCode)
	copyHeaders(c.Writer.Header(), tapResp.Header)
	fmt.Fprintf(c.Writer, string(body))
}

func DeviceSwipe(c *gin.Context) {
	udid := c.Param("udid")
	device := device.DeviceMap[udid]

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	swipeResp, err := appiumSwipe(device, requestBody.X, requestBody.Y, requestBody.EndX, requestBody.EndY)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer swipeResp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(swipeResp.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Writer.WriteHeader(swipeResp.StatusCode)
	copyHeaders(c.Writer.Header(), swipeResp.Header)
	fmt.Fprintf(c.Writer, string(body))
}

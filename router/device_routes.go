package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shamanec/GADS-devices-provider/device"
	"github.com/shamanec/GADS-devices-provider/util"
	log "github.com/sirupsen/logrus"
)

// Check the device health by checking Appium and WDA(for iOS)
func DeviceHealth(c *gin.Context) {
	udid := c.Param("udid")
	bool, err := device.GetDeviceHealth(udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "check_device_health",
		}).Error("Could not check device health, err: " + err.Error())
		JSONError(c.Writer, "check_device_health", "Could not check device health", 500)
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
	device := device.GetDeviceByUDID(udid)

	host := "http://localhost:"

	var deviceHomeURL string
	if device.OS == "android" {
		var requestBody actionData
		if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}

		deviceHomeURL = host + device.AppiumPort + "/session/" + requestBody.SessionID + "/appium/device/press_keycode"
	}

	if device.OS == "ios" {
		deviceHomeURL = host + device.WDAPort + "/wda/homescreen"
	}

	// Create a new HTTP client
	client := http.DefaultClient

	homeRequestBody := ""
	if device.OS == "android" {
		homeRequestBody = `{"keycode": 3}`
	}

	req, err := http.NewRequest(http.MethodPost, deviceHomeURL, bytes.NewBuffer([]byte(homeRequestBody)))
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	homeResponse, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer homeResponse.Body.Close()

	// Read the response body
	homeResponseBody, err := ioutil.ReadAll(homeResponse.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	copyHeaders(c.Writer.Header(), homeResponse.Header)
	fmt.Fprintf(c.Writer, string(homeResponseBody))
}

// Call respective Appium/WDA endpoint to lock the device
func DeviceLock(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	host := "http://localhost:"
	var deviceHomeURL string
	if device.OS == "android" {
		deviceHomeURL = host + device.AppiumPort + "/session/" + requestBody.SessionID + "/appium/device/lock"
	}

	if device.OS == "ios" {
		deviceHomeURL = host + device.WDAPort + "/session/" + requestBody.SessionID + "/wda/lock"
	}

	// Create a new HTTP client
	client := http.DefaultClient

	req, err := http.NewRequest(http.MethodPost, deviceHomeURL, nil)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	lockResponse, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := ioutil.ReadAll(lockResponse.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	copyHeaders(c.Writer.Header(), lockResponse.Header)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

// Call the respective Appium/WDA endpoint to unlock the device
func DeviceUnlock(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	host := "http://localhost:"
	var deviceHomeURL string
	if device.OS == "android" {
		deviceHomeURL = host + device.AppiumPort + "/session/" + requestBody.SessionID + "/appium/device/unlock"
	}

	if device.OS == "ios" {
		deviceHomeURL = host + device.WDAPort + "/session/" + requestBody.SessionID + "/wda/unlock"
	}

	// Create a new HTTP client
	client := http.DefaultClient

	req, err := http.NewRequest(http.MethodPost, deviceHomeURL, nil)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	lockResponse, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := ioutil.ReadAll(lockResponse.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	copyHeaders(c.Writer.Header(), lockResponse.Header)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

// Call the respective Appium/WDA endpoint to take a screenshot of the device screen
func DeviceScreenshot(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	host := "http://localhost:"
	var deviceHomeURL string
	if device.OS == "android" {
		deviceHomeURL = host + device.AppiumPort + "/session/" + requestBody.SessionID + "/screenshot"
	}

	if device.OS == "ios" {
		deviceHomeURL = host + device.WDAPort + "/session/" + requestBody.SessionID + "/screenshot"
	}

	// Create a new HTTP client
	client := http.DefaultClient

	req, err := http.NewRequest(http.MethodGet, deviceHomeURL, nil)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	lockResponse, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := ioutil.ReadAll(lockResponse.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	copyHeaders(c.Writer.Header(), lockResponse.Header)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

// ================================
// Device screen streaming
const mjpegFrameFooter = "\r\n\r\n"
const mjpegFrameHeader = "--BoundaryString\r\nContent-type: image/jpg\r\nContent-Length: %d\r\n\r\n"

// Call the device stream endpoint and proxy it to the respective provider stream endpoint
func DeviceStream(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	deviceStreamURL := ""
	if device.OS == "android" {
		deviceStreamURL = "http://localhost:" + device.ContainerServerPort + "/stream"
	}

	if device.OS == "ios" {
		deviceStreamURL = "http://localhost:" + device.StreamPort
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

//=======================================
// ACTIONS

type actionData struct {
	X         float64 `json:"x,omitempty"`
	Y         float64 `json:"y,omitempty"`
	EndX      float64 `json:"endX,omitempty"`
	EndY      float64 `json:"endY,omitempty`
	SessionID string  `json:"sessionID,omitempty"`
}

func DeviceTap(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	var requestURL string

	if device.OS == "android" {
		requestURL = "http://localhost:" + device.AppiumPort + "/session/" + requestBody.SessionID + "/actions"
	}

	if device.OS == "ios" {
		requestURL = "http://localhost:" + device.WDAPort + "/session/" + requestBody.SessionID + "/actions"
	}

	action := devicePointerActions{
		[]devicePointerAction{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: deviceActionParameters{
					PointerType: "touch",
				},
				Actions: []deviceAction{
					{
						Type:     "pointerMove",
						Duration: 0,
						X:        requestBody.X,
						Y:        requestBody.Y,
					},
					{
						Type:   "pointerDown",
						Button: 0,
					},
					{
						Type:     "pause",
						Duration: 200,
					},
					{
						Type:     "pointerUp",
						Duration: 0,
					},
				},
			},
		},
	}

	actionJSON, err := util.ConvertToJSONString(action)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	// Create a new HTTP client
	client := http.DefaultClient

	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewBuffer([]byte(actionJSON)))
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	res, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer res.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	copyHeaders(c.Writer.Header(), res.Header)
	fmt.Fprintf(c.Writer, string(body))
}

func DeviceSwipe(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	var requestURL string

	if device.OS == "android" {
		requestURL = "http://localhost:" + device.AppiumPort + "/session/" + requestBody.SessionID + "/actions"
	}

	if device.OS == "ios" {
		requestURL = "http://localhost:" + device.WDAPort + "/session/" + requestBody.SessionID + "/actions"
	}

	action := devicePointerActions{
		[]devicePointerAction{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: deviceActionParameters{
					PointerType: "touch",
				},
				Actions: []deviceAction{
					{
						Type:     "pointerMove",
						Duration: 0,
						X:        requestBody.X,
						Y:        requestBody.Y,
					},
					{
						Type:   "pointerDown",
						Button: 0,
					},
					{
						Type:     "pointerMove",
						Duration: 750,
						Origin:   "viewport",
						X:        requestBody.EndX,
						Y:        requestBody.EndY,
					},
					{
						Type:     "pointerUp",
						Duration: 0,
					},
				},
			},
		},
	}

	actionJSON, err := util.ConvertToJSONString(action)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	// Create a new HTTP client
	client := http.DefaultClient

	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewBuffer([]byte(actionJSON)))
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	res, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer res.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	copyHeaders(c.Writer.Header(), res.Header)
	fmt.Fprintf(c.Writer, string(body))
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

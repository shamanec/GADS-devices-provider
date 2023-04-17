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
		JSONError(c.Writer, "check_device_health", "Could not check device health, err:"+err.Error(), 500)
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
		deviceHomeURL = host + device.AppiumPort + "/session/" + device.AppiumSessionID + "/appium/device/press_keycode"
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

	lockResponse, err := appiumLockUnlock(device, "lock")
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := ioutil.ReadAll(lockResponse.Body)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	copyHeaders(c.Writer.Header(), lockResponse.Header)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

// Call the respective Appium/WDA endpoint to unlock the device
func DeviceUnlock(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	lockResponse, err := appiumLockUnlock(device, "unlock")
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := ioutil.ReadAll(lockResponse.Body)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	copyHeaders(c.Writer.Header(), lockResponse.Header)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

// Call the respective Appium/WDA endpoint to take a screenshot of the device screen
func DeviceScreenshot(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	screenshotResp, err := appiumScreenshot(device)
	defer screenshotResp.Body.Close()

	// Read the response body
	screenshotRespBody, err := ioutil.ReadAll(screenshotResp.Body)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	copyHeaders(c.Writer.Header(), screenshotResp.Header)
	fmt.Fprintf(c.Writer, string(screenshotRespBody))
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

//======================================
// Appium source

func DeviceAppiumSource(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	resp, err := appiumSource(device)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
	}

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	defer resp.Body.Close()

	copyHeaders(c.Writer.Header(), resp.Header)
	fmt.Fprintf(c.Writer, string(body))
}

//=======================================
// ACTIONS

type actionData struct {
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	EndX       float64 `json:"endX,omitempty"`
	EndY       float64 `json:"endY,omitempty`
	TextToType string  `json:"text,omitempty"`
}

func DeviceTypeText(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	var activeElementRequestURL string

	if device.OS == "android" {
		activeElementRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/active"
	}

	if device.OS == "ios" {
		activeElementRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/element/active"
	}

	activeElementResp, err := http.Get(activeElementRequestURL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Could not get active element: "+err.Error())
		return
	}

	// Read the response body
	activeElementRespBody, err := ioutil.ReadAll(activeElementResp.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	var activeElementData map[string]interface{}
	err = json.Unmarshal(activeElementRespBody, &activeElementData)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	activeElementID := activeElementData["value"].(map[string]interface{})["ELEMENT"].(string)

	setValueRequestURL := ""
	if device.OS == "android" {
		setValueRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/" + activeElementID + "/value"
	}

	if device.OS == "ios" {
		setValueRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/element/" + activeElementID + "/value"
	}

	setValueRequestBody := `{"text":"` + requestBody.TextToType + `"}`
	setValueResponse, err := http.Post(setValueRequestURL, "application/json", bytes.NewBuffer([]byte(setValueRequestBody)))
	// Read the response body
	body, err := ioutil.ReadAll(setValueResponse.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	copyHeaders(c.Writer.Header(), setValueResponse.Header)
	fmt.Fprintf(c.Writer, string(body))
}

func DeviceClearText(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var activeElementRequestURL string

	if device.OS == "android" {
		activeElementRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/active"
	}

	if device.OS == "ios" {
		activeElementRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/element/active"
	}

	activeElementResp, err := http.Get(activeElementRequestURL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Could not get active element: "+err.Error())
		return
	}

	// Read the response body
	activeElementRespBody, err := ioutil.ReadAll(activeElementResp.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	var activeElementData map[string]interface{}
	err = json.Unmarshal(activeElementRespBody, &activeElementData)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	activeElementID := activeElementData["value"].(map[string]interface{})["ELEMENT"].(string)

	clearValueRequestURL := ""
	if device.OS == "android" {
		clearValueRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/" + activeElementID + "/clear"
	}

	if device.OS == "ios" {
		clearValueRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/element/" + activeElementID + "/clear"
	}

	clearValueResponse, err := http.Post(clearValueRequestURL, "application/json", nil)
	// Read the response body
	body, err := ioutil.ReadAll(clearValueResponse.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	copyHeaders(c.Writer.Header(), clearValueResponse.Header)
	fmt.Fprintf(c.Writer, string(body))
}

func DeviceTap(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := appiumTap(device, requestBody.X, requestBody.Y)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
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

	res, err := appiumSwipe(device, requestBody.X, requestBody.Y, requestBody.EndX, requestBody.EndY)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
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

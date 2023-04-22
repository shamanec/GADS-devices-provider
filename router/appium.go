package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/shamanec/GADS-devices-provider/device"
	"github.com/shamanec/GADS-devices-provider/util"
)

func appiumLockUnlock(device *device.Device, lock string) (*http.Response, error) {
	host := "http://localhost:"
	var deviceHomeURL string
	if device.OS == "android" {
		deviceHomeURL = host + device.AppiumPort + "/session/" + device.AppiumSessionID + "/appium/device/" + lock
	}

	if device.OS == "ios" {
		deviceHomeURL = host + device.WDAPort + "/session/" + device.WDASessionID + "/wda/" + lock
	}

	lockResponse, err := http.Post(deviceHomeURL, "", nil)
	if err != nil {
		return nil, err
	}

	return lockResponse, nil
}

func appiumTap(device *device.Device, x float64, y float64) (*http.Response, error) {
	var appiumRequestURL string

	// Generate the respective Appium server request url
	switch device.OS {
	case "android":
		appiumRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/actions"
	case "ios":
		appiumRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/actions"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.OS)
	}

	// Generate the struct object for the Appium actions JSON request
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
						X:        x,
						Y:        y,
					},
					{
						Type:   "pointerDown",
						Button: 0,
					},
					{
						Type:     "pause",
						Duration: 50,
					},
					{
						Type:     "pointerUp",
						Duration: 0,
					},
				},
			},
		},
	}

	// Convert the struct object to an actual JSON string
	actionJSON, err := util.ConvertToJSONString(action)
	if err != nil {
		return nil, fmt.Errorf("Could not convert Appium actions struct to a JSON string: %s", err)
	}

	// Create a new http client
	client := http.DefaultClient
	// Generate the request
	req, err := http.NewRequest(http.MethodPost, appiumRequestURL, bytes.NewBuffer([]byte(actionJSON)))
	if err != nil {
		return nil, fmt.Errorf("Could not generate http request to Appium /actions endpoint: %s", err)
	}

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed calling Appium /actions endpoint: %s", err)
	}

	// Return the response object
	return res, nil
}

func appiumSwipe(device *device.Device, x, y, endX, endY float64) (*http.Response, error) {
	var appiumRequestURL string

	// Generate the respective Appium server request url
	switch device.OS {
	case "android":
		appiumRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/actions"
	case "ios":
		appiumRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/actions"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.OS)
	}

	// Generate the struct object for the Appium actions JSON request
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
						X:        x,
						Y:        y,
					},
					{
						Type:   "pointerDown",
						Button: 0,
					},
					{
						Type:     "pointerMove",
						Duration: 500,
						Origin:   "viewport",
						X:        endX,
						Y:        endY,
					},
					{
						Type:     "pointerUp",
						Duration: 0,
					},
				},
			},
		},
	}

	// Convert the struct object to an actual JSON string
	actionJSON, err := util.ConvertToJSONString(action)
	if err != nil {
		return nil, fmt.Errorf("Could not convert Appium actions struct to a JSON string: %s", err)
	}

	// Create a new http client
	client := http.DefaultClient
	// Generate the request
	req, err := http.NewRequest(http.MethodPost, appiumRequestURL, bytes.NewBuffer([]byte(actionJSON)))
	if err != nil {
		return nil, fmt.Errorf("Could not generate http request to Appium /actions endpoint: %s", err)
	}

	// Perform the request
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed calling Appium /actions endpoint: %s", err)
	}

	// Return the response object
	return res, nil
}

func appiumSource(device *device.Device) (*http.Response, error) {
	sourceURL := ""
	if device.OS == "android" {
		sourceURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/source"
	}

	if device.OS == "ios" {
		sourceURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/source"
	}

	resp, err := http.Get(sourceURL)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func appiumScreenshot(device *device.Device) (*http.Response, error) {
	host := "http://localhost:"
	var screenshotURL string
	if device.OS == "android" {
		screenshotURL = host + device.AppiumPort + "/session/" + device.AppiumSessionID + "/screenshot"
	}

	if device.OS == "ios" {
		screenshotURL = host + device.WDAPort + "/session/" + device.WDASessionID + "/screenshot"
	}

	resp, err := http.Get(screenshotURL)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func appiumTypeText(device *device.Device, text string) (*http.Response, error) {
	var activeElementRequestURL string

	if device.OS == "android" {
		activeElementRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/active"
	}

	if device.OS == "ios" {
		activeElementRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/element/active"
	}

	activeElementResp, err := http.Get(activeElementRequestURL)
	if err != nil {
		return nil, err
	}

	// Read the response body
	activeElementRespBody, err := ioutil.ReadAll(activeElementResp.Body)
	if err != nil {
		return nil, err
	}

	var activeElementData map[string]interface{}
	err = json.Unmarshal(activeElementRespBody, &activeElementData)
	if err != nil {
		return nil, err
	}

	activeElementID := activeElementData["value"].(map[string]interface{})["ELEMENT"].(string)

	setValueRequestURL := ""
	if device.OS == "android" {
		setValueRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/" + activeElementID + "/value"
	}

	if device.OS == "ios" {
		setValueRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/element/" + activeElementID + "/value"
	}

	setValueRequestBody := `{"text":"` + text + `"}`
	setValueResponse, err := http.Post(setValueRequestURL, "application/json", bytes.NewBuffer([]byte(setValueRequestBody)))
	if err != nil {
		return nil, err
	}

	return setValueResponse, nil
}

func appiumClearText(device *device.Device) (*http.Response, error) {
	var activeElementRequestURL string

	if device.OS == "android" {
		activeElementRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/active"
	}

	if device.OS == "ios" {
		activeElementRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/element/active"
	}

	activeElementResp, err := http.Get(activeElementRequestURL)
	if err != nil {
		return nil, err
	}

	// Read the response body
	activeElementRespBody, err := ioutil.ReadAll(activeElementResp.Body)
	if err != nil {
		return nil, err
	}

	var activeElementData map[string]interface{}
	err = json.Unmarshal(activeElementRespBody, &activeElementData)
	if err != nil {
		return nil, err
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
	if err != nil {
		return nil, err
	}

	return clearValueResponse, nil
}

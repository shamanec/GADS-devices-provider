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

func appiumLockUnlock(device *device.LocalDevice, lock string) (*http.Response, error) {
	var deviceHomeURL string
	switch device.Device.OS {
	case "android":
		deviceHomeURL = "http://localhost:" + device.Device.AppiumPort + "/session/" + device.Device.AppiumSessionID + "/appium/device/" + lock
	case "ios":
		deviceHomeURL = "http://localhost:" + device.Device.WDAPort + "/session/" + device.Device.WDASessionID + "/wda/" + lock
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.Device.OS)
	}

	lockResponse, err := http.Post(deviceHomeURL, "", nil)
	if err != nil {
		return nil, err
	}

	return lockResponse, nil
}

func appiumTap(device *device.LocalDevice, x float64, y float64) (*http.Response, error) {
	var appiumRequestURL string

	// Generate the respective Appium server request url
	switch device.Device.OS {
	case "android":
		appiumRequestURL = "http://localhost:" + device.Device.AppiumPort + "/session/" + device.Device.AppiumSessionID + "/actions"
	case "ios":
		appiumRequestURL = "http://localhost:" + device.Device.WDAPort + "/session/" + device.Device.WDASessionID + "/actions"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.Device.OS)
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

func appiumSwipe(device *device.LocalDevice, x, y, endX, endY float64) (*http.Response, error) {
	var appiumRequestURL string

	// Generate the respective Appium server request url
	switch device.Device.OS {
	case "android":
		appiumRequestURL = "http://localhost:" + device.Device.AppiumPort + "/session/" + device.Device.AppiumSessionID + "/actions"
	case "ios":
		appiumRequestURL = "http://localhost:" + device.Device.WDAPort + "/session/" + device.Device.WDASessionID + "/actions"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.Device.OS)
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

func appiumSource(device *device.LocalDevice) (*http.Response, error) {
	sourceURL := ""
	switch device.Device.OS {
	case "android":
		sourceURL = "http://localhost:" + device.Device.AppiumPort + "/session/" + device.Device.AppiumSessionID + "/source"
	case "ios":
		sourceURL = "http://localhost:" + device.Device.WDAPort + "/session/" + device.Device.WDASessionID + "/source"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.Device.OS)
	}

	client := http.DefaultClient
	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Could not generate http request to Appium /source endpoint: %s", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed calling Appium /source endpoint: %s", err)
	}

	return res, nil
}

func appiumScreenshot(device *device.LocalDevice) (*http.Response, error) {
	var screenshotURL string
	switch device.Device.OS {
	case "android":
		screenshotURL = "http://localhost:" + device.Device.AppiumPort + "/session/" + device.Device.AppiumSessionID + "/screenshot"
	case "ios":
		screenshotURL = "http://localhost:" + device.Device.WDAPort + "/session/" + device.Device.WDASessionID + "/screenshot"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.Device.OS)
	}

	resp, err := http.Get(screenshotURL)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type ActiveElementData struct {
	Value struct {
		Element string `json:"ELEMENT"`
	} `json:"value"`
}

func appiumTypeText(device *device.LocalDevice, text string) (*http.Response, error) {
	var activeElementRequestURL string
	switch device.Device.OS {
	case "android":
		activeElementRequestURL = "http://localhost:" + device.Device.AppiumPort + "/session/" + device.Device.AppiumSessionID + "/element/active"
	case "ios":
		activeElementRequestURL = "http://localhost:" + device.Device.WDAPort + "/session/" + device.Device.WDASessionID + "/element/active"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.Device.OS)
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
	fmt.Println(string(activeElementRespBody))

	var activeElementData ActiveElementData
	err = json.Unmarshal(activeElementRespBody, &activeElementData)
	if err != nil {
		return nil, err
	}

	fmt.Printf("%s", activeElementData)
	activeElementID := activeElementData.Value.Element

	setValueRequestURL := ""
	switch device.Device.OS {
	case "android":
		setValueRequestURL = "http://localhost:" + device.Device.AppiumPort + "/session/" + device.Device.AppiumSessionID + "/element/" + activeElementID + "/value"
	case "ios":
		setValueRequestURL = "http://localhost:" + device.Device.WDAPort + "/session/" + device.Device.WDASessionID + "/element/" + activeElementID + "/value"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.Device.OS)
	}

	setValueRequestBody := `{"text":"` + text + `"}`
	setValueResponse, err := http.Post(setValueRequestURL, "application/json", bytes.NewBuffer([]byte(setValueRequestBody)))
	if err != nil {
		return nil, err
	}

	return setValueResponse, nil
}

func appiumClearText(device *device.LocalDevice) (*http.Response, error) {
	var activeElementRequestURL string
	switch device.Device.OS {
	case "android":
		activeElementRequestURL = "http://localhost:" + device.Device.AppiumPort + "/session/" + device.Device.AppiumSessionID + "/element/active"
	case "ios":
		activeElementRequestURL = "http://localhost:" + device.Device.WDAPort + "/session/" + device.Device.WDASessionID + "/element/active"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.Device.OS)
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
	switch device.Device.OS {
	case "android":
		clearValueRequestURL = "http://localhost:" + device.Device.AppiumPort + "/session/" + device.Device.AppiumSessionID + "/element/" + activeElementID + "/clear"
	case "ios":
		clearValueRequestURL = "http://localhost:" + device.Device.WDAPort + "/session/" + device.Device.WDASessionID + "/element/" + activeElementID + "/clear"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.Device.OS)
	}

	clearValueResponse, err := http.Post(clearValueRequestURL, "application/json", nil)
	if err != nil {
		return nil, err
	}

	return clearValueResponse, nil
}

func appiumHome(device *device.LocalDevice) (*http.Response, error) {
	var homeURL string
	switch device.Device.OS {
	case "android":
		homeURL = "http://localhost:" + device.Device.AppiumPort + "/session/" + device.Device.AppiumSessionID + "/appium/device/press_keycode"
	case "ios":
		homeURL = "http://localhost:" + device.Device.WDAPort + "/wda/homescreen"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.Device.OS)
	}

	requestBody := ""
	if device.Device.OS == "android" {
		requestBody = `{"keycode": 3}`
	}

	homeResponse, err := http.Post(homeURL, "application/json", bytes.NewBuffer([]byte(requestBody)))
	if err != nil {
		return nil, err
	}

	return homeResponse, nil
}

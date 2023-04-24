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
	var deviceHomeURL string
	switch device.OS {
	case "android":
		deviceHomeURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/appium/device/" + lock
	case "ios":
		deviceHomeURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/wda/" + lock
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.OS)
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
	switch device.OS {
	case "android":
		sourceURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/source"
	case "ios":
		sourceURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/source"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.OS)
	}

	resp, err := http.Get(sourceURL)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func appiumScreenshot(device *device.Device) (*http.Response, error) {
	var screenshotURL string
	switch device.OS {
	case "android":
		screenshotURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/screenshot"
	case "ios":
		screenshotURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/screenshot"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.OS)
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

func appiumTypeText(device *device.Device, text string) (*http.Response, error) {
	var activeElementRequestURL string
	switch device.OS {
	case "android":
		activeElementRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/active"
	case "ios":
		activeElementRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/element/active"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.OS)
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
	switch device.OS {
	case "android":
		setValueRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/" + activeElementID + "/value"
	case "ios":
		setValueRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/element/" + activeElementID + "/value"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.OS)
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
	switch device.OS {
	case "android":
		activeElementRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/active"
	case "ios":
		activeElementRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/element/active"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.OS)
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
	switch device.OS {
	case "android":
		clearValueRequestURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/" + activeElementID + "/clear"
	case "ios":
		clearValueRequestURL = "http://localhost:" + device.WDAPort + "/session/" + device.WDASessionID + "/element/" + activeElementID + "/clear"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.OS)
	}

	clearValueResponse, err := http.Post(clearValueRequestURL, "application/json", nil)
	if err != nil {
		return nil, err
	}

	return clearValueResponse, nil
}

func appiumHome(device *device.Device) (*http.Response, error) {
	var homeURL string
	switch device.OS {
	case "android":
		homeURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/appium/device/press_keycode"
	case "ios":
		homeURL = "http://localhost:" + device.WDAPort + "/wda/homescreen"
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.OS)
	}

	requestBody := ""
	if device.OS == "android" {
		requestBody = `{"keycode": 3}`
	}

	homeResponse, err := http.Post(homeURL, "application/json", bytes.NewBuffer([]byte(requestBody)))
	if err != nil {
		return nil, err
	}

	return homeResponse, nil
}

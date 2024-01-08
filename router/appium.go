package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/shamanec/GADS-devices-provider/models"
	"github.com/shamanec/GADS-devices-provider/util"
)

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

func appiumLockUnlock(device *models.Device, lock string) (*http.Response, error) {
	var deviceHomeURL string
	deviceHomeURL = "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/appium/device/" + lock

	req, err := http.NewRequest(http.MethodPost, deviceHomeURL, nil)
	if err != nil {
		return nil, err
	}

	lockResponse, err := netClient.Do(req)
	if err != nil {
		return nil, err
	}

	return lockResponse, nil
}

func appiumTap(device *models.Device, x float64, y float64) (*http.Response, error) {
	appiumRequestURL := "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/actions"

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

	actionJSON, err := util.ConvertToJSONString(action)
	if err != nil {
		return nil, fmt.Errorf("Could not convert Appium actions struct to a JSON string: %s", err)
	}

	req, err := http.NewRequest(http.MethodPost, appiumRequestURL, bytes.NewBuffer([]byte(actionJSON)))
	if err != nil {
		return nil, fmt.Errorf("Could not generate http request to Appium /actions endpoint: %s", err)
	}

	tapResponse, err := netClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed calling Appium /actions endpoint: %s", err)
	}

	return tapResponse, nil
}

func appiumTouchAndHold(device *models.Device, x float64, y float64) (*http.Response, error) {
	appiumRequestURL := "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/actions"

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
						Duration: 2000,
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
		return nil, fmt.Errorf("Could not convert Appium actions struct to a JSON string: %s", err)
	}

	req, err := http.NewRequest(http.MethodPost, appiumRequestURL, bytes.NewBuffer([]byte(actionJSON)))
	if err != nil {
		return nil, fmt.Errorf("Could not generate http request to Appium /actions endpoint: %s", err)
	}

	touchAndHoldResponse, err := netClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed calling Appium /actions endpoint: %s", err)
	}

	return touchAndHoldResponse, nil
}

func appiumSwipe(device *models.Device, x, y, endX, endY float64) (*http.Response, error) {
	appiumRequestURL := "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/actions"

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

	actionJSON, err := util.ConvertToJSONString(action)
	if err != nil {
		return nil, fmt.Errorf("Could not convert Appium actions struct to a JSON string: %s", err)
	}

	req, err := http.NewRequest(http.MethodPost, appiumRequestURL, bytes.NewBuffer([]byte(actionJSON)))
	if err != nil {
		return nil, fmt.Errorf("Could not generate http request to Appium /actions endpoint: %s", err)
	}

	swipeResponse, err := netClient.Do(req)
	if err != nil {
		return swipeResponse, fmt.Errorf("Failed calling Appium /actions endpoint: %s", err)
	}

	return swipeResponse, nil
}

func appiumSource(device *models.Device) (*http.Response, error) {
	sourceURL := "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/source"

	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Could not generate http request to Appium /source endpoint: %s", err)
	}

	sourceResponse, err := netClient.Do(req)
	if err != nil {
		return sourceResponse, fmt.Errorf("Failed calling Appium /source endpoint: %s", err)
	}

	return sourceResponse, nil
}

func appiumScreenshot(device *models.Device) (*http.Response, error) {
	screenshotURL := "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/screenshot"

	req, err := http.NewRequest(http.MethodGet, screenshotURL, nil)
	if err != nil {
		return nil, err
	}

	screenshotResponse, err := netClient.Do(req)
	if err != nil {
		return screenshotResponse, err
	}

	return screenshotResponse, nil
}

type ActiveElementData struct {
	Value struct {
		Element string `json:"ELEMENT"`
	} `json:"value"`
}

func appiumTypeText(device *models.Device, text string) (*http.Response, error) {
	activeElementRequestURL := "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/active"

	activeElReq, err := http.NewRequest(http.MethodGet, activeElementRequestURL, nil)
	if err != nil {
		return nil, err
	}

	activeElementResp, err := netClient.Do(activeElReq)
	if err != nil {
		return activeElementResp, err
	}

	// Read the response body
	activeElementRespBody, err := ioutil.ReadAll(activeElementResp.Body)
	if err != nil {
		return nil, err
	}

	var activeElementData ActiveElementData
	err = json.Unmarshal(activeElementRespBody, &activeElementData)
	if err != nil {
		return nil, err
	}

	activeElementID := activeElementData.Value.Element

	setValueRequestURL := "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/" + activeElementID + "/value"

	setValueRequestBody := `{"text":"` + text + `"}`

	setValueReq, err := http.NewRequest(http.MethodPost, setValueRequestURL, bytes.NewBuffer([]byte(setValueRequestBody)))
	if err != nil {
		return nil, err
	}

	setValueResponse, err := netClient.Do(setValueReq)
	if err != nil {
		return nil, err
	}

	return setValueResponse, nil
}

func appiumClearText(device *models.Device) (*http.Response, error) {
	activeElementRequestURL := "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/active"

	activeElReq, err := http.NewRequest(http.MethodGet, activeElementRequestURL, nil)
	if err != nil {
		return nil, err
	}

	activeElementResp, err := netClient.Do(activeElReq)
	if err != nil {
		return activeElementResp, err
	}

	activeElementRespBody, err := io.ReadAll(activeElementResp.Body)
	if err != nil {
		return nil, err
	}

	var activeElementData map[string]interface{}
	err = json.Unmarshal(activeElementRespBody, &activeElementData)
	if err != nil {
		return nil, err
	}

	activeElementID := activeElementData["value"].(map[string]interface{})["ELEMENT"].(string)

	clearValueRequestURL := "http://localhost:" + device.AppiumPort + "/session/" + device.AppiumSessionID + "/element/" + activeElementID + "/clear"

	clearValueReq, err := http.NewRequest(http.MethodPost, clearValueRequestURL, nil)
	if err != nil {
		return nil, err
	}

	clearValueResponse, err := netClient.Do(clearValueReq)
	if err != nil {
		return clearValueResponse, err
	}

	return clearValueResponse, nil
}

func appiumHome(device *models.Device) (*http.Response, error) {
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

	homeReq, err := http.NewRequest(http.MethodPost, homeURL, bytes.NewBuffer([]byte(requestBody)))
	if err != nil {
		return nil, err
	}

	homeResponse, err := netClient.Do(homeReq)
	if err != nil {
		return homeResponse, err
	}

	return homeResponse, nil
}

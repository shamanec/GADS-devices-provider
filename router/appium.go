package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shamanec/GADS-devices-provider/models"
)

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

func appiumRequest(device *models.Device, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/session/%s/%s", device.AppiumPort, device.AppiumSessionID, endpoint)
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return netClient.Do(req)
}

func wdaRequest(device *models.Device, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/%s", device.WDAPort, endpoint)
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return netClient.Do(req)
}

func appiumRequestNoSession(device *models.Device, method, endpoint string, requestBody io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%s/%s", device.AppiumPort, endpoint)
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	return netClient.Do(req)
}

func appiumLockUnlock(device *models.Device, lock string) (*http.Response, error) {
	endpoint := fmt.Sprintf("appium/device/%s", lock)
	return appiumRequest(device, http.MethodPost, endpoint, nil)
}

func appiumTap(device *models.Device, x float64, y float64) (*http.Response, error) {
	// Generate the struct object for the Appium actions JSON request
	action := models.DevicePointerActions{
		Actions: []models.DevicePointerAction{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: models.DeviceActionParameters{
					PointerType: "touch",
				},
				Actions: []models.DeviceAction{
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

	actionJSON, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return nil, err
	}

	return appiumRequest(device, http.MethodPost, "actions", bytes.NewReader(actionJSON))
}

func appiumTouchAndHold(device *models.Device, x float64, y float64) (*http.Response, error) {
	// Generate the struct object for the Appium actions JSON request
	action := models.DevicePointerActions{
		Actions: []models.DevicePointerAction{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: models.DeviceActionParameters{
					PointerType: "touch",
				},
				Actions: []models.DeviceAction{
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

	actionJSON, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return nil, err
	}

	return appiumRequest(device, http.MethodPost, "actions", bytes.NewReader(actionJSON))
}

func appiumSwipe(device *models.Device, x, y, endX, endY float64) (*http.Response, error) {
	// Generate the struct object for the Appium actions JSON request
	action := models.DevicePointerActions{
		Actions: []models.DevicePointerAction{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: models.DeviceActionParameters{
					PointerType: "touch",
				},
				Actions: []models.DeviceAction{
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

	actionJSON, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return nil, err
	}

	return appiumRequest(device, http.MethodPost, "actions", bytes.NewReader(actionJSON))
}

func appiumSource(device *models.Device) (*http.Response, error) {
	return appiumRequest(device, http.MethodGet, "source", nil)
}

func appiumScreenshot(device *models.Device) (*http.Response, error) {
	return appiumRequest(device, http.MethodGet, "screenshot", nil)
}

func appiumGetActiveElement(device *models.Device) (*http.Response, error) {
	return appiumRequest(device, http.MethodGet, "element/active", nil)
}

func getActiveElementID(resp *http.Response) (string, error) {
	activeElementRespBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var activeElementData models.ActiveElementData
	err = json.Unmarshal(activeElementRespBody, &activeElementData)
	if err != nil {
		return "", err
	}

	return activeElementData.Value.Element, nil
}

func appiumTypeText(device *models.Device, text string) (*http.Response, error) {
	activeElementResp, err := appiumGetActiveElement(device)
	if err != nil {
		return activeElementResp, err
	}

	activeElementID, err := getActiveElementID(activeElementResp)
	if err != nil {
		return nil, err
	}

	typeTextPayload := models.AppiumTypeText{
		Text: text,
	}

	typeJSON, err := json.MarshalIndent(typeTextPayload, "", "  ")
	if err != nil {
		return nil, err
	}

	return appiumRequest(device, http.MethodPost, fmt.Sprintf("element/%s/value", activeElementID), bytes.NewBuffer(typeJSON))
}

func appiumClearText(device *models.Device) (*http.Response, error) {
	activeElementResp, err := appiumGetActiveElement(device)
	if err != nil {
		return activeElementResp, err
	}

	activeElementID, err := getActiveElementID(activeElementResp)
	if err != nil {
		return nil, err
	}

	return appiumRequest(device, http.MethodPost, fmt.Sprintf("element/%s/clear", activeElementID), nil)
}

func appiumHome(device *models.Device) (*http.Response, error) {
	switch device.OS {
	case "android":
		requestBody := models.AndroidKeycodePayload{
			Keycode: 3,
		}

		typeJSON, err := json.MarshalIndent(requestBody, "", "  ")
		if err != nil {
			return nil, err
		}

		return appiumRequest(device, http.MethodPost, "appium/device/press_keycode", bytes.NewReader(typeJSON))
	case "ios":
		return wdaRequest(device, http.MethodPost, "wda/homescreen", nil)
	default:
		return nil, fmt.Errorf("Unsupported device OS: %s", device.OS)
	}
}

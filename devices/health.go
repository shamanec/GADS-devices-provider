package devices

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/shamanec/GADS-devices-provider/models"
	"github.com/shamanec/GADS-devices-provider/util"
)

// Check if a device is healthy by checking Appium and WebDriverAgent(for iOS) services
func GetDeviceHealth(device *models.LocalDevice) (bool, error) {
	err := checkAppiumSession(device)
	if err != nil {
		return false, err
	}

	return device.Device.Connected, nil
}

func checkAppiumSession(device *models.LocalDevice) error {
	response, err := http.Get("http://localhost:" + device.AppiumPort + "/sessions")
	if err != nil {
		device.AppiumSessionID = ""
		return err
	}
	responseBody, _ := io.ReadAll(response.Body)

	var responseJson AppiumGetSessionsResponse
	err = util.UnmarshalJSONString(string(responseBody), &responseJson)
	if err != nil {
		device.AppiumSessionID = ""
		return err
	}

	if len(responseJson.Value) == 0 {
		sessionID, err := createAppiumSession(device)
		if err != nil {
			device.AppiumSessionID = ""
			return err
		}
		device.AppiumSessionID = sessionID
		return nil
	}

	device.AppiumSessionID = responseJson.Value[0].ID
	return nil
}

func createAppiumSession(device *models.LocalDevice) (string, error) {
	var automationName = "UiAutomator2"
	var platformName = "Android"
	var waitForIdleTimeout = 10
	if device.Device.OS == "ios" {
		automationName = "XCUITest"
		platformName = "iOS"
		waitForIdleTimeout = 0
	}

	data := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"alwaysMatch": map[string]interface{}{
				"appium:automationName":     automationName,
				"platformName":              platformName,
				"appium:newCommandTimeout":  120,
				"appium:waitForIdleTimeout": waitForIdleTimeout,
			},
			"firstMatch": []map[string]interface{}{},
		},
		"desiredCapabilities": map[string]interface{}{
			"appium:automationName":     automationName,
			"platformName":              platformName,
			"appium:newCommandTimeout":  120,
			"appium:waitForIdleTimeout": waitForIdleTimeout,
		},
	}

	jsonString, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}

	response, err := http.Post("http://localhost:"+device.AppiumPort+"/session", "application/json", bytes.NewBuffer(jsonString))
	if err != nil {
		return "", err
	}

	responseBody, _ := io.ReadAll(response.Body)
	var responseJson AppiumCreateSessionResponse
	err = util.UnmarshalJSONString(string(responseBody), &responseJson)
	if err != nil {
		return "", err
	}

	return responseJson.Value.SessionID, nil
}

type AppiumGetSessionsResponse struct {
	Value []struct {
		ID string `json:"id"`
	} `json:"value"`
}

type AppiumCreateSessionResponse struct {
	Value struct {
		SessionID string `json:"sessionId"`
	} `json:"value"`
}

package device

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shamanec/GADS-devices-provider/util"
)

// Check if a device is healthy by checking Appium and WebDriverAgent(for iOS) services
func GetDeviceHealth(udid string) (bool, error) {
	device := GetDeviceByUDID(udid)

	allGood := false

	if Config.AppiumConfig.UseAppium {
		allGood, err := device.appiumHealthy()
		if err != nil {
			return false, err
		}

		if allGood {
			err = device.checkAppiumSession()
			if err != nil {
				return false, err
			}
		}
	}

	var err error
	if device.OS == "ios" {
		allGood, err = device.wdaHealthy()
		if err != nil {
			return false, err
		}
		if allGood {
			err = device.checkWDASession()
			if err != nil {
				return false, err
			}
		}
	}

	return allGood, nil
}

// Check if the Appium server for a device is up
func (device *Device) appiumHealthy() (bool, error) {
	response, err := http.Get("http://localhost:" + device.AppiumPort + "/status")
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	responseCode := response.StatusCode

	if responseCode != 200 {
		return false, nil
	}

	return true, nil
}

// Check if the WebDriverAgent server for an iOS device is up
func (device *Device) wdaHealthy() (bool, error) {
	response, err := http.Get("http://localhost:" + device.WDAPort + "/status")
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	responseCode := response.StatusCode
	if responseCode != 200 {
		return false, nil
	}

	return true, nil
}

func (device *Device) checkAppiumSession() error {
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
		sessionID, err := device.createAppiumSession()
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

func (device *Device) createAppiumSession() (string, error) {
	requestString := `{
		"capabilities": {
			"alwaysMatch": {
				"appium:automationName": "UiAutomator2",
				"platformName": "Android",
				"appium:ensureWebviewsHavePages": true,
				"appium:nativeWebScreenshot": true,
				"appium:newCommandTimeout": 0,
				"appium:connectHardwareKeyboard": true
			},
			"firstMatch": [
				{}
			]
		},
		"desiredCapabilities": {
			"appium:automationName": "UiAutomator2",
			"platformName": "Android",
			"appium:ensureWebviewsHavePages": true,
			"appium:nativeWebScreenshot": true,
			"appium:newCommandTimeout": 0,
			"appium:connectHardwareKeyboard": true
		}
	}`

	response, err := http.Post("http://localhost:"+device.AppiumPort+"/session", "application/json", strings.NewReader(requestString))
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

func (device *Device) checkWDASession() error {
	response, err := http.Get("http://localhost:" + device.WDAPort + "/status")
	if err != nil {
		return err
	}

	responseBody, _ := io.ReadAll(response.Body)

	var responseJson map[string]interface{}
	err = json.Unmarshal(responseBody, &responseJson)
	if err != nil {
		device.WDASessionID = ""
		return err
	}

	if responseJson["sessionId"] == "" || responseJson["sessionId"] == nil {
		sessionId, err := device.createWDASession()
		if err != nil {
			device.WDASessionID = ""
			return err
		}

		if sessionId == "" {
			device.WDASessionID = ""
			return err
		}
	}

	device.WDASessionID = fmt.Sprintf("%v", responseJson["sessionId"])
	return nil
}

func (device *Device) createWDASession() (string, error) {
	requestString := `{
		"capabilities": {
			"firstMatch": [
				{
					"arguments": [],
					"environment": {},
					"eventloopIdleDelaySec": 0,
					"shouldWaitForQuiescence": true,
					"shouldUseTestManagerForVisibilityDetection": false,
					"maxTypingFrequency": 60,
					"shouldUseSingletonTestManager": true,
					"shouldTerminateApp": true,
					"forceAppLaunch": true,
					"useNativeCachingStrategy": true,
					"forceSimulatorSoftwareKeyboardPresence": false
				}
			],
			"alwaysMatch": {}
		}
	}`

	response, err := http.Post("http://localhost:"+device.WDAPort+"/session", "application/json", strings.NewReader(requestString))
	if err != nil {
		return "", err
	}

	responseBody, _ := io.ReadAll(response.Body)

	var responseJson map[string]interface{}
	err = json.Unmarshal(responseBody, &responseJson)
	if err != nil {
		return "", err
	}

	if responseJson["sessionId"] == "" || responseJson["sessionId"] == nil {
		if err != nil {
			return "", errors.New("Could not get `sessionId` while creating a new WebDriverAgent session")
		}
	}

	return fmt.Sprintf("%v", responseJson["sessionId"]), nil
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

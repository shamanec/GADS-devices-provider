package router

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/shamanec/GADS-devices-provider/device"
	"github.com/shamanec/GADS-devices-provider/util"
)

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

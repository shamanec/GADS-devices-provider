package device

import (
	"net/http"
)

// Check if a device is healthy by checking Appium and WebDriverAgent(for iOS) services
func GetDeviceHealth(udid string) (bool, error) {
	device := GetDeviceByUDID(udid)

	allGood := false
	allGood, err := device.appiumHealthy()
	if err != nil {
		return false, err
	}

	if device.OS == "ios" {
		allGood, err = device.wdaHealthy()
		if err != nil {
			return false, err
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

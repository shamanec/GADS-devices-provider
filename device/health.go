package device

import (
	"net/http"
)

func (device *Device) appiumHealthy() (bool, error) {
	response, err := http.Get("http://localhost:" + device.AppiumPort + "/status")
	if err != nil {
		return false, err
	}

	responseCode := response.StatusCode
	if responseCode != 200 {
		return false, nil
	}

	return true, nil
}

func (device *Device) wdaHealthy() (bool, error) {
	response, err := http.Get("http://localhost:" + device.WDAPort + "/status")
	if err != nil {
		return false, err
	}

	responseCode := response.StatusCode
	if responseCode != 200 {
		return false, nil
	}

	return true, nil
}

func GetDeviceHealth(udid string) (bool, error) {
	device := getDeviceByUDID(udid)

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

func getDeviceByUDID(udid string) *Device {
	for _, device := range Config.Devices {
		if device.UDID == udid {
			return device
		}
	}

	return nil
}

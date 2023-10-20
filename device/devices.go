package device

import (
	"runtime"
)

var DeviceMap = make(map[string]*LocalDevice)

func UpdateDevices() {
	Setup()

	if runtime.GOOS == "linux" {
		go updateDevicesLinux()
	} else if runtime.GOOS == "darwin" {
		go updateDevicesOSX()
	} else if runtime.GOOS == "windows" {
		go updateDevicesWindows()
	}
	go updateDevicesMongo()
}

// Create Mongo collections for all devices for logging
// Create a map of *device.LocalDevice for easier access across the code
func Setup() {
	getLocalDevices()
	createMongoLogCollectionsForAllDevices()
	createDeviceMap()
}

func createDeviceMap() {
	for _, device := range localDevices {
		DeviceMap[device.Device.UDID] = device
	}
}

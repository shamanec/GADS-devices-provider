package device

import (
	"runtime"
)

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

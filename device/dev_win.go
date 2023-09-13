package device

import (
	"os"
	"slices"
	"time"

	"github.com/shamanec/GADS-devices-provider/util"
)

func updateDevicesWindows() {
	util.LogInfo("provider", "Providing devices on a Windows host")

	androidDevicesInConfig := androidDevicesInConfig()

	if androidDevicesInConfig {
		util.LogInfo("provider", "There are Android devices in config, checking if adb is available on host")

		if !adbAvailable() {
			util.LogError("provider", "adb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	getLocalDevices()
	removeAdbForwardedPorts()

	for {
		connectedDevices := getConnectedDevicesCommon(false, true)

		if len(connectedDevices) == 0 {
			util.LogDebug("provider", "No connected devices found when updating devices")

			for _, device := range localDevices {
				device.Device.Connected = false
				// device.resetLocalDevice()
			}
		} else {
			for _, device := range localDevices {
				if slices.Contains(connectedDevices, device.Device.UDID) {
					device.Device.Connected = true
					if device.ProviderState != "preparing" && device.ProviderState != "live" {
						device.setContext()
						if device.Device.OS == "android" {
							go device.setupAndroidDevice()
						}
					}
					continue
				}
				device.Device.Connected = false
			}
		}
		time.Sleep(10 * time.Second)
	}
}

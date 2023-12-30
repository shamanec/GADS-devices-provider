package device

import (
	"os"
	"slices"
	"time"

	"github.com/shamanec/GADS-devices-provider/config"
	"github.com/shamanec/GADS-devices-provider/logger"
)

func updateDevicesWindows() {
	logger.ProviderLogger.LogInfo("provider", "Providing devices on a Windows host")

	if config.Config.EnvConfig.ProvideAndroid {
		logger.ProviderLogger.LogInfo("provider", "There are Android devices in config, checking if adb is available on host")

		if !adbAvailable() {
			logger.ProviderLogger.LogError("provider", "adb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	removeAdbForwardedPorts()

	for {
		connectedDevices := getConnectedDevicesCommon()

		if len(connectedDevices) == 0 {
			logger.ProviderLogger.LogDebug("provider", "No connected devices found when updating devices")

			for _, device := range localDevices {
				device.Device.Connected = false
				// device.resetLocalDevice()
			}
		} else {
			for _, device := range localDevices {
				if slices.Contains(connectedDevices, device.Device.UDID) {
					device.Device.Connected = true
					if device.ProviderState != "preparing" && device.ProviderState != "live" {
						setContext(device)
						if device.Device.OS == "android" {
							go setupAndroidDevice(device)
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

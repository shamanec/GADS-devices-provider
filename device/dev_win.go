package device

import (
	"fmt"
	"os"
	"slices"
	"time"

	log "github.com/sirupsen/logrus"
)

func updateDevicesWindows() {
	log.WithFields(log.Fields{
		"event": "provider",
	}).Info("Updating devices on a Windows host")

	androidDevicesInConfig := androidDevicesInConfig()

	if androidDevicesInConfig {
		log.WithFields(log.Fields{
			"event": "provider",
		}).Info("There are Android devices in config, checking if adb is available on host")

		if !adbAvailable() {
			fmt.Println("adb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	getLocalDevices()
	removeAdbForwardedPorts()

	for {
		connectedDevices := getConnectedDevicesCommon(false, true)

		if len(connectedDevices) == 0 {
			log.WithFields(log.Fields{
				"event": "update_devices",
			}).Info("No devices connected")

			for _, device := range localDevices {
				device.Device.Connected = false
				device.resetLocalDevice()
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

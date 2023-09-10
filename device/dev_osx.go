package device

import (
	"fmt"
	"os"
	"slices"
	"time"

	log "github.com/sirupsen/logrus"
)

func updateDevicesOSX() {
	// Create common logs directory if it doesn't already exist
	if _, err := os.Stat("./logs"); os.IsNotExist(err) {
		os.Mkdir("./logs", os.ModePerm)
	}

	if !xcodebuildAvailable() {
		fmt.Println("xcodebuild is not available, you need to set up the host as explained in the readme")
		os.Exit(1)
	}

	androidDevicesInConfig := androidDevicesInConfig()

	if androidDevicesInConfig {
		if !adbAvailable() {
			fmt.Println("adb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	_, err := os.Stat(Config.EnvConfig.WDAPath)
	if err != nil {
		fmt.Println(Config.EnvConfig.WDAPath + " does not exist, you need to provide valid path to the WebDriverAgent repo in config.json")
		os.Exit(1)
	}

	err = buildWebDriverAgent()
	if err != nil {
		fmt.Println("Could not successfully build WebDriverAgent for testing - " + err.Error())
		os.Exit(1)
	}

	getLocalDevices()
	removeAdbForwardedPorts()

	for {
		connectedDevices := getConnectedDevicesCommon(true, androidDevicesInConfig)

		if len(connectedDevices) == 0 {
			log.WithFields(log.Fields{
				"event": "update_devices",
			}).Info("No devices connected")

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
						if device.Device.OS == "ios" {
							device.WdaReadyChan = make(chan bool, 1)
							go device.setupIOSDevice()
						}

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

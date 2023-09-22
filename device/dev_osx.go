package device

import (
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/shamanec/GADS-devices-provider/util"
)

func updateDevicesOSX() {
	// Create common logs directory if it doesn't already exist
	if _, err := os.Stat("./logs"); os.IsNotExist(err) {
		os.Mkdir("./logs", os.ModePerm)
	}

	if !xcodebuildAvailable() {
		util.ProviderLogger.LogError("provider", "xcodebuild is not available, you need to set up the host as explained in the readme")
		fmt.Println("xcodebuild is not available, you need to set up the host as explained in the readme")
		os.Exit(1)
	}

	androidDevicesInConfig := androidDevicesInConfig()

	if androidDevicesInConfig {
		if !adbAvailable() {
			util.ProviderLogger.LogError("provider", "adb is not available, you need to set up the host as explained in the readme")
			fmt.Println("adb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	_, err := os.Stat(Config.EnvConfig.WDAPath)
	if err != nil {
		util.ProviderLogger.LogError("provider", Config.EnvConfig.WDAPath+" does not exist, you need to provide valid path to the WebDriverAgent repo in config.json")
		fmt.Println(Config.EnvConfig.WDAPath + " does not exist, you need to provide valid path to the WebDriverAgent repo in config.json")
		os.Exit(1)
	}

	err = buildWebDriverAgent()
	if err != nil {
		util.ProviderLogger.LogError("provider", fmt.Sprintf("Could not successfully build WebDriverAgent for testing - %s", err))
		fmt.Println("Could not successfully build WebDriverAgent for testing - " + err.Error())
		os.Exit(1)
	}

	getLocalDevices()
	removeAdbForwardedPorts()

	for {
		connectedDevices := getConnectedDevicesCommon(true, androidDevicesInConfig)

		if len(connectedDevices) == 0 {
			util.ProviderLogger.LogDebug("provider", "No devices connected")

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

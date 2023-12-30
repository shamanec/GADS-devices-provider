package devices

import (
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/shamanec/GADS-devices-provider/config"
	"github.com/shamanec/GADS-devices-provider/logger"
	"github.com/shamanec/GADS-devices-provider/models"
	"github.com/shamanec/GADS-devices-provider/util"
)

func updateDevicesLinux() {
	// Create common logs directory if it doesn't already exist
	if _, err := os.Stat("./logs"); os.IsNotExist(err) {
		os.Mkdir("./logs", os.ModePerm)
	}

	if config.Config.EnvConfig.ProvideAndroid {
		if !adbAvailable() {
			logger.ProviderLogger.LogError("provider", "adb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	if config.Config.EnvConfig.ProvideIOS {
		if !goIOSAvailable() {
			logger.ProviderLogger.LogError("provider", "go-ios is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	removeAdbForwardedPorts()

	for {
		connectedDevices := getConnectedDevicesCommon()

		for _, device := range localDevices {
			if slices.Contains(connectedDevices, device.Device.UDID) {
				device.Device.Connected = true
				if device.ProviderState != "preparing" && device.ProviderState != "live" {
					setContext(device)
					if device.Device.OS == "ios" {
						device.WdaReadyChan = make(chan bool, 1)
						go setupIOSDeviceGoIOS(device)
					}

					if device.Device.OS == "android" {
						go setupAndroidDevice(device)
					}
				}
				continue
			} else {
				if device.Device.Connected {
					resetLocalDevice(device)
				}
				device.Device.Connected = false
			}
		}
		time.Sleep(10 * time.Second)
	}

}

func setupIOSDeviceGoIOS(device *models.LocalDevice) {
	device.ProviderState = "preparing"
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Running setup for device `%v`", device.Device.UDID))

	// Get go-ios device entry for pairing/mounting images
	getGoIOSDevice(device)

	// Get a free port on the host for WebDriverAgent server
	wdaPort, err := util.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent port for device `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.Device.WDAPort = fmt.Sprint(wdaPort)

	// Get a free port on the host for WebDriverAgent stream
	streamPort, err := util.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent stream port for device `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.Device.StreamPort = fmt.Sprint(streamPort)

	// Forward the WebDriverAgent server and stream to the host
	go goIOSForward(device, device.Device.WDAPort, "8100")
	go goIOSForward(device, device.Device.StreamPort, "9100")

	err = pairIOS(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not pair device `%s` - %s", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}

	err = mountDeveloperImageIOS(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not mount developer disk images on device `%s` - %s", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}

	err = InstallAppWithDevice(device, "WebDriverAgent.ipa")
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not install WebDriverAgent on device `%s` - %s", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}

	go startWdaWithGoIOS(device)

	// Wait until WebDriverAgent successfully starts
	select {
	case <-device.WdaReadyChan:
		logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Successfully started WebDriverAgent for device `%v` forwarded on port %v", device.Device.UDID, device.Device.WDAPort))
		break
	case <-time.After(30 * time.Second):
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully start WebDriverAgent on device `%v` in 30 seconds", device.Device.UDID))
		resetLocalDevice(device)
		return
	}

	// Create a WebDriverAgent session and update the MJPEG stream settings
	err = updateWebDriverAgent(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not update WebDriverAgent stream settings for device `%s`, device setup will NOT be aborted - %s", device.Device.UDID, err))
	}

	go startAppium(device)
	if config.Config.EnvConfig.UseSeleniumGrid {
		go startGridNode(device)
	}

	device.Device.InstalledApps = getInstalledAppsIOS(device)

	// Mark the device as 'live'
	device.ProviderState = "live"
}

package device

import (
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/shamanec/GADS-devices-provider/util"
	log "github.com/sirupsen/logrus"
)

func updateDevicesLinux() {
	// Create common logs directory if it doesn't already exist
	if _, err := os.Stat("./logs"); os.IsNotExist(err) {
		os.Mkdir("./logs", os.ModePerm)
	}

	androidDevicesInConfig := androidDevicesInConfig()

	if androidDevicesInConfig {
		if !adbAvailable() {
			util.ProviderLogger.LogError("provider", "adb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	iOSDevicesInConfig := iOSDevicesInConfig()
	if iOSDevicesInConfig {
		if !goIOSAvailable() {
			util.ProviderLogger.LogError("provider", "go-ios is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

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
							go device.setupIOSDeviceGoIOS()
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

func (device *LocalDevice) setupIOSDeviceGoIOS() {
	device.ProviderState = "preparing"
	util.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Running setup for device `%v`", device.Device.UDID))

	// Get go-ios device entry for pairing/mounting images
	// Mounting currently unused, images are mounted automatically through Xcode device setup
	// Pairing currently unused, TODO after go-ios supports iOS >=17
	device.getGoIOSDevice()

	// Get a free port on the host for WebDriverAgent server
	wdaPort, err := util.GetFreePort()
	if err != nil {
		util.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent port for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
	device.Device.WDAPort = fmt.Sprint(wdaPort)

	// Get a free port on the host for WebDriverAgent stream
	streamPort, err := util.GetFreePort()
	if err != nil {
		util.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent stream port for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
	device.Device.StreamPort = fmt.Sprint(streamPort)

	// Forward the WebDriverAgent server and stream to the host
	go device.goIOSForward(device.Device.WDAPort, "8100")
	go device.goIOSForward(device.Device.StreamPort, "9100")

	device.pairIOS()
	device.mountDeveloperImageIOS()

	InstallAppWithDevice(device.GoIOSDeviceEntry, "WebDriverAgent.ipa")

	go device.startWdaWithGoIOS()

	// Wait until WebDriverAgent successfully starts
	select {
	case <-device.WdaReadyChan:
		util.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Successfully started WebDriverAgent for device `%v` forwarded on port %v", device.Device.UDID, device.Device.WDAPort))
		break
	case <-time.After(30 * time.Second):
		util.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully start WebDriverAgent on device `%v` in 30 seconds", device.Device.UDID))
		device.resetLocalDevice()
		return
	}

	// Create a WebDriverAgent session and update the MJPEG stream settings
	err = device.updateWebDriverAgent()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error(fmt.Sprintf("Did not successfully create WebDriverAgent session or update its stream settings for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	go device.startAppium()

	// Start a goroutine that periodically checks if the WebDriverAgent server is up
	go device.updateDeviceHealthStatus()

	// Mark the device as 'live' and update it in RethinkDB
	device.ProviderState = "live"
}

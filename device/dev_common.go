package device

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/shamanec/GADS-devices-provider/util"
	log "github.com/sirupsen/logrus"
)

var mu sync.Mutex
var netClient = &http.Client{
	Timeout: time.Second * 120,
}
var localDevices []*LocalDevice

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

// DEVICES SETUP

// Read the devices from Config and create a new slice with "upgraded" LocalDevices that contain fields just for the local setup
func getLocalDevices() {
	for _, device := range util.Config.Devices {
		localDevice := LocalDevice{
			Device:        device,
			ProviderState: "init",
			IsResetting:   false,
		}
		localDevice.setContext()
		localDevice.Device.HostAddress = util.Config.EnvConfig.HostAddress
		localDevice.Device.Provider = util.Config.EnvConfig.ProviderNickname
		localDevices = append(localDevices, &localDevice)

		// Create logs directory for each device if it doesn't already exist
		if _, err := os.Stat("./logs/device_" + device.UDID); os.IsNotExist(err) {
			os.Mkdir("./logs/device_"+device.UDID, os.ModePerm)
		}
	}
}

func (device *LocalDevice) setupAndroidDevice() {
	device.ProviderState = "preparing"

	util.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Running setup for device `%v`", device.Device.UDID))

	isStreamAvailable, err := device.isGadsStreamServiceRunning()
	if err != nil {
		util.ProviderLogger.LogError("provider", fmt.Sprintf("Could not check if GADS-stream is running on device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	// Get a free port on the host for WebDriverAgent server
	streamPort, err := util.GetFreePort()
	if err != nil {
		util.ProviderLogger.LogError("provider", fmt.Sprintf("Could not allocate free host port for GADS-stream for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
	device.Device.StreamPort = fmt.Sprint(streamPort)

	if !isStreamAvailable {
		err = device.installGadsStream()
		if err != nil {
			util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not install GADS-stream on Android device - %v:\n %v", device.Device.UDID, err))
			device.resetLocalDevice()
			return
		}

		err = device.addGadsStreamRecordingPermissions()
		if err != nil {
			util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not set GADS-stream recording permissions on Android device - %v:\n %v", device.Device.UDID, err))
			device.resetLocalDevice()
			return
		}

		err = device.startGadsStreamApp()
		if err != nil {
			util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not start GADS-stream app on Android device - %v:\n %v", device.Device.UDID, err))
			device.resetLocalDevice()
			return
		}

		device.pressHomeButton()
	}

	err = device.forwardGadsStream()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS-stream port to host port %v for Android device - %v:\n %v", device.Device.StreamPort, device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	go device.startAppium()
	go device.updateDeviceHealthStatus()
}

func (device *LocalDevice) setupIOSDevice() {
	device.ProviderState = "preparing"
	util.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Running setup for device `%v`", device.Device.UDID))

	// Get go-ios device entry for pairing/mounting images
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

	// Start WebDriverAgent with `xcodebuild`
	go device.startWdaWithXcodebuild()

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

// COMMON

// Gets all connected iOS and Android devices to the host
func getConnectedDevicesCommon(ios bool, android bool) []string {
	log.WithFields(log.Fields{
		"event": "provider",
	}).Debug("Getting connected devices to host")

	connectedDevices := []string{}

	androidDevices := []string{}
	iosDevices := []string{}

	if android {
		androidDevices = getConnectedDevicesAndroid()
	}

	if ios {
		iosDevices = getConnectedDevicesIOS()
	}

	connectedDevices = append(connectedDevices, iosDevices...)
	connectedDevices = append(connectedDevices, androidDevices...)

	return connectedDevices
}

// Gets the connected iOS devices using the `go-ios` library
func getConnectedDevicesIOS() []string {
	var connectedDevices []string

	deviceList, err := ios.ListDevices()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "provider",
		}).Warn("Could not get connected iOS devices with `go-ios` library, returning empty slice - " + err.Error())
		return connectedDevices
	}

	for _, connDevice := range deviceList.DeviceList {
		if !slices.Contains(connectedDevices, connDevice.Properties.SerialNumber) {
			connectedDevices = append(connectedDevices, connDevice.Properties.SerialNumber)
		}
	}
	return connectedDevices
}

// Gets the connected android devices using `adb`
func getConnectedDevicesAndroid() []string {
	var connectedDevices []string

	cmd := exec.Command("adb", "devices")
	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "provider",
		}).Debug("Could not get connected Android devices with `adb`, creating exec cmd StdoutPipe failed, returning empty slice - " + err.Error())
		return connectedDevices
	}

	if err := cmd.Start(); err != nil {
		log.WithFields(log.Fields{
			"event": "provider",
		}).Debug("Could not get connected Android devices with `adb`, starting command failed, returning empty slice - " + err.Error())
		return connectedDevices
	}

	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "List of devices") && line != "" && strings.Contains(line, "device") {
			connectedDevices = append(connectedDevices, strings.Fields(line)[0])
		}
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		log.WithFields(log.Fields{
			"event": "provider",
		}).Debug("Could not get connected Android devices with `adb`, waiting for command to finish failed, returning empty slice - " + err.Error())
		return []string{}
	}
	return connectedDevices
}

func androidDevicesInConfig() bool {
	for _, device := range localDevices {
		if device.Device.OS == "android" {
			return true
		}
	}
	return false
}

func iOSDevicesInConfig() bool {
	for _, device := range localDevices {
		if device.Device.OS == "ios" {
			return true
		}
	}
	return false
}

func (device *LocalDevice) resetLocalDevice() {

	if !device.IsResetting {
		mu.Lock()
		device.IsResetting = true
		mu.Unlock()

		device.CtxCancel()
		mu.Lock()
		device.ProviderState = "init"
		device.Device.Healthy = false
		device.IsResetting = false
		mu.Unlock()
	}

}

// Set a context for a device to enable cancelling running goroutines related to that device when its disconnected
func (device *LocalDevice) setContext() {
	ctx, cancelFunc := context.WithCancel(context.Background())
	device.CtxCancel = cancelFunc
	device.Context = ctx
}

// HEALTH

// Loops checking if the Appium/WebDriverAgent servers for the device are alive and updates the DB each time
func (device *LocalDevice) updateDeviceHealthStatus() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	util.ProviderLogger.LogInfo("device_setup", fmt.Sprintf("Started health status check for device `%v`", device.Device.UDID))

	for {
		select {
		case <-ticker.C:
			device.checkDeviceHealthStatus()
		case <-device.Context.Done():
			return
		}
	}
}

// Checks Appium/WebDriverAgent servers are alive for the respective device
// Also updates Appium/WebDriverAgent sessions
// TODO - Currently unfinished, does not really check Appium for iOS/Android right now. Need to check if it can be unified with the health status endpoint code
func (device *LocalDevice) checkDeviceHealthStatus() {
	allGood := false
	allGood, err := device.appiumHealthy()
	if err != nil {
		device.Device.Healthy = false
	}

	if allGood {
		err = device.checkAppiumSession()
		if err != nil {
			device.Device.Healthy = false
		}
	}

	if device.Device.OS == "ios" {
		allGood, err = device.wdaHealthy()
		if err != nil {
			device.Device.Healthy = false
		}
	}

	device.Device.LastHealthyTimestamp = time.Now().UnixMilli()
	device.Device.Healthy = true
}

// APPIUM

type appiumCapabilities struct {
	UDID                  string `json:"appium:udid"`
	WdaMjpegPort          string `json:"appium:mjpegServerPort,omitempty"`
	ClearSystemFiles      string `json:"appium:clearSystemFiles,omitempty"`
	WdaURL                string `json:"appium:webDriverAgentUrl,omitempty"`
	PreventWdaAttachments string `json:"appium:preventWDAAttachments,omitempty"`
	SimpleIsVisibleCheck  string `json:"appium:simpleIsVisibleCheck,omitempty"`
	WdaLocalPort          string `json:"appium:wdaLocalPort,omitempty"`
	PlatformVersion       string `json:"appium:platformVersion,omitempty"`
	AutomationName        string `json:"appium:automationName"`
	PlatformName          string `json:"platformName"`
	DeviceName            string `json:"appium:deviceName"`
	WdaLaunchTimeout      string `json:"appium:wdaLaunchTimeout,omitempty"`
	WdaConnectionTimeout  string `json:"appium:wdaConnectionTimeout,omitempty"`
}

func (device *LocalDevice) startAppium() {
	// Create a usbmuxd.log file for Stderr
	appiumLogger, _ := util.CreateCustomLogger("./logs/device_"+device.Device.UDID+"/appium.log", device.Device.UDID)

	var capabilities appiumCapabilities

	if device.Device.OS == "ios" {
		capabilities = appiumCapabilities{
			UDID:                  device.Device.UDID,
			WdaURL:                "http://localhost:" + device.Device.WDAPort,
			WdaMjpegPort:          device.Device.StreamPort,
			WdaLocalPort:          device.Device.WDAPort,
			WdaLaunchTimeout:      "120000",
			WdaConnectionTimeout:  "240000",
			ClearSystemFiles:      "false",
			PreventWdaAttachments: "true",
			SimpleIsVisibleCheck:  "false",
			AutomationName:        "XCUITest",
			PlatformName:          "iOS",
			DeviceName:            device.Device.Name,
		}
	} else if device.Device.OS == "android" {
		capabilities = appiumCapabilities{
			UDID:           device.Device.UDID,
			AutomationName: "UiAutomator2",
			PlatformName:   "Android",
			DeviceName:     device.Device.Name,
		}
	}

	capabilitiesJson, _ := json.Marshal(capabilities)

	// Create a usbmuxd.log file for Stderr
	appiumLog, err := os.Create("./logs/device_" + device.Device.UDID + "/appium.log")
	if err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not create appium.log file for device - %v, err - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
	defer appiumLog.Close()

	// Get a free port on the host for Appium server
	appiumPort, err := util.GetFreePort()
	if err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not allocate free Appium host port for device - %v, err - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
	device.Device.AppiumPort = fmt.Sprint(appiumPort)

	cmd := exec.CommandContext(device.Context, "appium", "-p", device.Device.AppiumPort, "--log-timestamp", "--allow-cors", "--default-capabilities", string(capabilitiesJson))

	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error creating stdoutpipe while running WebDriverAgent with xcodebuild for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	if err := cmd.Start(); err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not start WebDriverAgent with xcodebuild for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		appiumLogger.LogInfo("appium", strings.TrimSpace(line))
	}

	if err := cmd.Wait(); err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error waiting for Appium command to finish, it errored out or device `%v` was disconnected - %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

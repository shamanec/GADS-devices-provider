package device

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/shamanec/GADS-devices-provider/util"
	log "github.com/sirupsen/logrus"
)

var usedPorts = make(map[int]bool)
var mu sync.Mutex
var netClient = &http.Client{
	Timeout: time.Second * 120,
}
var localDevices []*LocalDevice

// DEVICES SETUP

// Read the devices from Config and create a new slice with "upgraded" LocalDevices that contain fields just for the local setup
func getLocalDevices() {
	for _, device := range util.Config.Devices {
		localDevice := LocalDevice{
			Device:        device,
			ProviderState: "init",
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
	}

	// Get a free port on the host for WebDriverAgent server
	streamPort, err := getFreePort()
	if err != nil {
		util.ProviderLogger.LogError("provider", fmt.Sprintf("Could not allocate free host port for GADS-stream for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
	device.Device.StreamPort = fmt.Sprint(streamPort)

	if !isStreamAvailable {
		device.installGadsStream()
		device.addGadsStreamRecordingPermissions()
		device.startGadsStreamApp()
		device.pressHomeButton()
	}

	device.forwardGadsStream()

	go device.startAppium()
	go device.updateDeviceHealthStatus()
}

func (device *LocalDevice) setupIOSDevice() {
	device.ProviderState = "preparing"
	util.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Running setup for device `%v`", device.Device.UDID))

	// Get go-ios device entry for pairing/mounting images
	// Mounting currently unused, images are mounted automatically through Xcode device setup
	// Pairing currently unused, TODO after go-ios supports iOS >=17
	device.getGoIOSDevice()

	// Get a free port on the host for WebDriverAgent server
	wdaPort, err := getFreePort()
	if err != nil {
		util.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent port for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
	device.Device.WDAPort = fmt.Sprint(wdaPort)

	// Get a free port on the host for WebDriverAgent stream
	streamPort, err := getFreePort()
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

// Check if xcodebuild is available on the host by checking its version
func xcodebuildAvailable() bool {
	cmd := exec.Command("xcodebuild", "-version")
	log.WithFields(log.Fields{
		"event": "provider",
	}).Debug("Checking if xcodebuild is available on host")

	if err := cmd.Run(); err != nil {
		log.WithFields(log.Fields{
			"event": "provider",
		}).Warn("xcodebuild is not available or command failed - " + err.Error())
		return false
	}
	return true
}

// Check if adb is available on the host by starting the server
func adbAvailable() bool {
	cmd := exec.Command("adb", "start-server")
	log.WithFields(log.Fields{
		"event": "provider",
	}).Debug("Checking if adb is available on host")

	if err := cmd.Run(); err != nil {
		log.WithFields(log.Fields{
			"event": "provider",
		}).Warn("adb is not available or command failed - " + err.Error())
		return false
	}
	return true
}

func goIOSAvailable() bool {
	cmd := exec.Command("ios", "list")
	log.WithFields(log.Fields{
		"event": "provider",
	}).Debug("Checking if go-ios is available on host")

	if err := cmd.Run(); err != nil {
		log.WithFields(log.Fields{
			"event": "provider",
		}).Warn("go-ios is not available or command failed - " + err.Error())
		return false
	}
	return true
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
	mu.Lock()
	defer mu.Unlock()

	util.ProviderLogger.LogDebug("provider", fmt.Sprintf("Resetting LocalDevice for device `%v` after error. Cancelling context, setting ProviderState to `init`, Healthy to `false` and updating the DB", device.Device.UDID))

	device.CtxCancel()
	device.ProviderState = "init"
	device.Device.Healthy = false
}

// Set a context for a device to enable cancelling running goroutines related to that device when its disconnected
func (device *LocalDevice) setContext() {
	ctx, cancelFunc := context.WithCancel(context.Background())
	device.CtxCancel = cancelFunc
	device.Context = ctx
}

// ANDROID DEVICES

// Remove all adb forwarded ports(if any) on provider start
func removeAdbForwardedPorts() {
	util.ProviderLogger.LogDebug("provider", "Attempting to remove all `adb` forwarded ports on provider start")

	cmd := exec.Command("adb", "forward", "--remove-all")
	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogWarn("provider", "Could not remove `adb` forwarded ports, there was an error or no devices are connected - "+err.Error())
	}
}

func (device *LocalDevice) isGadsStreamServiceRunning() (bool, error) {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "dumpsys", "activity", "services", "com.shamanec.stream/.ScreenCaptureService")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Checking if GADS-stream is already running on Android device - %v", device.Device.UDID))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}

	// If command returned "(nothing)" then the service is not running
	if strings.Contains(string(output), "(nothing)") {
		return false, nil
	}

	return true, nil
}

// Install gads-stream.apk on the device
func (device *LocalDevice) installGadsStream() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "install", "-r", "./apps/gads-stream.apk")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Installing GADS-stream apk on Android device - %v", device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not install GADS-stream on Android device - %v:\n %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

// Add recording permissions to gads-stream app to avoid popup on start
func (device *LocalDevice) addGadsStreamRecordingPermissions() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "appops", "set", "com.shamanec.stream", "PROJECT_MEDIA", "allow")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Adding GADS-stream recording permissions on Android device - %v", device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not set GADS-stream recording permissions on Android device - %v:\n %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

// Start the gads-stream app using adb
func (device *LocalDevice) startGadsStreamApp() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "am", "start", "-n", "com.shamanec.stream/com.shamanec.stream.ScreenCaptureActivity")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Starting GADS-stream app on Android device - %v - with command - `%v`", device.Device.UDID, cmd.Path))

	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not start GADS-stream app on Android device - %v:\n %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

// Press the Home button using adb to hide the transparent gads-stream activity
func (device *LocalDevice) pressHomeButton() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "input", "keyevent", "KEYCODE_HOME")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Pressing Home button with adb on Android device - %v", device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not 'press' Home button with `adb` on Android device - %v, you need to press it yourself to hide the transparent activity og GADS-stream:\n %v", device.Device.UDID, err))
	}
}

// Forward gads-stream socket to the host container
func (device *LocalDevice) forwardGadsStream() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "forward", "tcp:"+device.Device.StreamPort, "tcp:1991")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Forwarding GADS-stream port to host port %v for Android device - %v", device.Device.StreamPort, device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS-stream port to host port %v for Android device - %v:\n %v", device.Device.StreamPort, device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

// IOS DEVICES

// Forward iOS device ports using `go-ios` CLI, for some reason using the library doesn't work properly
func (device *LocalDevice) goIOSForward(hostPort string, devicePort string) {
	cmd := exec.CommandContext(device.Context, "ios", "forward", hostPort, devicePort, "--udid="+device.Device.UDID)

	// Create a pipe to capture the command's output
	_, err := cmd.StdoutPipe()
	if err != nil {
		util.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not create stdoutpipe executing `ios forward` for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	// Start the port forward command
	err = cmd.Start()
	if err != nil {
		util.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Error executing `ios forward` for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	if err := cmd.Wait(); err != nil {
		util.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Error waiting `ios forward` to finish for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
}

func buildWebDriverAgent() error {
	// Command to run continuously (replace with your command)
	cmd := exec.Command("xcodebuild", "-project", "WebDriverAgent.xcodeproj", "-scheme", "WebDriverAgentRunner", "-destination", "generic/platform=iOS", "build-for-testing")
	cmd.Dir = util.Config.EnvConfig.WDAPath

	cmd.Stderr = os.Stderr
	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	util.ProviderLogger.LogInfo("provider", fmt.Sprintf("Starting WebDriverAgent xcodebuild in path `%s` with command `%s` ", util.Config.EnvConfig.WDAPath, cmd.String()))
	if err := cmd.Start(); err != nil {
		return err
	}

	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		util.ProviderLogger.LogDebug("webdriveragent_xcodebuild", line)
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		util.ProviderLogger.LogError("provider", fmt.Sprintf("Error waiting for build WebDriverAgent with `xcodebuild` command to finish - %s", err))
		util.ProviderLogger.LogError("provider", "Building WebDriverAgent for testing was unsuccessful")
		os.Exit(1)
	}
	return nil
}

func (device *LocalDevice) startWdaWithXcodebuild() {
	// Create a usbmuxd.log file for Stderr
	logger, _ := util.CreateCustomLogger("./logs/device_"+device.Device.UDID+"/wda.log", device.Device.UDID)

	// Command to run continuously (replace with your command)
	cmd := exec.CommandContext(device.Context, "xcodebuild", "-project", "WebDriverAgent.xcodeproj", "-scheme", "WebDriverAgentRunner", "-destination", "platform=iOS,id="+device.Device.UDID, "test-without-building", "-allowProvisioningUpdates")
	cmd.Dir = util.Config.EnvConfig.WDAPath

	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("Error creating stdoutpipe while running WebDriverAgent with xcodebuild for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	if err := cmd.Start(); err != nil {
		logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("Could not start WebDriverAgent with xcodebuild for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()

		logger.LogInfo("webdriveragent", strings.TrimSpace(line))

		if strings.Contains(line, "Restarting after") {
			device.resetLocalDevice()
			return
		}

		if strings.Contains(line, "ServerURLHere") {
			// device.DeviceIP = strings.Split(strings.Split(line, "//")[1], ":")[0]
			device.WdaReadyChan <- true
		}
	}

	if err := cmd.Wait(); err != nil {
		logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("Error waiting for WebDriverAgent xcodebuild command to finish, it errored out or device `%v` was disconnected - %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

func (device *LocalDevice) getGoIOSDevice() {
	goIosDevice, err := ios.GetDevice(device.Device.UDID)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error(fmt.Sprintf("Could not get `go-ios` DeviceEntry for device - %v, err - %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}

	device.GoIOSDeviceEntry = goIosDevice
}

// HEALTH

// Create a new WebDriverAgent session and update stream settings
func (device *LocalDevice) updateWebDriverAgent() error {
	util.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Updating WebDriverAgent session and mjpeg stream settings for device `%s`", device.Device.UDID))

	err := device.createWebDriverAgentSession()
	if err != nil {
		util.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not create WebDriverAgent session for device %v - %v", device.Device.UDID, err))
		return err
	}

	err = device.updateWebDriverAgentStreamSettings()
	if err != nil {
		util.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not update WebDriverAgent stream settings for device %v - %v", device.Device.UDID, err))
		return err
	}

	return nil
}

func (device *LocalDevice) updateWebDriverAgentStreamSettings() error {
	// Set 30 frames per second, without any scaling, half the original screenshot quality
	// TODO should make this configurable in some way, although can be easily updated the same way
	requestString := `{"settings": {"mjpegServerFramerate": 30, "mjpegServerScreenshotQuality": 30, "mjpegScalingFactor": 100}}`

	// Post the mjpeg server settings
	response, err := http.Post("http://localhost:"+device.Device.WDAPort+"/session/"+device.Device.WDASessionID+"/appium/settings", "application/json", strings.NewReader(requestString))
	if err != nil {
		return err
	}

	if response.StatusCode != 200 {
		return errors.New("Could not successfully update WDA stream settings, status code=" + strconv.Itoa(response.StatusCode))
	}

	return nil
}

// Create a new WebDriverAgent session
func (device *LocalDevice) createWebDriverAgentSession() error {
	// TODO see if this JSON can be simplified
	requestString := `{
		"capabilities": {
			"firstMatch": [{}],
			"alwaysMatch": {
				
			}
		}
	}`

	req, err := http.NewRequest(http.MethodPost, "http://localhost:"+device.Device.WDAPort+"/session", strings.NewReader(requestString))
	if err != nil {
		return err
	}

	response, err := netClient.Do(req)
	if err != nil {
		return err
	}

	// Get the response into a byte slice
	responseBody, _ := io.ReadAll(response.Body)
	// Unmarshal response into a basic map
	var responseJson map[string]interface{}
	err = json.Unmarshal(responseBody, &responseJson)
	if err != nil {
		return err
	}

	// Check the session ID from the map
	if responseJson["sessionId"] == "" {
		if err != nil {
			return errors.New("could not get `sessionId` while creating a new WebDriverAgent session")
		}
	}

	device.Device.WDASessionID = fmt.Sprintf("%v", responseJson["sessionId"])
	return nil
}

// Loops checking if the Appium/WebDriverAgent servers for the device are alive and updates the DB each time
func (device *LocalDevice) updateDeviceHealthStatus() {
	util.ProviderLogger.LogInfo("device_setup", fmt.Sprintf("Started health status check for device %v. Will poll Appium/WebDriverAgent servers respective to the device each second", device.Device.UDID))
	for {
		select {
		case <-time.After(1 * time.Second):
			device.checkDeviceHealthStatus()
		case <-device.Context.Done():
			return
		}
	}
}

// Checks Appium/WebDriverAgent servers are alive for the respective device
// And updates the device health status in the DB
// TODO - Currently unfinished, does not really check Appium for iOS/Android right now. Need to check if it can be unified with the health status endpoint code
func (device *LocalDevice) checkDeviceHealthStatus() {
	if device.Device.OS == "ios" {
		wdaGood := false
		wdaGood, err := device.isWdaHealthy()
		if err != nil {
			util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Failed checking WebDriverAgent status for device %v - %v", device.Device.UDID, err))
		}

		if wdaGood {
			device.Device.LastHealthyTimestamp = time.Now().UnixMilli()
			device.Device.Healthy = true
			return
		}
	} else {
		device.Device.LastHealthyTimestamp = time.Now().UnixMilli()
		device.Device.Healthy = true
		return
	}

	device.Device.Healthy = false
}

// Check if the WebDriverAgent server for an iOS device is up
func (device *LocalDevice) isWdaHealthy() (bool, error) {
	req, err := http.NewRequest(http.MethodGet, "http://localhost:"+device.Device.WDAPort+"/status", nil)
	if err != nil {
		return false, err
	}

	response, err := netClient.Do(req)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	responseCode := response.StatusCode
	if responseCode != 200 {
		return false, nil
	}

	return true, nil
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
	appiumPort, err := getFreePort()
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

func (device *LocalDevice) startWdaWithGoIOS() {
	// Create a usbmuxd.log file for Stderr
	wdaLogger, _ := util.CreateCustomLogger("./logs/device_"+device.Device.UDID+"/wda.log", device.Device.UDID)

	cmd := exec.CommandContext(context.Background(), "ios", "runwda", "--bundleid="+util.Config.EnvConfig.WDABundleID, "--testrunnerbundleid="+util.Config.EnvConfig.WDABundleID, "--xctestconfig=WebDriverAgentRunner.xctest", "--udid="+device.Device.UDID)

	fmt.Println("COMMAND IS: " + cmd.String())
	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error creating stdoutpipe while running WebDriverAgent with go-ios for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	// Create a pipe to capture the command's error output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error creating stderrpipe while running WebDriverAgent with go-ios for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	if err := cmd.Start(); err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not start WebDriverAgent with go-ios for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	// Create a combined reader from stdout and stderr
	combinedReader := io.MultiReader(stderr, stdout)
	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(combinedReader)

	// errScanner := bufio.NewScanner(stderr)
	// for errScanner.Scan() {
	// 	line := errScanner.Text()
	// 	wdaLogger.LogError("webdriveragent", strings.TrimSpace(line))
	// }

	for scanner.Scan() {
		line := scanner.Text()

		wdaLogger.LogInfo("webdriveragent", strings.TrimSpace(line))

		if strings.Contains(line, "ServerURLHere") {
			// device.DeviceIP = strings.Split(strings.Split(line, "//")[1], ":")[0]
			device.WdaReadyChan <- true
		}
	}

	if err := cmd.Wait(); err != nil {
		wdaLogger.LogError("webdriveragent", fmt.Sprintf("Error waiting for WebDriverAgent go-ios command to finish, it errored out or device `%v` was disconnected - %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

// Mount the developer disk images downloading them automatically in /opt/DeveloperDiskImages
func (device *LocalDevice) mountDeveloperImageIOS() error {
	basedir := "./devimages"

	var err error
	path, err := imagemounter.DownloadImageFor(device.GoIOSDeviceEntry, basedir)
	if err != nil {
		return err
	}

	err = imagemounter.MountImage(device.GoIOSDeviceEntry, path)
	if err != nil {
		return err
	}

	return nil
}

func (device *LocalDevice) pairIOS() error {
	log.WithFields(log.Fields{
		"event": "ios_device_setup",
	}).Debug("Pairing iOS device - " + device.Device.UDID)

	p12, err := os.ReadFile("./configs/supervision.p12")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Warn(fmt.Sprintf("Could not read /opt/supervision.p12 file when pairing device with UDID: %s, falling back to unsupervised pairing, err:%s", device.Device.UDID, err))
		err = ios.Pair(device.GoIOSDeviceEntry)
		if err != nil {
			return errors.New("Could not pair successfully, err:" + err.Error())
		}
		return nil
	}

	err = ios.PairSupervised(device.GoIOSDeviceEntry, p12, util.Config.EnvConfig.SupervisionPassword)
	if err != nil {
		return errors.New("Could not pair successfully, err:" + err.Error())
	}

	return nil
}

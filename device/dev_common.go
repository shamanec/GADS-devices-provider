package device

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/pelletier/go-toml"
	"github.com/shamanec/GADS-devices-provider/util"
)

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
	err := util.CheckGadsStreamAndDownload()
	if err != nil {
		panic(fmt.Sprintf("Could not check availability of and download GADS-stream latest release - %s", err))
	}
}

func createDeviceMap() {
	for _, device := range localDevices {
		DeviceMap[device.Device.UDID] = device
	}
}

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

		localDevice.createGridTOML()

		// Create logs directory for each device if it doesn't already exist
		if _, err := os.Stat("./logs/device_" + device.UDID); os.IsNotExist(err) {
			err = os.Mkdir("./logs/device_"+device.UDID, os.ModePerm)
			if err != nil {
				panic(fmt.Sprintf("Could not create logs folder for device `%s` - %s\n", device.UDID, err))
			}
		}

		logger, err := util.CreateCustomLogger("./logs/device_"+device.UDID+"/device.log", device.UDID)
		if err != nil {
			panic(fmt.Sprintf("Could not create a customer logger for device `%s` - %s", device.UDID, err))
		}
		localDevice.Logger = *logger
	}
}

func (device *LocalDevice) setupAndroidDevice() {
	device.ProviderState = "preparing"

	util.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Running setup for device `%v`", device.Device.UDID))

	isStreamAvailable, err := device.isGadsStreamServiceRunning()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not check if GADS-stream is running on device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	streamPort, err := util.GetFreePort()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not allocate free host port for GADS-stream for device `%v` - %v", device.Device.UDID, err))
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
	if util.Config.EnvConfig.UseSeleniumGrid {
		go device.startGridNode()
	}
	// Mark the device as 'live'
	device.ProviderState = "live"
}

func (device *LocalDevice) setupIOSDevice() {
	device.ProviderState = "preparing"
	util.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Running setup for device `%v`", device.Device.UDID))

	// Get go-ios device entry for pairing/mounting images
	device.getGoIOSDevice()

	wdaPort, err := util.GetFreePort()
	if err != nil {
		util.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent port for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
	device.Device.WDAPort = fmt.Sprint(wdaPort)

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
		util.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully create WebDriverAgent session or update its stream settings for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	go device.startAppium()
	if util.Config.EnvConfig.UseSeleniumGrid {
		go device.startGridNode()
	}

	// Mark the device as 'live'
	device.ProviderState = "live"
}

// Gets all connected iOS and Android devices to the host
func getConnectedDevicesCommon(ios bool, android bool) []string {
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
		util.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not get connected iOS devices with `go-ios` library, returning empty slice - %s", err))
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
		util.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not get connected Android devices with `adb`, creating exec cmd StdoutPipe failed, returning empty slice - %s", err))
		return connectedDevices
	}

	if err := cmd.Start(); err != nil {
		util.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not get connected Android devices with `adb`, starting command failed, returning empty slice - %s", err))
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
		util.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not get connected Android devices with `adb`, waiting for command to finish failed, returning empty slice - %s", err))
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
		util.ProviderLogger.LogInfo("provider", fmt.Sprintf("Resetting LocalDevice for device `%v` after error. Cancelling context, setting ProviderState to `init`, Healthy to `false` and updating the DB", device.Device.UDID))

		device.IsResetting = true
		device.CtxCancel()
		device.ProviderState = "init"
		device.IsResetting = false
	}

}

// Set a context for a device to enable cancelling running goroutines related to that device when its disconnected
func (device *LocalDevice) setContext() {
	ctx, cancelFunc := context.WithCancel(context.Background())
	device.CtxCancel = cancelFunc
	device.Context = ctx
}

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

	cmd := exec.CommandContext(device.Context, "appium", "-p", device.Device.AppiumPort, "--log-timestamp", "--allow-cors", "--session-override", "--default-capabilities", string(capabilitiesJson))

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
		device.Logger.LogDebug("appium", strings.TrimSpace(line))
	}

	if err := cmd.Wait(); err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error waiting for Appium command to finish, it errored out or device `%v` was disconnected - %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

type AppiumTomlNode struct {
	DetectDrivers bool `toml:"detect-drivers"`
}

type AppiumTomlServer struct {
	Port int `toml:"port"`
}

type AppiumTomlRelay struct {
	URL            string   `toml:"url"`
	StatusEndpoint string   `toml:"status-endpoint"`
	Configs        []string `toml:"configs"`
}

type AppiumTomlConfig struct {
	Server AppiumTomlServer `toml:"server"`
	Node   AppiumTomlNode   `toml:"node"`
	Relay  AppiumTomlRelay  `toml:"relay"`
}

var port_counter = 0

func (device *LocalDevice) createGridTOML() {
	automationName := ""
	if device.Device.OS == "ios" {
		automationName = "XCUITest"
	} else {
		automationName = "UiAutomator2"
	}

	url := fmt.Sprintf("http://%s:%v/device/%s/appium", util.Config.EnvConfig.HostAddress, util.Config.EnvConfig.HostPort, device.Device.UDID)
	configs := fmt.Sprintf(`{"appium:deviceName": "%s", "platformName": "%s", "appium:platformVersion": "%s", "appium:automationName": "%s"}`, device.Device.Name, device.Device.OS, device.Device.OSVersion, automationName)

	config := AppiumTomlConfig{
		Server: AppiumTomlServer{
			Port: 5555 + port_counter,
		},
		Node: AppiumTomlNode{
			DetectDrivers: false,
		},
		Relay: AppiumTomlRelay{
			URL:            url,
			StatusEndpoint: "/status",
			Configs: []string{
				"1",
				configs,
			},
		},
	}

	res, err := toml.Marshal(config)
	if err != nil {
		panic(fmt.Sprintf("Failed marshalling TOML Appium config for device `%s` - %s", device.Device.UDID, err))
	}

	file, err := os.Create("./config/" + device.Device.UDID + ".toml")
	if err != nil {
		panic(fmt.Sprintf("Failed creating TOML Appium config file for device `%s` - %s", device.Device.UDID, err))
	}
	defer file.Close()

	_, err = io.WriteString(file, string(res))
	if err != nil {
		panic(fmt.Sprintf("Failed writing to TOML Appium config file for device `%s` - %s", device.Device.UDID, err))
	}
	port_counter++
}

func (device *LocalDevice) startGridNode() {
	time.Sleep(5 * time.Second)
	cmd := exec.CommandContext(device.Context, "java", "-jar", "./apps/"+util.Config.EnvConfig.SeleniumJar, "node", "--config", util.ProjectDir+"/config/"+device.Device.UDID+".toml", "--grid-url", util.Config.EnvConfig.SeleniumGrid)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error creating stdoutpipe while starting Selenium Grid node for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	if err := cmd.Start(); err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not start Selenium Grid node for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		device.Logger.LogDebug("grid-node", strings.TrimSpace(line))
	}

	if err := cmd.Wait(); err != nil {
		util.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error waiting for Selenium Grid node command to finish, it errored out or device `%v` was disconnected - %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

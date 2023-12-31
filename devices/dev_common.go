package devices

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/pelletier/go-toml"
	"github.com/shamanec/GADS-devices-provider/config"
	"github.com/shamanec/GADS-devices-provider/logger"
	"github.com/shamanec/GADS-devices-provider/models"
	"github.com/shamanec/GADS-devices-provider/util"
)

var netClient = &http.Client{
	Timeout: time.Second * 120,
}
var localDevices []*models.LocalDevice
var DeviceMap = make(map[string]*models.LocalDevice)

func UpdateDevices() {
	Setup()

	switch runtime.GOOS {
	case "linux":
		go updateDevicesLinux()
	case "darwin":
		go updateDevicesOSX()
	case "windows":
		go updateDevicesWindows()
	default:
		log.Fatal("OS is not one of `linux`, `darwin`, `windows`")
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
	for _, device := range config.Config.Devices {
		localDevice := models.LocalDevice{
			Device:        device,
			ProviderState: "init",
			IsResetting:   false,
		}

		setContext(&localDevice)
		localDevice.Device.HostAddress = config.Config.EnvConfig.HostAddress
		localDevice.Device.Provider = config.Config.EnvConfig.Nickname
		localDevice.Device.Model = "N/A"
		localDevice.Device.OSVersion = "N/A"
		localDevices = append(localDevices, &localDevice)

		if config.Config.EnvConfig.UseSeleniumGrid {
			createGridTOML(&localDevice)
		}

		// Create logs directory for each device if it doesn't already exist
		if _, err := os.Stat("./logs/device_" + device.UDID); os.IsNotExist(err) {
			err = os.Mkdir("./logs/device_"+device.UDID, os.ModePerm)
			if err != nil {
				panic(fmt.Sprintf("Could not create logs folder for device `%s` - %s\n", device.UDID, err))
			}
		}

		logger, err := logger.CreateCustomLogger(fmt.Sprintf("%s/logs/device_%s/device.log", config.Config.EnvConfig.ProviderFolder, device.UDID), device.UDID)
		if err != nil {
			panic(fmt.Sprintf("Could not create a customer logger for device `%s` - %s", device.UDID, err))
		}
		localDevice.Logger = *logger
	}
}

func setupAndroidDevice(device *models.LocalDevice) {
	device.ProviderState = "preparing"

	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Running setup for device `%v`", device.Device.UDID))

	err := updateScreenSize(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not update screen dimensions with adb for device `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}
	updateModel(device)
	updateOSVersion(device)

	isStreamAvailable, err := isGadsStreamServiceRunning(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not check if GADS-stream is running on device `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}

	streamPort, err := util.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not allocate free host port for GADS-stream for device `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.StreamPort = fmt.Sprint(streamPort)

	if !isStreamAvailable {
		err = installGadsStream(device)
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not install GADS-stream on Android device - %v:\n %v", device.Device.UDID, err))
			resetLocalDevice(device)
			return
		}

		err = addGadsStreamRecordingPermissions(device)
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not set GADS-stream recording permissions on Android device - %v:\n %v", device.Device.UDID, err))
			resetLocalDevice(device)
			return
		}

		err = startGadsStreamApp(device)
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not start GADS-stream app on Android device - %v:\n %v", device.Device.UDID, err))
			resetLocalDevice(device)
			return
		}

		pressHomeButton(device)
	}

	err = forwardGadsStream(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS-stream port to host port %v for Android device - %v:\n %v", device.StreamPort, device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}

	device.Device.InstalledApps = getInstalledAppsAndroid(device)

	go startAppium(device)
	if config.Config.EnvConfig.UseSeleniumGrid {
		go startGridNode(device)
	}

	// Mark the device as 'live'
	device.ProviderState = "live"
}

func setupIOSDevice(device *models.LocalDevice) {
	device.ProviderState = "preparing"
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Running setup for device `%v`", device.Device.UDID))

	// Get go-ios device entry for pairing/mounting images
	getGoIOSDevice(device)

	// Get device info with go-ios to get the hardware model
	plistValues, err := ios.GetValuesPlist(device.GoIOSDeviceEntry)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not get info plist values with go-ios `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}
	// Update hardware model got from plist, os version and product type
	device.Device.HardwareModel = plistValues["HardwareModel"].(string)
	device.Device.OSVersion = plistValues["ProductVersion"].(string)
	device.Device.IOSProductType = plistValues["ProductType"].(string)

	// Update the screen dimensions of the device using data from the IOSDeviceDimensions map
	err = updateScreenSize(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not update screen dimensions for device `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}
	updateModel(device)

	wdaPort, err := util.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent port for device `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.WDAPort = fmt.Sprint(wdaPort)

	streamPort, err := util.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent stream port for device `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.StreamPort = fmt.Sprint(streamPort)

	// Forward the WebDriverAgent server and stream to the host
	go goIOSForward(device, device.WDAPort, "8100")
	go goIOSForward(device, device.StreamPort, "9100")

	// Start WebDriverAgent with `xcodebuild`
	go startWdaWithXcodebuild(device)

	// Wait until WebDriverAgent successfully starts
	select {
	case <-device.WdaReadyChan:
		logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Successfully started WebDriverAgent for device `%v` forwarded on port %v", device.Device.UDID, device.WDAPort))
		break
	case <-time.After(30 * time.Second):
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully start WebDriverAgent on device `%v` in 30 seconds", device.Device.UDID))
		resetLocalDevice(device)
		return
	}

	// Create a WebDriverAgent session and update the MJPEG stream settings
	err = updateWebDriverAgent(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully create WebDriverAgent session or update its stream settings for device `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}

	go startAppium(device)
	if config.Config.EnvConfig.UseSeleniumGrid {
		startGridNode(device)
	}

	device.Device.InstalledApps = getInstalledAppsIOS(device)

	// Mark the device as 'live'
	device.ProviderState = "live"
}

// Gets all connected iOS and Android devices to the host
func getConnectedDevicesCommon() []string {
	connectedDevices := []string{}

	androidDevices := []string{}
	iosDevices := []string{}

	if config.Config.EnvConfig.ProvideAndroid {
		androidDevices = GetConnectedDevicesAndroid()
	}

	if config.Config.EnvConfig.ProvideIOS {
		iosDevices = GetConnectedDevicesIOS()
	}

	connectedDevices = append(connectedDevices, iosDevices...)
	connectedDevices = append(connectedDevices, androidDevices...)

	return connectedDevices
}

// Gets the connected iOS devices using the `go-ios` library
func GetConnectedDevicesIOS() []string {
	var connectedDevices []string

	deviceList, err := ios.ListDevices()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not get connected iOS devices with `go-ios` library, returning empty slice - %s", err))
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
func GetConnectedDevicesAndroid() []string {
	var connectedDevices []string

	cmd := exec.Command("adb", "devices")
	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not get connected Android devices with `adb`, creating exec cmd StdoutPipe failed, returning empty slice - %s", err))
		return connectedDevices
	}

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not get connected Android devices with `adb`, starting command failed, returning empty slice - %s", err))
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
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not get connected Android devices with `adb`, waiting for command to finish failed, returning empty slice - %s", err))
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

func resetLocalDevice(device *models.LocalDevice) {

	if !device.IsResetting {
		logger.ProviderLogger.LogInfo("provider", fmt.Sprintf("Resetting LocalDevice for device `%v` after error. Cancelling context, setting ProviderState to `init`, Healthy to `false` and updating the DB", device.Device.UDID))

		device.IsResetting = true
		device.CtxCancel()
		device.ProviderState = "init"
		device.IsResetting = false
	}

}

// Set a context for a device to enable cancelling running goroutines related to that device when its disconnected
func setContext(device *models.LocalDevice) {
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

func startAppium(device *models.LocalDevice) {
	var capabilities appiumCapabilities

	if device.Device.OS == "ios" {
		capabilities = appiumCapabilities{
			UDID:                  device.Device.UDID,
			WdaURL:                "http://localhost:" + device.WDAPort,
			WdaMjpegPort:          device.StreamPort,
			WdaLocalPort:          device.WDAPort,
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
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not create appium.log file for device - %v, err - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}
	defer appiumLog.Close()

	// Get a free port on the host for Appium server
	appiumPort, err := util.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not allocate free Appium host port for device - %v, err - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.AppiumPort = fmt.Sprint(appiumPort)

	cmd := exec.CommandContext(device.Context, "appium", "-p", device.AppiumPort, "--log-timestamp", "--session-override", "--default-capabilities", string(capabilitiesJson))

	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error creating stdoutpipe while running WebDriverAgent with xcodebuild for device `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not start WebDriverAgent with xcodebuild for device `%v` - %v", device.Device.UDID, err))
		resetLocalDevice(device)
		return
	}

	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		device.Logger.LogDebug("appium", strings.TrimSpace(line))
	}

	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error waiting for Appium command to finish, it errored out or device `%v` was disconnected - %v", device.Device.UDID, err))
		resetLocalDevice(device)
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

func createGridTOML(device *models.LocalDevice) {
	automationName := ""
	if device.Device.OS == "ios" {
		automationName = "XCUITest"
	} else {
		automationName = "UiAutomator2"
	}

	url := fmt.Sprintf("http://%s:%v/device/%s/appium", config.Config.EnvConfig.HostAddress, config.Config.EnvConfig.Port, device.Device.UDID)
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

func startGridNode(device *models.LocalDevice) {
	// time.Sleep(5 * time.Second)
	// cmd := exec.CommandContext(device.Context, "java", "-jar", "./apps/"+config.Config.EnvConfig.SeleniumJar, "node", "--config", util.ProjectDir+"/config/"+device.Device.UDID+".toml", "--grid-url", config.Config.EnvConfig.SeleniumGrid)

	// stdout, err := cmd.StdoutPipe()
	// if err != nil {
	// 	logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error creating stdoutpipe while starting Selenium Grid node for device `%v` - %v", device.Device.UDID, err))
	// 	device.resetLocalDevice()
	// 	return
	// }

	// if err := cmd.Start(); err != nil {
	// 	logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not start Selenium Grid node for device `%v` - %v", device.Device.UDID, err))
	// 	device.resetLocalDevice()
	// 	return
	// }

	// scanner := bufio.NewScanner(stdout)

	// for scanner.Scan() {
	// 	line := scanner.Text()
	// 	device.Logger.LogDebug("grid-node", strings.TrimSpace(line))
	// }

	// if err := cmd.Wait(); err != nil {
	// 	logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error waiting for Selenium Grid node command to finish, it errored out or device `%v` was disconnected - %v", device.Device.UDID, err))
	// 	device.resetLocalDevice()
	// }
}

func updateScreenSize(device *models.LocalDevice) error {
	if device.Device.OS == "ios" {
		if dimensions, ok := util.IOSDeviceInfoMap[device.Device.IOSProductType]; ok {
			device.Device.ScreenHeight = dimensions.Height
			device.Device.ScreenWidth = dimensions.Width
		} else {
			return fmt.Errorf("could not find `%s` hardware model in the IOSDeviceDimensions map, please update the map", device.Device.HardwareModel)
		}
	} else {
		err := updateAndroidScreenSizeADB(device)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateModel(device *models.LocalDevice) {
	if device.Device.OS == "ios" {
		if info, ok := util.IOSDeviceInfoMap[device.Device.IOSProductType]; ok {
			device.Device.Model = info.Model
		} else {
			device.Device.Model = "Unknown iOS device"
		}
	} else {
		brandCmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "getprop", "ro.product.brand")
		var outBuffer bytes.Buffer
		brandCmd.Stdout = &outBuffer
		if err := brandCmd.Run(); err != nil {
			device.Device.Model = "Unknown brand and model"
		}
		brand := outBuffer.String()
		outBuffer.Reset()

		modelCmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "getprop", "ro.product.model")
		modelCmd.Stdout = &outBuffer
		if err := modelCmd.Run(); err != nil {
			device.Device.Model = "Unknown brand/model"
			return
		}
		model := outBuffer.String()

		device.Device.Model = fmt.Sprintf("%s %s", strings.TrimSpace(brand), strings.TrimSpace(model))
	}
}

func updateOSVersion(device *models.LocalDevice) {
	if device.Device.OS == "ios" {

	} else {
		sdkCmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "getprop", "ro.build.version.sdk")
		var outBuffer bytes.Buffer
		sdkCmd.Stdout = &outBuffer
		if err := sdkCmd.Run(); err != nil {
			device.Device.OSVersion = "N/A"
		}
		sdkVersion := strings.TrimSpace(outBuffer.String())
		if osVersion, ok := util.AndroidVersionToSDK[sdkVersion]; ok {
			device.Device.OSVersion = osVersion
		} else {
			device.Device.OSVersion = "N/A"
		}
	}
}

func UpdateInstalledApps(device *models.LocalDevice) {
	if device.Device.OS == "ios" {
		device.Device.InstalledApps = getInstalledAppsIOS(device)
	} else {
		device.Device.InstalledApps = getInstalledAppsAndroid(device)
	}
}

func UninstallApp(device *models.LocalDevice, app string) error {
	if device.Device.OS == "ios" {
		err := uninstallAppIOS(device, app)
		if err != nil {
			return err
		}
	} else {
		err := uninstallAppAndroid(device, app)
		if err != nil {
			return err
		}
	}

	return nil
}

func InstallApp(device *models.LocalDevice, app string) error {
	if device.Device.OS == "ios" {
		err := installAppIOS(device, app)
		if err != nil {
			return err
		}
	} else {
		err := installAppAndroid(device, app)
		if err != nil {
			return err
		}
	}

	return nil
}

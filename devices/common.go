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
	"slices"
	"strings"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/pelletier/go-toml"
	"github.com/shamanec/GADS-devices-provider/config"
	"github.com/shamanec/GADS-devices-provider/db"
	"github.com/shamanec/GADS-devices-provider/logger"
	"github.com/shamanec/GADS-devices-provider/models"
	"github.com/shamanec/GADS-devices-provider/util"
)

var ConnectedDevices []models.ConnectedDevice

var netClient = &http.Client{
	Timeout: time.Second * 120,
}
var DeviceMap = make(map[string]*models.Device)

func UpdateDevices() {
	Setup()

	// Start updating devices each 10 seconds in a goroutine
	go updateDevicesAnyOS()
	// Start updating the local devices data to Mongo in a goroutine
	go updateDevicesMongo()
}

func updateDevicesAnyOS() {
	// Create common logs directory if it doesn't already exist
	if _, err := os.Stat(fmt.Sprintf("%s/logs", config.Config.EnvConfig.ProviderFolder)); os.IsNotExist(err) {
		os.Mkdir(fmt.Sprintf("%s/logs", config.Config.EnvConfig.ProviderFolder), os.ModePerm)
	}

	// If we want to provide Android devices check if adb is available on PATH
	if config.Config.EnvConfig.ProvideAndroid {
		if !adbAvailable() {
			logger.ProviderLogger.LogError("provider", "adb is not available, you need to set up the host as explained in the readme")
			fmt.Println("adb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	// If running on MacOS
	if config.Config.EnvConfig.OS == "darwin" {
		// Check if xcodebuild is available - Xcode and command line tools should be installed
		if !xcodebuildAvailable() {
			logger.ProviderLogger.LogError("provider", "xcodebuild is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}

		// Check if provided WebDriverAgent repo path exists
		_, err := os.Stat(config.Config.EnvConfig.WdaRepoPath)
		if err != nil {
			logger.ProviderLogger.LogError("provider", config.Config.EnvConfig.WdaRepoPath+" does not exist, you need to provide valid path to the WebDriverAgent repo in config.json")
			fmt.Println(config.Config.EnvConfig.WdaRepoPath + " does not exist, you need to provide valid path to the WebDriverAgent repo in config.json")
			os.Exit(1)
		}

		// Build the WebDriverAgent using xcodebuild from the provided repo path
		err = buildWebDriverAgent()
		if err != nil {
			logger.ProviderLogger.LogError("provider", fmt.Sprintf("Could not successfully build WebDriverAgent for testing - %s", err))
			fmt.Println("Could not successfully build WebDriverAgent for testing - " + err.Error())
			os.Exit(1)
		}
	}

	// Try to remove potentially hanging ports forwarded by adb
	removeAdbForwardedPorts()

	for {
		dbDevices, _ := db.GetConfiguredDevices(config.Config.EnvConfig.Nickname)
		// Update local devices map
		for _, dbDevice := range dbDevices {
			// If the device is not in the local device map
			// Set it up and add it
			if _, ok := DeviceMap[dbDevice.UDID]; !ok {
				dbDevice.ProviderState = "init"
				dbDevice.IsResetting = false

				setContext(dbDevice)
				dbDevice.HostAddress = config.Config.EnvConfig.HostAddress
				dbDevice.Provider = config.Config.EnvConfig.Nickname
				dbDevice.Model = "N/A"
				dbDevice.OSVersion = "N/A"
				DeviceMap[dbDevice.UDID] = dbDevice

				if config.Config.EnvConfig.UseSeleniumGrid {
					createGridTOML(dbDevice)
				}

				// Create logs directory for each device if it doesn't already exist
				if _, err := os.Stat(fmt.Sprintf("%s/logs/device_%s", config.Config.EnvConfig.ProviderFolder, dbDevice.UDID)); os.IsNotExist(err) {
					err = os.Mkdir(fmt.Sprintf("%s/logs/device_%s", config.Config.EnvConfig.ProviderFolder, dbDevice.UDID), os.ModePerm)
					if err != nil {
						panic(fmt.Sprintf("Could not create logs folder for device `%s` - %s\n", dbDevice.UDID, err))
					}
				}

				logger, err := logger.CreateCustomLogger(fmt.Sprintf("%s/logs/device_%s/device.log", config.Config.EnvConfig.ProviderFolder, dbDevice.UDID), dbDevice.UDID)
				if err != nil {
					panic(fmt.Sprintf("Could not create a customer logger for device `%s` - %s", dbDevice.UDID, err))
				}
				dbDevice.Logger = *logger
			}
		}

		ConnectedDevices = GetConnectedDevicesCommon()

		// If there are no devices or all devices were disconnected
		// Loop through the local devices and reset them
		if len(ConnectedDevices) == 0 {
			logger.ProviderLogger.LogDebug("provider", "No devices connected")

			for _, device := range DeviceMap {
				if device.Connected {
					device.Connected = false
					resetLocalDevice(device)
				}
			}
		} else {
			// Loop through the provider devices
			for _, device := range DeviceMap {
				// If a connected device is part of the provider devices
			CONNECTED:
				for _, connDevice := range ConnectedDevices {
					if connDevice.UDID == device.UDID {
						// Set it as connected
						device.Connected = true
						// If we are not already preparing the device or its not already prepared
						if device.ProviderState != "preparing" && device.ProviderState != "live" {
							// Setup the device
							setContext(device)
							if device.OS == "ios" {
								device.WdaReadyChan = make(chan bool, 1)
								go setupIOSDevice(device)
							}

							if device.OS == "android" {
								go setupAndroidDevice(device)
							}
						}
						break CONNECTED
					}
					// If local devices is not in the connected devices
					// Set connected as false
					device.Connected = false
				}
			}
		}
		time.Sleep(10 * time.Second)
	}
}

// Create Mongo collections for all devices for logging
// Create a map of *device.LocalDevice for easier access across the code
func Setup() {
	createMongoLogCollectionsForAllDevices()
	if config.Config.EnvConfig.ProvideAndroid {
		err := util.CheckGadsStreamAndDownload()
		if err != nil {
			log.Fatalf("Could not check availability of and download GADS-stream latest release - %s", err)
		}
	}
}

func setupAndroidDevice(device *models.Device) {
	device.ProviderState = "preparing"

	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Running setup for device `%v`", device.UDID))

	err := updateScreenSize(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not update screen dimensions with adb for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	getModel(device)
	getAndroidOSVersion(device)

	isStreamAvailable, err := isGadsStreamServiceRunning(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not check if GADS-stream is running on device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	streamPort, err := util.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not allocate free host port for GADS-stream for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.StreamPort = fmt.Sprint(streamPort)

	if !isStreamAvailable {
		apps := getInstalledAppsAndroid(device)
		if slices.Contains(apps, "com.shamanec.stream") {
			err = uninstallGadsStream(device)
			if err != nil {
				logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not uninstall GADS-stream from Android device - %v:\n %v", device.UDID, err))
				resetLocalDevice(device)
				return
			}
			time.Sleep(1 * time.Second)
		}

		err = installGadsStream(device)
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not install GADS-stream on Android device - %v:\n %v", device.UDID, err))
			resetLocalDevice(device)
			return
		}
		time.Sleep(1 * time.Second)

		err = addGadsStreamRecordingPermissions(device)
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not set GADS-stream recording permissions on Android device - %v:\n %v", device.UDID, err))
			resetLocalDevice(device)
			return
		}
		time.Sleep(1 * time.Second)

		err = startGadsStreamApp(device)
		if err != nil {
			logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not start GADS-stream app on Android device - %v:\n %v", device.UDID, err))
			resetLocalDevice(device)
			return
		}
		time.Sleep(1 * time.Second)

		pressHomeButton(device)
	}

	err = forwardGadsStream(device)
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS-stream port to host port %v for Android device - %v:\n %v", device.StreamPort, device.UDID, err))
		resetLocalDevice(device)
		return
	}

	device.InstalledApps = getInstalledAppsAndroid(device)

	go startAppium(device)
	if config.Config.EnvConfig.UseSeleniumGrid {
		go startGridNode(device)
	}

	// Mark the device as 'live'
	device.ProviderState = "live"
}

func setupIOSDevice(device *models.Device) {
	device.ProviderState = "preparing"
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Running setup for device `%v`", device.UDID))

	goIosDeviceEntry, err := ios.GetDevice(device.UDID)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not get `go-ios` DeviceEntry for device - %v, err - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	device.GoIOSDeviceEntry = goIosDeviceEntry

	// Get device info with go-ios to get the hardware model
	plistValues, err := ios.GetValuesPlist(device.GoIOSDeviceEntry)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not get info plist values with go-ios `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	// Update hardware model got from plist, os version and product type
	device.HardwareModel = plistValues["HardwareModel"].(string)
	device.OSVersion = plistValues["ProductVersion"].(string)
	device.IOSProductType = plistValues["ProductType"].(string)

	// Update the screen dimensions of the device using data from the IOSDeviceDimensions map
	err = updateScreenSize(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not update screen dimensions for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	getModel(device)

	wdaPort, err := util.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent port for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.WDAPort = fmt.Sprint(wdaPort)

	streamPort, err := util.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not allocate free WebDriverAgent stream port for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.StreamPort = fmt.Sprint(streamPort)

	// Forward the WebDriverAgent server and stream to the host
	go goIOSForward(device, device.WDAPort, "8100")
	go goIOSForward(device, device.StreamPort, "9100")

	isAboveIOS17, err := isAboveIOS17(device)
	if err != nil {
		device.Logger.LogError("ios_device_setup", fmt.Sprintf("Could not determine if device `%v` is above iOS 17 - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if isAboveIOS17 {
		if config.Config.EnvConfig.OS != "darwin" {
			logger.ProviderLogger.LogError("ios_device_setup", "Using iOS >= 17 devices on Linux and Windows is not supported")
			resetLocalDevice(device)
			return
		}
		// Start WebDriverAgent with `xcodebuild`
		go startWdaWithXcodebuild(device)
	} else {
		wda_path := ""
		// If on MacOS use the WebDriverAgent app from the xcodebuild output
		if config.Config.EnvConfig.OS == "darwin" {
			wda_path = config.Config.EnvConfig.WdaRepoPath + "build/Build/Products/Debug-iphoneos/WebDriverAgentRunner-Runner.app"
		} else {
			// If on Linux or Windows use the prebuilt and provided WebDriverAgent.ipa/app file
			wda_path = fmt.Sprintf("%s/conf/%s", config.Config.EnvConfig.ProviderFolder, config.Config.EnvConfig.WebDriverBinary)
		}
		err = InstallAppWithDevice(device, wda_path)
		if err != nil {
			logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Could not install WebDriverAgent on device `%s` - %s", device.UDID, err))
			resetLocalDevice(device)
			return
		}

		go startWdaWithGoIOS(device)
	}

	// Wait until WebDriverAgent successfully starts
	select {
	case <-device.WdaReadyChan:
		logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Successfully started WebDriverAgent for device `%v` forwarded on port %v", device.UDID, device.WDAPort))
		break
	case <-time.After(30 * time.Second):
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully start WebDriverAgent on device `%v` in 30 seconds", device.UDID))
		resetLocalDevice(device)
		return
	}

	// Create a WebDriverAgent session and update the MJPEG stream settings
	err = updateWebDriverAgent(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("Did not successfully create WebDriverAgent session or update its stream settings for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	go startAppium(device)
	if config.Config.EnvConfig.UseSeleniumGrid {
		go startGridNode(device)
	}

	device.InstalledApps = getInstalledAppsIOS(device)

	// Mark the device as 'live'
	device.ProviderState = "live"
}

// Gets all connected iOS and Android devices to the host
func GetConnectedDevicesCommon() []models.ConnectedDevice {
	connectedDevices := []models.ConnectedDevice{}

	androidDevices := []models.ConnectedDevice{}
	iosDevices := []models.ConnectedDevice{}

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
func GetConnectedDevicesIOS() []models.ConnectedDevice {
	var connectedDevices []models.ConnectedDevice

	deviceList, err := ios.ListDevices()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not get connected iOS devices with `go-ios` library, returning empty slice - %s", err))
		return connectedDevices
	}

	for _, connDevice := range deviceList.DeviceList {
		connectedDevices = append(connectedDevices, models.ConnectedDevice{OS: "ios", UDID: connDevice.Properties.SerialNumber})
	}
	return connectedDevices
}

// Gets the connected android devices using `adb`
func GetConnectedDevicesAndroid() []models.ConnectedDevice {
	var connectedDevices []models.ConnectedDevice

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
			connectedDevices = append(connectedDevices, models.ConnectedDevice{OS: "android", UDID: strings.Fields(line)[0]})
		}
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not get connected Android devices with `adb`, waiting for command to finish failed, returning empty slice - %s", err))
		return []models.ConnectedDevice{}
	}
	return connectedDevices
}

func resetLocalDevice(device *models.Device) {

	if !device.IsResetting {
		logger.ProviderLogger.LogInfo("provider", fmt.Sprintf("Resetting LocalDevice for device `%v` after error. Cancelling context, setting ProviderState to `init`, Healthy to `false` and updating the DB", device.UDID))

		device.IsResetting = true
		device.CtxCancel()
		device.ProviderState = "init"
		device.IsResetting = false
	}

}

// Set a context for a device to enable cancelling running goroutines related to that device when its disconnected
func setContext(device *models.Device) {
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

func startAppium(device *models.Device) {
	var capabilities appiumCapabilities

	if device.OS == "ios" {
		capabilities = appiumCapabilities{
			UDID:                  device.UDID,
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
			DeviceName:            device.Name,
		}
	} else if device.OS == "android" {
		capabilities = appiumCapabilities{
			UDID:           device.UDID,
			AutomationName: "UiAutomator2",
			PlatformName:   "Android",
			DeviceName:     device.Name,
		}
	}

	capabilitiesJson, _ := json.Marshal(capabilities)

	// Get a free port on the host for Appium server
	appiumPort, err := util.GetFreePort()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not allocate free Appium host port for device - %v, err - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
	device.AppiumPort = fmt.Sprint(appiumPort)

	cmd := exec.CommandContext(device.Context, "appium", "-p", device.AppiumPort, "--log-timestamp", "--session-override", "--default-capabilities", string(capabilitiesJson))

	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error creating stdoutpipe while running WebDriverAgent with xcodebuild for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not start WebDriverAgent with xcodebuild for device `%v` - %v", device.UDID, err))
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
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error waiting for Appium command to finish, it errored out or device `%v` was disconnected - %v", device.UDID, err))
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

func createGridTOML(device *models.Device) {
	automationName := ""
	if device.OS == "ios" {
		automationName = "XCUITest"
	} else {
		automationName = "UiAutomator2"
	}

	url := fmt.Sprintf("http://%s:%v/device/%s/appium", config.Config.EnvConfig.HostAddress, config.Config.EnvConfig.Port, device.UDID)
	configs := fmt.Sprintf(`{"appium:deviceName": "%s", "platformName": "%s", "appium:platformVersion": "%s", "appium:automationName": "%s"}`, device.Name, device.OS, device.OSVersion, automationName)

	port, _ := util.GetFreePort()
	conf := AppiumTomlConfig{
		Server: AppiumTomlServer{
			Port: port,
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

	res, err := toml.Marshal(conf)
	if err != nil {
		panic(fmt.Sprintf("Failed marshalling TOML Appium config for device `%s` - %s", device.UDID, err))
	}

	file, err := os.Create(fmt.Sprintf("%s/conf/%s.toml", config.Config.EnvConfig.ProviderFolder, device.UDID))
	if err != nil {
		panic(fmt.Sprintf("Failed creating TOML Appium config file for device `%s` - %s", device.UDID, err))
	}
	defer file.Close()

	_, err = io.WriteString(file, string(res))
	if err != nil {
		panic(fmt.Sprintf("Failed writing to TOML Appium config file for device `%s` - %s", device.UDID, err))
	}
	port_counter++
}

func startGridNode(device *models.Device) {
	time.Sleep(5 * time.Second)
	cmd := exec.CommandContext(device.Context, "java", "-jar", fmt.Sprintf("%s/conf/%s", config.Config.EnvConfig.ProviderFolder, config.Config.EnvConfig.SeleniumJarFile), "node", fmt.Sprintf("--host %s", config.Config.EnvConfig.HostAddress), "--config", fmt.Sprintf("%s/conf/%s.toml", config.Config.EnvConfig.ProviderFolder, device.UDID), "--grid-url", config.Config.EnvConfig.SeleniumGrid)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error creating stdoutpipe while starting Selenium Grid node for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if err := cmd.Start(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Could not start Selenium Grid node for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		device.Logger.LogDebug("grid-node", strings.TrimSpace(line))
	}

	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("Error waiting for Selenium Grid node command to finish, it errored out or device `%v` was disconnected - %v", device.UDID, err))
		resetLocalDevice(device)
	}
}

func updateScreenSize(device *models.Device) error {
	if device.OS == "ios" {
		if dimensions, ok := util.IOSDeviceInfoMap[device.IOSProductType]; ok {
			device.ScreenHeight = dimensions.Height
			device.ScreenWidth = dimensions.Width
		} else {
			return fmt.Errorf("could not find `%s` hardware model in the IOSDeviceDimensions map, please update the map", device.HardwareModel)
		}
	} else {
		err := updateAndroidScreenSizeADB(device)
		if err != nil {
			return err
		}
	}

	return nil
}

func getModel(device *models.Device) {
	if device.OS == "ios" {
		if info, ok := util.IOSDeviceInfoMap[device.IOSProductType]; ok {
			device.Model = info.Model
		} else {
			device.Model = "Unknown iOS device"
		}
	} else {
		brandCmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "getprop", "ro.product.brand")
		var outBuffer bytes.Buffer
		brandCmd.Stdout = &outBuffer
		if err := brandCmd.Run(); err != nil {
			device.Model = "Unknown brand and model"
		}
		brand := outBuffer.String()
		outBuffer.Reset()

		modelCmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "getprop", "ro.product.model")
		modelCmd.Stdout = &outBuffer
		if err := modelCmd.Run(); err != nil {
			device.Model = "Unknown brand/model"
			return
		}
		model := outBuffer.String()

		device.Model = fmt.Sprintf("%s %s", strings.TrimSpace(brand), strings.TrimSpace(model))
	}
}

func getAndroidOSVersion(device *models.Device) {
	if device.OS == "ios" {

	} else {
		sdkCmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "getprop", "ro.build.version.sdk")
		var outBuffer bytes.Buffer
		sdkCmd.Stdout = &outBuffer
		if err := sdkCmd.Run(); err != nil {
			device.OSVersion = "N/A"
		}
		sdkVersion := strings.TrimSpace(outBuffer.String())
		if osVersion, ok := util.AndroidVersionToSDK[sdkVersion]; ok {
			device.OSVersion = osVersion
		} else {
			device.OSVersion = "N/A"
		}
	}
}

func UpdateInstalledApps(device *models.Device) {
	if device.OS == "ios" {
		device.InstalledApps = getInstalledAppsIOS(device)
	} else {
		device.InstalledApps = getInstalledAppsAndroid(device)
	}
}

func UninstallApp(device *models.Device, app string) error {
	if device.OS == "ios" {
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

func InstallApp(device *models.Device, app string) error {
	if device.OS == "ios" {
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

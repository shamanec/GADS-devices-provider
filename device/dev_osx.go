package device

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
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
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	log "github.com/sirupsen/logrus"
)

var usedPorts = make(map[int]bool)
var mu sync.Mutex

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

var localDevices []*LocalDevice

func getLocalDevices() {
	for _, device := range Config.Devices {
		localDevice := LocalDevice{
			Device:        device,
			ProviderState: "init",
		}
		localDevices = append(localDevices, &localDevice)

		// Create logs directory for each device if it doesn't already exist
		if _, err := os.Stat("./logs/device_" + device.UDID); os.IsNotExist(err) {
			os.Mkdir("./logs/device_"+device.UDID, os.ModePerm)
		}
	}
}

func androidDevicesInConfig() bool {
	for _, device := range Config.Devices {
		if device.OS == "android" {
			return true
		}
	}
	return false
}

func (device *LocalDevice) resetLocalDevice() {
	mu.Lock()
	defer mu.Unlock()

	device.CtxCancel()
	device.ProviderState = "init"
	device.Device.Healthy = false
	device.Device.updateDB()
}

// COMMON

// Set a context for a device to enable cancelling running goroutines related to that device when its disconnected
func (device *LocalDevice) setContext() {
	ctx, cancelFunc := context.WithCancel(context.Background())
	device.CtxCancel = cancelFunc
	device.Context = ctx
}

// ANDROID DEVICES
func (device *LocalDevice) setupAndroidDevice() {
	device.ProviderState = "preparing"

	log.WithFields(log.Fields{
		"event": "android_device_setup",
	}).Info(fmt.Sprintf("Running setup for Android device - %v", device.Device.UDID))

	isStreamAvailable, err := device.isGadsStreamServiceRunning()
	if err != nil {
		device.resetLocalDevice()
	}

	// Get a free port on the host for WebDriverAgent server
	streamPort, err := getFreePort()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_device_setup",
		}).Error(fmt.Sprintf("Could not allocate free GADS-stream port for device - %v, err - %v", device.Device.UDID, err))
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
	fmt.Println("DEVICE PORT " + device.Device.StreamPort)
}

func removeAdbForwardedPorts() {
	cmd := exec.Command("adb", "forward", "--remove-all")
	err := cmd.Run()
	if err != nil {
		fmt.Println("Could not remove adb forwarded ports")
	}
}

func (device *LocalDevice) isGadsStreamServiceRunning() (bool, error) {
	fmt.Println("INFO: Checking if gads-stream is installed and service is running")
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "dumpsys", "activity", "services", "com.shamanec.stream/.ScreenCaptureService")

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error running command:", err)
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

	err := cmd.Run()
	if err != nil {
		device.resetLocalDevice()
	}
}

// Add recording permissions to gads-stream app to avoid popup on start
func (device *LocalDevice) addGadsStreamRecordingPermissions() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "appops", "set", "com.shamanec.stream", "PROJECT_MEDIA", "allow")
	err := cmd.Run()
	if err != nil {
		device.resetLocalDevice()
	}
}

// Start the gads-stream app using adb
func (device *LocalDevice) startGadsStreamApp() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "am", "start", "-n", "com.shamanec.stream/com.shamanec.stream.ScreenCaptureActivity")
	err := cmd.Run()
	if err != nil {
		device.resetLocalDevice()
	}
}

// Press the Home button using adb to hide the transparent gads-stream activity
func (device *LocalDevice) pressHomeButton() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "input", "keyevent", "KEYCODE_HOME")
	err := cmd.Run()
	if err != nil {
		device.resetLocalDevice()
	}
}

// Forward gads-stream socket to the host container
func (device *LocalDevice) forwardGadsStream() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "forward", "tcp:"+device.Device.StreamPort, "tcp:1991")
	err := cmd.Run()
	if err != nil {
		device.resetLocalDevice()
	}
}

// IOS DEVICES

func updateIOSDevicesOSX() {
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

	// err = buildWebDriverAgent()
	// if err != nil {
	// 	fmt.Println("Could not successfully build WebDriverAgent for testing - " + err.Error())
	// 	os.Exit(1)
	// }

	getLocalDevices()
	removeAdbForwardedPorts()

	for {
		connectedDevices := getConnectedDevicesOSX(true, androidDevicesInConfig)

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

func (device *LocalDevice) setupIOSDevice() {
	device.ProviderState = "preparing"

	log.WithFields(log.Fields{
		"event": "ios_device_setup",
	}).Info(fmt.Sprintf("Running setup for iOS device - %v", device.Device.UDID))

	// Get go-ios device entry for pairing/mounting images
	// Mounting currently unused, images are mounted automatically through Xcode device setup
	// Pairing currently unused, TODO after go-ios supports iOS >=17
	goIOSDevice, err := getGoIOSDevice(device.Device.UDID)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error(fmt.Sprintf("Could not get `go-ios` DeviceEntry for device - %v, err - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
	device.GoIOSDeviceEntry = goIOSDevice

	// Get a free port on the host for WebDriverAgent server
	wdaPort, err := getFreePort()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error(fmt.Sprintf("Could not allocate free WebDriverAgent port for device - %v, err - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
	device.Device.WDAPort = fmt.Sprint(wdaPort)

	// Get a free port on the host for WebDriverAgent stream
	streamPort, err := getFreePort()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error(fmt.Sprintf("Could not allocate free WebDriverAgent stream port for device `%v`, err - %v", device.Device.UDID, err))
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
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Info(fmt.Sprintf("Successfully started WebDriverAgent for device `%v` forwarded on port %v", device.Device.UDID, device.Device.WDAPort))
		break
	case <-time.After(30 * time.Second):
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error(fmt.Sprintf("Did not successfully start WebDriverAgent on device `%v` in 30 seconds", device.Device.UDID))
		device.resetLocalDevice()
		return
	}

	// Create a WebDriverAgent session and update the MJPEG stream settings
	err = device.updateWebDriverAgent()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error(fmt.Sprintf("Did not successfully update WebDriverAgent settings for device `%v`, err - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	// Start a goroutine that periodically checks if the WebDriverAgent server is up
	go device.updateDeviceHealthStatus()

	// Mark the device as 'live' and update it in RethinkDB
	device.ProviderState = "live"
	device.Device.updateDB()
}

// Forward iOS device ports using `go-ios` CLI, for some reason using the library doesn't work properly
func (device *LocalDevice) goIOSForward(hostPort string, devicePort string) {
	cmd := exec.CommandContext(device.Context, "ios", "forward", hostPort, devicePort, "--udid="+device.Device.UDID)

	// Create a pipe to capture the command's output
	_, err := cmd.StdoutPipe()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error(fmt.Sprintf("Could not create stdoutpipe executing `ios forward` for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	// Start the port forward command
	err = cmd.Start()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error(fmt.Sprintf("Error executing `ios forward` for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}

	if err := cmd.Wait(); err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error(fmt.Sprintf("Error waiting `ios forward` to finish for device `%v` - %v", device.Device.UDID, err))
		device.resetLocalDevice()
		return
	}
}

// Gets all connected iOS and Android devices to the host
func getConnectedDevicesOSX(ios bool, android bool) []string {
	androidDevices := []string{}
	iosDevices := []string{}

	if android {
		androidDevices = getConnectedDevicesAndroid()
	}

	if ios {
		iosDevices = getConnectedDevicesIOS()
	}

	connectedDevices := []string{}
	connectedDevices = append(connectedDevices, iosDevices...)
	connectedDevices = append(connectedDevices, androidDevices...)

	return connectedDevices
}

// Gets the connected iOS devices using the `go-ios` library
func getConnectedDevicesIOS() []string {
	var connectedDevices []string

	deviceList, err := ios.ListDevices()
	if err != nil {
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
		fmt.Println("Error creating stdout pipe when getting connected android devices - ", err)
		return connectedDevices
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting command:", err)
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
		return []string{}
	}
	return connectedDevices
}

// Check if xcodebuild is available on the host by checking its version
func xcodebuildAvailable() bool {
	cmd := exec.Command("xcodebuild", "-version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// Check if adb is available on the host by starting the server
func adbAvailable() bool {
	cmd := exec.Command("adb", "start-server")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func buildWebDriverAgent() error {
	// Command to run continuously (replace with your command)
	cmd := exec.Command("xcodebuild", "-project", "WebDriverAgent.xcodeproj", "-scheme", "WebDriverAgentRunner", "-destination", "generic/platform=iOS", "build-for-testing")
	cmd.Dir = Config.EnvConfig.WDAPath

	cmd.Stderr = os.Stderr
	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating stdout pipe:", err)
		return err
	}

	fmt.Println("Starting WebDriverAgent xcodebuild with command - " + cmd.String())
	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting command:", err)
		return err
	}

	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		fmt.Println("Error waiting for command to finish:", err)
		fmt.Println("Building WebDriverAgent for testing was unsuccessful")
		os.Exit(1)
	}
	return nil
}

func (device *LocalDevice) startWdaWithXcodebuild() {
	// Create a usbmuxd.log file for Stderr
	wdaLog, err := os.Create("./logs/device_" + device.Device.UDID + "/wda.log")
	if err != nil {
		device.ProviderState = "init"
		device.CtxCancel()
		return
	}
	defer wdaLog.Close()

	// Command to run continuously (replace with your command)
	cmd := exec.CommandContext(device.Context, "xcodebuild", "-project", "WebDriverAgent.xcodeproj", "-scheme", "WebDriverAgentRunner", "-destination", "platform=iOS,id="+device.Device.UDID, "test-without-building", "-allowProvisioningUpdates")
	cmd.Dir = Config.EnvConfig.WDAPath

	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating stdout pipe:", err)
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting command:", err)
		return
	}

	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "Restarting after") {
			return
		}

		_, err := fmt.Fprintln(wdaLog, line)
		if err != nil {
			fmt.Println("Could not write to device wda.log file")
		}

		if strings.Contains(line, "ServerURLHere") {
			// device.DeviceIP = strings.Split(strings.Split(line, "//")[1], ":")[0]
			device.WdaReadyChan <- true
		}
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Error waiting for command to finish:", err)
	}
}

func (device *LocalDevice) pairIOS() error {
	log.WithFields(log.Fields{
		"event": "pair_ios_device",
	}).Info("Pairing iOS device - " + device.Device.UDID)

	p12, err := os.ReadFile("../configs/supervision.p12")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "pair_ios_device",
		}).Warn(fmt.Sprintf("Could not read /opt/supervision.p12 file when pairing device with UDID: %s, falling back to unsupervised pairing, err:%s", device.Device.UDID, err))
		err = ios.Pair(device.GoIOSDeviceEntry)
		if err != nil {
			return errors.New("Could not pair successfully, err:" + err.Error())
		}
		return nil
	}

	err = ios.PairSupervised(device.GoIOSDeviceEntry, p12, Config.EnvConfig.SupervisionPassword)
	if err != nil {
		return errors.New("Could not pair successfully, err:" + err.Error())
	}

	return nil
}

func getGoIOSDevice(udid string) (ios.DeviceEntry, error) {
	device, err := ios.GetDevice(udid)
	if err != nil {
		return ios.DeviceEntry{}, err
	}
	return device, nil
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

func InstallAppWithDevice(device ios.DeviceEntry, fileName string) error {
	filePath := fileName

	conn, err := zipconduit.New(device)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "install_app",
		}).Error("Could not create zipconduit when installing app. Error: " + err.Error())
		return errors.New("Failed installing application:" + err.Error())
	}

	err = conn.SendFile(filePath)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "install_app",
		}).Error("Could not install app. Error: " + err.Error())
		return errors.New("Failed installing application:" + err.Error())
	}
	return nil
}

// Create a new WebDriverAgent session and update stream settings
func (device *LocalDevice) updateWebDriverAgent() error {
	fmt.Println("INFO: Updating WebDriverAgent session and mjpeg stream settings")

	err := device.createWebDriverAgentSession()
	if err != nil {
		return err
	}

	err = device.updateWebDriverAgentStreamSettings()
	if err != nil {
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

func runShellCommand(ctx context.Context, command string, args ...string) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		fmt.Println("Error starting command:", err)
		return
	}

	cmd.Wait()
}

func (device *LocalDevice) updateDeviceHealthStatus() {
	for {
		select {
		case <-time.After(1 * time.Second):
			device.checkDeviceHealthStatus()
		case <-device.Context.Done():
			fmt.Println("STOPPING DEVICE HEALTH CHECK")
			return
		}
	}
}

func (device *LocalDevice) checkDeviceHealthStatus() {
	wdaGood := false
	wdaGood, err := device.isWdaHealthy()
	if err != nil {
		fmt.Println(err)
	}

	if wdaGood {
		device.Device.LastHealthyTimestamp = time.Now().UnixMilli()
		device.Device.Healthy = true
		device.Device.updateDB()
		return
	}

	device.Device.Healthy = false
	device.Device.updateDB()
}

func getFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			port = l.Addr().(*net.TCPAddr).Port
			mu.Lock()
			defer mu.Unlock()
			if _, ok := usedPorts[port]; ok {
				return getFreePort()
			}
			usedPorts[port] = true
			return port, nil
		}
	}
	return
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

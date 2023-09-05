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
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	log "github.com/sirupsen/logrus"
)

var usedPorts map[int]bool
var connectedDevices []string

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
	}
}

// COMMON

// Set a context for a device to enable cancelling running goroutines related to that device when its disconnected
func (device *LocalDevice) setContext() {
	ctx, cancelFunc := context.WithCancel(context.Background())
	device.CtxCancel = cancelFunc
	device.Context = ctx
}

// IOS DEVICES

func updateIOSDevicesOSX() {
	if !isXcodebuildAvailable() {
		fmt.Println("xcodebuild is not available, you need to set up the host as explained in the readme")
		os.Exit(1)
	}
	// Use os.Stat to check if the directory exists
	_, err := os.Stat(Config.EnvConfig.WDAPath)
	if err != nil {
		fmt.Println(Config.EnvConfig.WDAPath + " does not exist, you need to provide valid path to the WebDriverAgent repo in config.json")
		os.Exit(1)
	}

	err = buildWDA()
	if err != nil {
		fmt.Println("Could not successfully build WebDriverAgent for testing - " + err.Error())
		os.Exit(1)
	}

	getLocalDevices()

	for {
		getConnectedDevicesIOS()

		if len(connectedDevices) == 0 {
			log.WithFields(log.Fields{
				"event": "update_devices",
			}).Info("No devices connected")

			for _, device := range localDevices {
				device.Device.Connected = false
				device.ProviderState = "init"
				device.CtxCancel()
			}
		} else {
			for _, device := range localDevices {
				if slices.Contains(connectedDevices, device.Device.UDID) {
					device.Device.Connected = true
					if device.ProviderState != "preparing" && device.ProviderState != "live" {
						device.setContext()
						device.WdaReadyChan = make(chan bool, 1)
						go device.setupIOSDevice()
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

	goIOSDevice, err := getGoIOSDevice(device.Device.UDID)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error("Could not get go-ios device - " + err.Error())
		device.ProviderState = "init"
		fmt.Println()
		return
	}
	device.GoIOSDeviceEntry = goIOSDevice

	wdaPort, err := getFreePort()
	if err != nil {
		fmt.Println("PORT FAIL")
		device.ProviderState = "init"
		return
	}
	device.Device.WDAPort = fmt.Sprint(wdaPort)

	streamPort, err := getFreePort()
	if err != nil {
		fmt.Println("PORT FAIL")
		device.ProviderState = "init"
		return
	}
	device.Device.StreamPort = fmt.Sprint(streamPort)

	go device.goIOSForward(device.Device.WDAPort, "8100")
	go device.goIOSForward(device.Device.StreamPort, "9100")

	go device.startWdaXcodebuild()

	select {
	case <-device.WdaReadyChan:
		fmt.Println("WebDriverAgent was successfully started")
		break
	case <-time.After(30 * time.Second):
		fmt.Println("WebDriverAgent was not started in 30 seconds")
		device.CtxCancel()
		device.ProviderState = "init"
		return
	}

	// err = device.updateWebDriverAgent()
	// if err != nil {
	// 	fmt.Println("ERROR WDA UPDATE - " + err.Error())
	// 	device.ProviderState = "init"
	// 	return
	// }

	go device.updateDeviceHealthStatus()

	device.ProviderState = "live"
	device.Device.updateDB()
}

func (device *LocalDevice) goIOSForward(hostPort string, devicePort string) {
	cmd := exec.CommandContext(device.Context, "ios", "forward", hostPort, devicePort)

	// Create a pipe to capture the command's output
	_, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error creating stdout pipe: %v\n", err)
		return
	}

	err = cmd.Start()
	if err != nil {
		return
	}

	if err := cmd.Wait(); err != nil {
		return
	}
}

func getConnectedDevicesIOS() {
	deviceList, err := ios.ListDevices()
	if err != nil {
		connectedDevices = []string{}
		return
	}

	for _, connDevice := range deviceList.DeviceList {
		if !slices.Contains(connectedDevices, connDevice.Properties.SerialNumber) {
			connectedDevices = append(connectedDevices, connDevice.Properties.SerialNumber)
		}
	}
}

// Check if xcodebuild is available on the host by checking its version
func isXcodebuildAvailable() bool {
	cmd := exec.Command("xcodebuild", "-version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func buildWDA() error {
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

func (device *LocalDevice) startWdaXcodebuild() {
	// Command to run continuously (replace with your command)
	cmd := exec.CommandContext(device.Context, "xcodebuild", "-project", "WebDriverAgent.xcodeproj", "-scheme", "WebDriverAgentRunner", "-destination", "platform=iOS,id="+device.Device.UDID, "test-without-building")
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
		fmt.Println(line)

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
	fmt.Println(string(responseBody))
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
			if _, ok := usedPorts[port]; ok {
				return getFreePort()
			}
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

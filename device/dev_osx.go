package device

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/codeskyblue/go-sh"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/forward"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	log "github.com/sirupsen/logrus"
)

var usedPorts map[int]bool

var netClient = &http.Client{
	Timeout: time.Second * 120,
}

func Test() {
	go updateDevicesOSX()
	time.Sleep(5 * time.Second)
	go setupDevices()
}

func updateDevicesOSX() {
	log.WithFields(log.Fields{
		"event": "update_devices_connected_state",
	}).Info("Updating iOS devices connection state")

	connectedDevices, err := getConnectedDevicesIOS()
	if err != nil {
		panic("Could not get devices: " + err.Error())
	}

	for {
		for _, device := range Config.Devices {
			device.Mu.Lock()
			device.Connected = false
			for _, connectedDevice := range connectedDevices {
				if connectedDevice == device.UDID {
					device.Connected = true
				}
			}
			device.Mu.Unlock()
		}
		time.Sleep(15 * time.Second)
	}
}

func setupDevices() {
	for _, device := range Config.Devices {
		if device.Connected && device.ProviderState != "setup" && device.ProviderState != "live" {
			go device.setupIOSDevice()
		} else if !device.Connected && device.ProviderState == "setup" || device.ProviderState == "live" {
			fmt.Println("TEST")
		}
	}
}

func getConnectedDevicesIOS() ([]string, error) {
	deviceList, err := ios.ListDevices()
	if err != nil {
		return nil, err
	}
	var connDevices []string
	for _, connDevice := range deviceList.DeviceList {
		connDevices = append(connDevices, connDevice.Properties.SerialNumber)
	}

	return connDevices, nil
}

func (device *Device) setupIOSDevice() error {
	device.ProviderState = "setup"

	goIOSDevice, _ := getGoIOSDevice(device.UDID)
	device.GoIOSDevice = goIOSDevice

	device.pairIOS()
	device.mountDeveloperImageIOS()
	device.forwardIOS(8100, 8100)
	device.forwardIOS(9100, 9100)
	device.installAndStartWebDriverAgent()
	time.Sleep(3 * time.Second)
	updateWebDriverAgent()

	return nil
}

func (device *Device) pairIOS() error {
	log.WithFields(log.Fields{
		"event": "pair_ios_device",
	}).Info("Pairing iOS device - " + device.UDID)

	p12, err := os.ReadFile("../configs/supervision.p12")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "pair_ios_device",
		}).Warn(fmt.Sprintf("Could not read /opt/supervision.p12 file when pairing device with UDID: %s, falling back to unsupervised pairing, err:%s", device.UDID, err))
		err = ios.Pair(device.GoIOSDevice)
		if err != nil {
			return errors.New("Could not pair successfully, err:" + err.Error())
		}
		return nil
	}

	err = ios.PairSupervised(device.GoIOSDevice, p12, Config.EnvConfig.SupervisionPassword)
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
func (device *Device) mountDeveloperImageIOS() error {
	basedir := "./devimages"

	var err error
	path, err := imagemounter.DownloadImageFor(device.GoIOSDevice, basedir)
	if err != nil {
		return err
	}

	err = imagemounter.MountImage(device.GoIOSDevice, path)
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

func (device *Device) installAndStartWebDriverAgent() error {
	err := InstallAppWithDevice(device.GoIOSDevice, projectDir+"/apps/WebDriverAgent.ipa")
	if err != nil {
		fmt.Println(err)
		return err
	}

	go startWebDriverAgent(device.UDID)
	return nil
}

// Start the WebDriverAgent on the device
func startWebDriverAgent(udid string) {
	fmt.Println("INFO: Starting WebDriverAgent")
	outfile, err := os.Create(projectDir + "/opt/logs/wda.log")
	if err != nil {
		panic("Could not create /opt/logs/wda.log file, err:" + err.Error())
	}
	defer outfile.Close()

	session := sh.NewSession()
	session.Stdout = outfile
	session.Stderr = outfile

	// Lazy way to do this using go-ios binary, should some day update to use go-ios modules instead!!!
	err = session.Command("ios", "runwda", "--bundleid="+Config.EnvConfig.WDABundleID, "--testrunnerbundleid="+Config.EnvConfig.WDABundleID, "--xctestconfig=WebDriverAgentRunner.xctest", "--udid="+udid).Run()
	if err != nil {
		panic("Running WebDriverAgent using go-ios binary failed, err:" + err.Error())
	}
}

// Create a new WebDriverAgent session and update stream settings
func updateWebDriverAgent() error {
	fmt.Println("INFO: Updating WebDriverAgent session and mjpeg stream settings")

	sessionID, err := createWebDriverAgentSession()
	if err != nil {
		return err
	}

	err = updateWebDriverAgentStreamSettings(sessionID)
	if err != nil {
		return err
	}

	return nil
}

func updateWebDriverAgentStreamSettings(sessionID string) error {
	// Set 30 frames per second, without any scaling, half the original screenshot quality
	// TODO should make this configurable in some way, although can be easily updated the same way
	requestString := `{"settings": {"mjpegServerFramerate": 30, "mjpegServerScreenshotQuality": 50, "mjpegScalingFactor": 100}}`

	// Post the mjpeg server settings
	response, err := http.Post("http://localhost:8100/session/"+sessionID+"/appium/settings", "application/json", strings.NewReader(requestString))
	if err != nil {
		return err
	}

	if response.StatusCode != 200 {
		return errors.New("Could not successfully update WDA stream settings, status code=" + strconv.Itoa(response.StatusCode))
	}

	return nil
}

// Create a new WebDriverAgent session
func createWebDriverAgentSession() (string, error) {
	// TODO see if this JSON can be simplified
	requestString := `{
		"capabilities": {
			"firstMatch": [{}],
			"alwaysMatch": {
				
			}
		}
	}`

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8100/session", strings.NewReader(requestString))
	if err != nil {
		return "", err
	}

	response, err := netClient.Do(req)
	if err != nil {
		return "", err
	}

	fmt.Println("UPDATED SETTINGS")
	fmt.Println(time.Now())

	// Post to create new session
	// response, err := http.Post("http://localhost:8100/session", "application/json", strings.NewReader(requestString))
	// if err != nil {
	// 	return "", err
	// }

	// Get the response into a byte slice
	responseBody, _ := io.ReadAll(response.Body)

	// Unmarshal response into a basic map
	var responseJson map[string]interface{}
	err = json.Unmarshal(responseBody, &responseJson)
	if err != nil {
		return "", err
	}

	// Check the session ID from the map
	if responseJson["sessionId"] == "" {
		if err != nil {
			return "", errors.New("Could not get `sessionId` while creating a new WebDriverAgent session")
		}
	}

	return fmt.Sprintf("%v", responseJson["sessionId"]), nil
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

// Forward with context
func (device *Device) forwardIOS(hostPort uint16, phonePort uint16) error {
	log.Infof("Start listening on port %d forwarding to port %d on device", hostPort, phonePort)
	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", hostPort))
	if err != nil {
		return err
	}

	go connectionAcceptIOS(device.Ctx, l, device.GoIOSDevice.DeviceID, phonePort)

	return nil
}

func connectionAcceptIOS(ctx context.Context, l net.Listener, deviceID int, phonePort uint16) {
	for {
		clientConn, err := l.Accept()
		if err != nil {
			log.Errorf("Error accepting new connection %v", err)
			continue
		}
		log.WithFields(log.Fields{"conn": fmt.Sprintf("%#v", clientConn)}).Info("new client connected")
		go forward.StartNewProxyConnection(ctx, clientConn, deviceID, phonePort)
	}
}

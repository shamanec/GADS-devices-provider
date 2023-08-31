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
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codeskyblue/go-sh"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/forward"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	log "github.com/sirupsen/logrus"
)

var usedPorts map[int]bool
var connectedDevices []string
var cancelMap = make(map[string]context.CancelFunc)
var cancelMapMu sync.Mutex

// var tr = &http.Transport{
// 	MaxIdleConns:        20,
// 	MaxIdleConnsPerHost: 20,
// 	MaxConnsPerHost:     20,
// }

var netClient = &http.Client{
	Timeout: time.Second * 120,
	// Transport: tr,
}

// COMMON

// Set a context for a device to enable cancelling running goroutines related to that device when its disconnected
func (device *Device) setContext() {
	cancelMapMu.Lock()
	defer cancelMapMu.Unlock()

	ctx, cancelFunc := context.WithCancel(context.Background())
	cancelMap[device.UDID] = cancelFunc
	device.Ctx = ctx
}

// IOS DEVICES

func updateDevicesOSX() {
	for {
		getConnectedDevicesIOS()

		if len(connectedDevices) == 0 {
			log.WithFields(log.Fields{
				"event": "update_devices",
			}).Info("No devices connected")

			for _, device := range Config.Devices {
				device.Connected = false
				device.ProviderState = "init"
				if _, ok := cancelMap[device.UDID]; ok {
					cancelMap[device.UDID]()
					delete(cancelMap, device.UDID)
				}
			}
		} else {
			for _, device := range Config.Devices {
				if slices.Contains(connectedDevices, device.UDID) {
					device.Connected = true
					if device.ProviderState != "preparing" && device.ProviderState != "live" {
						device.setContext()
						go device.setupIOSDevice()
					}
				} else {
					device.Connected = false
				}
			}
		}
		time.Sleep(10 * time.Second)
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

func (device *Device) setupIOSDevice() {
	device.ProviderState = "preparing"

	goIOSDevice, err := getGoIOSDevice(device.UDID)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_device_setup",
		}).Error("Could not get go-ios device - " + err.Error())
		device.ProviderState = "init"
		return
	}

	device.GoIOSDevice = goIOSDevice

	err = device.pairIOS()
	if err != nil {
		device.ProviderState = "init"
		return
	}

	err = device.mountDeveloperImageIOS()
	if err != nil {
		device.ProviderState = "init"
		return
	}

	wdaPort, err := getFreePort()
	if err != nil {
		device.ProviderState = "init"
		return
	}
	device.WDAPort = fmt.Sprint(wdaPort)

	err = device.forwardIOS(wdaPort, 8100)
	if err != nil {
		device.ProviderState = "init"
		return
	}

	streamPort, err := getFreePort()
	if err != nil {
		device.ProviderState = "init"
		return
	}
	device.StreamPort = fmt.Sprint(streamPort)

	err = device.forwardIOS(streamPort, 9100)
	if err != nil {
		device.ProviderState = "init"
		return
	}

	err = device.installAndStartWebDriverAgent()
	if err != nil {
		device.ProviderState = "init"
		return
	}

	time.Sleep(20 * time.Second)
	err = device.updateWebDriverAgent()
	if err != nil {
		device.ProviderState = "init"
		return
	}
	fmt.Println(device.WDAPort)
	fmt.Println(device.StreamPort)

	device.ProviderState = "live"
	device.updateDB()
	go device.updateDeviceHealthStatus()
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
func (device *Device) updateWebDriverAgent() error {
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

func (device *Device) updateWebDriverAgentStreamSettings() error {
	// Set 30 frames per second, without any scaling, half the original screenshot quality
	// TODO should make this configurable in some way, although can be easily updated the same way
	requestString := `{"settings": {"mjpegServerFramerate": 30, "mjpegServerScreenshotQuality": 30, "mjpegScalingFactor": 100}}`

	// Post the mjpeg server settings
	response, err := http.Post("http://localhost:"+device.WDAPort+"/session/"+device.WDASessionID+"/appium/settings", "application/json", strings.NewReader(requestString))
	if err != nil {
		return err
	}

	if response.StatusCode != 200 {
		return errors.New("Could not successfully update WDA stream settings, status code=" + strconv.Itoa(response.StatusCode))
	}

	return nil
}

// Create a new WebDriverAgent session
func (device *Device) createWebDriverAgentSession() error {
	// TODO see if this JSON can be simplified
	requestString := `{
		"capabilities": {
			"firstMatch": [{}],
			"alwaysMatch": {
				
			}
		}
	}`

	req, err := http.NewRequest(http.MethodPost, "http://localhost:"+device.WDAPort+"/session", strings.NewReader(requestString))
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

	device.WDASessionID = fmt.Sprintf("%v", responseJson["sessionId"])
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

// Forward with context
func (device *Device) forwardIOS(hostPort int, phonePort int) error {
	log.Infof("Start listening on port %d forwarding to port %d on device", hostPort, phonePort)
	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", hostPort))
	if err != nil {
		return err
	}

	go connectionAcceptIOS(device.Ctx, l, device.GoIOSDevice.DeviceID, uint16(phonePort))

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

func (device *Device) updateDeviceHealthStatus() {
	for {
		select {
		case <-time.After(1 * time.Second):
			device.checkDeviceHealthStatus()
		case <-device.Ctx.Done():
			fmt.Println("Stopping device health check")
			return
		}
	}
}

func (device *Device) checkDeviceHealthStatus() {
	allGood := false
	appiumGood := true
	wdaGood := true

	if Config.AppiumConfig.UseAppium {
		appiumGood = false
		appiumGood, _ = device.appiumHealthy()
	}

	if appiumGood && device.OS == "ios" {
		wdaGood, _ = device.wdaHealthy()
	}

	allGood = appiumGood && wdaGood

	if allGood {
		device.LastHealthyTimestamp = time.Now().UnixMilli()
		device.Healthy = true
		device.updateDB()
		return
	}

	device.Healthy = false
	device.updateDB()
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

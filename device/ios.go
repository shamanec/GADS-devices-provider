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
	"strconv"
	"strings"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/shamanec/GADS-devices-provider/util"
	log "github.com/sirupsen/logrus"
)

// Check if xcodebuild is available on the host by checking its version
func xcodebuildAvailable() bool {
	cmd := exec.Command("xcodebuild", "-version")
	util.ProviderLogger.LogDebug("provider", "Checking if xcodebuild is available on host")

	if err := cmd.Run(); err != nil {
		util.ProviderLogger.LogDebug("provider", fmt.Sprintf("xcodebuild is not available or command failed - %s", err))
		return false
	}
	return true
}

// Check if go-ios binary is available
func goIOSAvailable() bool {
	cmd := exec.Command("ios", "-h")
	util.ProviderLogger.LogDebug("provider", "Checking if go-ios binary is available on host")

	if err := cmd.Run(); err != nil {
		util.ProviderLogger.LogDebug("provider", fmt.Sprintf("go-ios is not available on host or command failed - %s", err))
		return false
	}
	return true
}

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

// Build WebDriverAgent for testing with `xcodebuild`
func buildWebDriverAgent() error {
	cmd := exec.Command("xcodebuild", "-project", "WebDriverAgent.xcodeproj", "-scheme", "WebDriverAgentRunner", "-destination", "generic/platform=iOS", "build-for-testing")
	cmd.Dir = util.Config.EnvConfig.WDAPath

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
	logger, _ := util.CreateCustomLogger("./logs/device_"+device.Device.UDID+"/wda.log", device.Device.UDID)

	cmd := exec.CommandContext(device.Context, "xcodebuild", "-project", "WebDriverAgent.xcodeproj", "-scheme", "WebDriverAgentRunner", "-destination", "platform=iOS,id="+device.Device.UDID, "test-without-building", "-allowProvisioningUpdates")
	cmd.Dir = util.Config.EnvConfig.WDAPath

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

// Get go-ios device entry to use library directly, instead of CLI binary
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

	p12, err := os.ReadFile("./config/supervision.p12")
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
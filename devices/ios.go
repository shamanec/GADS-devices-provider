package devices

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/shamanec/GADS-devices-provider/config"
	"github.com/shamanec/GADS-devices-provider/logger"
	"github.com/shamanec/GADS-devices-provider/models"
)

// Forward iOS device ports using `go-ios` CLI, for some reason using the library doesn't work properly
func goIOSForward(device *models.Device, hostPort string, devicePort string) {
	cmd := exec.CommandContext(device.Context, "ios", "forward", hostPort, devicePort, "--udid="+device.UDID)

	// Create a pipe to capture the command's output
	_, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("goIOSForward: Could not create stdoutpipe executing `ios forward` for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	// Start the port forward command
	err = cmd.Start()
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("goIOSForward: Error executing `ios forward` for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("goIOSForward: Error waiting `ios forward` to finish for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}
}

// Start the prebuilt WebDriverAgent with `xcodebuild`
func startWdaWithXcodebuild(device *models.Device) {
	cmd := exec.CommandContext(device.Context, "xcodebuild", "-project", "WebDriverAgent.xcodeproj", "-scheme", "WebDriverAgentRunner", "-destination", "platform=iOS,id="+device.UDID, "test-without-building", "-allowProvisioningUpdates")
	cmd.Dir = config.Config.EnvConfig.WdaRepoPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		device.Logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("startWdaWithXcodebuild: Error creating stdoutpipe while running WebDriverAgent with xcodebuild for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	if err := cmd.Start(); err != nil {
		device.Logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("startWdaWithXcodebuild: Could not start WebDriverAgent with xcodebuild for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()

		// device.Logger.LogInfo("webdriveragent", strings.TrimSpace(line))

		if strings.Contains(line, "Restarting after") {
			resetLocalDevice(device)
			return
		}

		if strings.Contains(line, "ServerURLHere") {
			// device.DeviceIP = strings.Split(strings.Split(line, "//")[1], ":")[0]
			device.WdaReadyChan <- true
		}
	}

	if err := cmd.Wait(); err != nil {
		device.Logger.LogError("webdriveragent_xcodebuild", fmt.Sprintf("startWdaWithXcodebuild: Error waiting for WebDriverAgent(xcodebuild) command to finish, it errored out or device `%v` was disconnected - %v", device.UDID, err))
		resetLocalDevice(device)
	}
}

// Create a new WebDriverAgent session and update stream settings
func updateWebDriverAgent(device *models.Device) error {
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Updating WebDriverAgent session and mjpeg stream settings for device `%s`", device.UDID))

	err := createWebDriverAgentSession(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("updateWebDriverAgent: Could not create WebDriverAgent session for device %v - %v", device.UDID, err))
		return err
	}

	err = updateWebDriverAgentStreamSettings(device)
	if err != nil {
		logger.ProviderLogger.LogError("ios_device_setup", fmt.Sprintf("updateWebDriverAgent: Could not update WebDriverAgent stream settings for device %v - %v", device.UDID, err))
		return err
	}

	return nil
}

func updateWebDriverAgentStreamSettings(device *models.Device) error {
	// Set 30 frames per second, without any scaling, half the original screenshot quality
	// TODO should make this configurable in some way, although can be easily updated the same way
	requestString := `{"settings": {"mjpegServerFramerate": 30, "mjpegServerScreenshotQuality": 75, "mjpegScalingFactor": 100}}`

	// Post the mjpeg server settings
	response, err := http.Post("http://localhost:"+device.WDAPort+"/session/"+device.WDASessionID+"/appium/settings", "application/json", strings.NewReader(requestString))
	if err != nil {
		return err
	}

	if response.StatusCode != 200 {
		return fmt.Errorf("updateWebDriverAgentStreamSettings: Could not successfully update WDA stream settings, status code=%v", response.StatusCode)
	}

	return nil
}

// Create a new WebDriverAgent session
func createWebDriverAgentSession(device *models.Device) error {
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
			return fmt.Errorf("createWebDriverAgentSession: Could not get `sessionId` while creating a new WebDriverAgent session")
		}
	}

	device.WDASessionID = fmt.Sprintf("%v", responseJson["sessionId"])
	return nil
}

func startWdaWithGoIOS(device *models.Device) {

	cmd := exec.CommandContext(context.Background(), "ios", "runwda", "--bundleid="+config.Config.EnvConfig.WdaBundleID, "--testrunnerbundleid="+config.Config.EnvConfig.WdaBundleID, "--xctestconfig=WebDriverAgentRunner.xctest", "--udid="+device.UDID)

	// Create a pipe to capture the command's output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startWdaWithGoIOS: Error creating stdoutpipe while running WebDriverAgent with go-ios for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	// Create a pipe to capture the command's error output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startWdaWithGoIOS: Error creating stderrpipe while running WebDriverAgent with go-ios for device `%v` - %v", device.UDID, err))
		resetLocalDevice(device)
		return
	}

	err = cmd.Start()
	if err != nil {
		logger.ProviderLogger.LogError("device_setup", fmt.Sprintf("startWdaWithGoIOS: Failed executing `%s` - %v", cmd.Path, err))
		resetLocalDevice(device)
		return
	}

	// Create a combined reader from stdout and stderr
	combinedReader := io.MultiReader(stderr, stdout)
	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(combinedReader)

	for scanner.Scan() {
		line := scanner.Text()

		// device.Logger.LogDebug("webdriveragent", strings.TrimSpace(line))

		if strings.Contains(line, "ServerURLHere") {
			// device.DeviceIP = strings.Split(strings.Split(line, "//")[1], ":")[0]
			device.WdaReadyChan <- true
		}
	}

	err = cmd.Wait()
	if err != nil {
		device.Logger.LogError("webdriveragent", fmt.Sprintf("startWdaWithGoIOS: Error waiting for `%s` to finish, it errored out or device `%v` was disconnected - %v", cmd.Path, device.UDID, err))
		resetLocalDevice(device)
	}
}

func mountDeveloperImageIOS(device *models.Device) error {
	basedir := fmt.Sprintf("%s/devimages", config.Config.EnvConfig.ProviderFolder)

	var err error
	path, err := imagemounter.DownloadImageFor(device.GoIOSDeviceEntry, basedir)
	if err != nil {
		return fmt.Errorf("Could not download developer disk image with go-ios - %s", err)
	}

	err = imagemounter.MountImage(device.GoIOSDeviceEntry, path)
	if err != nil {
		return fmt.Errorf("Could not mount developer disk image with go-ios - %s", err)
	}

	return nil
}

func pairIOS(device *models.Device) error {
	logger.ProviderLogger.LogInfo("ios_device_setup", fmt.Sprintf("Pairing device `%s`", device.UDID))

	p12, err := os.ReadFile(fmt.Sprintf("%s/conf/supervision.p12", config.Config.EnvConfig.ProviderFolder))
	if err != nil {
		logger.ProviderLogger.LogWarn("ios_device_setup", fmt.Sprintf("Could not read supervision.p12 file when pairing device with UDID: %s, falling back to unsupervised pairing - %s", device.UDID, err))
		err = ios.Pair(device.GoIOSDeviceEntry)
		if err != nil {
			return fmt.Errorf("Could not perform unsupervised pairing successfully - %s", err)
		}
		return nil
	}

	err = ios.PairSupervised(device.GoIOSDeviceEntry, p12, config.Config.EnvConfig.SupervisionPassword)
	if err != nil {
		return fmt.Errorf("Could not perform supervised pairing successfully - %s", err)
	}

	return nil
}

func getInstalledAppsIOS(device *models.Device) []string {
	var installedApps []string
	cmd := exec.CommandContext(device.Context, "ios", "apps", "--udid="+device.UDID)

	device.InstalledApps = []string{}

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("getInstalledAppsIOS: Failed executing `%s` to get installed apps - %v", cmd.Path, err))
		return installedApps
	}

	// Get the command output json string
	jsonString := strings.TrimSpace(outBuffer.String())

	var appsData []struct {
		BundleID string `json:"CFBundleIdentifier"`
	}

	err := json.Unmarshal([]byte(jsonString), &appsData)
	if err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("getInstalledAppsIOS: Error unmarshalling `%s` output json - %v", cmd.Path, err))
		return installedApps
	}

	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
	for _, appData := range appsData {
		installedApps = append(installedApps, appData.BundleID)
	}

	return installedApps
}

func uninstallAppIOS(device *models.Device, bundleID string) error {
	cmd := exec.CommandContext(device.Context, "ios", "uninstall", bundleID, "--udid="+device.UDID)
	err := cmd.Run()
	if err != nil {
		device.Logger.LogError("uninstall_app", fmt.Sprintf("uninstallAppIOS: Failed executing `%s` - %v", cmd.Path, err))
		return err
	}

	return nil
}

func installAppWithPathIOS(device *models.Device, path string) error {
	if config.Config.EnvConfig.OS == "windows" {
		if strings.HasPrefix(path, "./") {
			path = strings.TrimPrefix(path, "./")
		}
	}

	cmd := exec.CommandContext(device.Context, "ios", "install", fmt.Sprintf("--path=%s", path), "--udid="+device.UDID)
	fmt.Println(cmd.Args)
	if err := cmd.Run(); err != nil {
		device.Logger.LogError("uninstall_app", fmt.Sprintf("Failed executing `%s` - %v", cmd.Path, err))
		return err
	}

	return nil
}

func installAppIOS(device *models.Device, appName string) error {
	cmd := exec.CommandContext(device.Context, "ios", "install", fmt.Sprintf("--path=%s/apps/%s", config.Config.EnvConfig.ProviderFolder, appName), "--udid="+device.UDID)
	if err := cmd.Run(); err != nil {
		device.Logger.LogError("uninstall_app", fmt.Sprintf("Failed executing `%s` - %v", cmd.Path, err))
		return err
	}

	return nil
}

func isAboveIOS17(device *models.Device) (bool, error) {
	majorVersion := strings.Split(device.OSVersion, ".")[0]
	convertedVersion, err := strconv.Atoi(majorVersion)
	if err != nil {
		return false, fmt.Errorf("isAboveIOS17: Failed converting `%s` to int - %s", majorVersion, err)
	}
	if convertedVersion >= 17 {
		return true, nil
	}
	return false, nil
}

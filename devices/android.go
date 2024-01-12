package devices

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/shamanec/GADS-devices-provider/config"
	"github.com/shamanec/GADS-devices-provider/logger"
	"github.com/shamanec/GADS-devices-provider/models"
)

// Check if adb is available on the host by starting the server
func adbAvailable() bool {
	cmd := exec.Command("adb", "start-server")
	logger.ProviderLogger.LogInfo("provider", "Checking if adb is available on host")

	if err := cmd.Run(); err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("adb is not available or command failed - %s", err))
		return false
	}
	return true
}

// Remove all adb forwarded ports(if any) on provider start
func removeAdbForwardedPorts() {
	logger.ProviderLogger.LogInfo("provider", "Attempting to remove all `adb` forwarded ports on provider start")

	cmd := exec.Command("adb", "forward", "--remove-all")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not remove `adb` forwarded ports, there was an error or no devices are connected - %s", err))
	}
}

// Check if the GADS-stream service is running on the device
func isGadsStreamServiceRunning(device *models.Device) (bool, error) {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "dumpsys", "activity", "services", "com.shamanec.stream/.ScreenCaptureService")
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Checking if GADS-stream is already running on device `%v`", device.UDID))

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
func installGadsStream(device *models.Device) error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "install", "-r", fmt.Sprintf("%s/apps/gads-stream.apk", config.Config.EnvConfig.ProviderFolder))
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Installing GADS-stream apk on device `%v`", device.UDID))

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func uninstallGadsStream(device *models.Device) error {
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Uninstalling GADS-stream from device `%v`", device.UDID))
	return UninstallApp(device, "com.shamanec.stream")
}

// Add recording permissions to gads-stream app to avoid popup on start
func addGadsStreamRecordingPermissions(device *models.Device) error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "appops", "set", "com.shamanec.stream", "PROJECT_MEDIA", "allow")
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Adding GADS-stream recording permissions on device `%v`", device.UDID))

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// Start the gads-stream app using adb
func startGadsStreamApp(device *models.Device) error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "am", "start", "-n", "com.shamanec.stream/com.shamanec.stream.ScreenCaptureActivity")
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Starting GADS-stream app on device `%v` with command `%v`", device.UDID, cmd.Path))

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// Press the Home button using adb to hide the transparent gads-stream activity
func pressHomeButton(device *models.Device) {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "input", "keyevent", "KEYCODE_HOME")
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Pressing Home button with adb on device `%v`", device.UDID))

	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not 'press' Home button with `adb` on device - `%v`, you need to press it yourself to hide the transparent activity of GADS-stream:\n %v", device.UDID, err))
	}
}

func forwardGadsStream(device *models.Device) error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "forward", "tcp:"+device.StreamPort, "tcp:1991")
	logger.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Forwarding GADS-stream port(1991) to host port `%v` for device `%v`", device.StreamPort, device.UDID))

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func updateAndroidScreenSizeADB(device *models.Device) error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "wm", "size")

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error executing command - %s", err)
	}

	output := outBuffer.String()
	// Some devices return more than one line with device screen info
	// Physical size and Override size
	// Thats why we'll process the response respectively
	// Specifically this was applied when caught on Samsung S20 and S9, might apply for others
	lines := strings.Split(output, "\n")
	// If the split lines are 2 then we have only one size returned
	// and one empty line
	if len(lines) == 2 {
		splitOutput := strings.Split(lines[0], ": ")
		screenDimensions := strings.Split(splitOutput[1], "x")

		device.ScreenWidth = strings.TrimSpace(screenDimensions[0])
		device.ScreenHeight = strings.TrimSpace(screenDimensions[1])
	}

	// If the split lines are 3 then we have two sizes returned
	// and one empty line
	// We need the second size here
	if len(lines) == 3 {
		splitOutput := strings.Split(lines[1], ": ")
		screenDimensions := strings.Split(splitOutput[1], "x")

		device.ScreenWidth = strings.TrimSpace(screenDimensions[0])
		device.ScreenHeight = strings.TrimSpace(screenDimensions[1])
	}

	return nil
}

func getInstalledAppsAndroid(device *models.Device) []string {
	var installedApps = []string{}
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "shell", "cmd", "package", "list", "packages", "-3")

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("Failed running ios apps command to get installed apps - %v", err))
		return installedApps
	}

	// Get the command output to string
	result := strings.TrimSpace(outBuffer.String())
	// Get all lines with package names
	lines := strings.Split(result, "\n")

	// Clean the package names and add them to the device installed apps
	for _, line := range lines {
		packageName := strings.Split(line, ":")[1]
		installedApps = append(installedApps, packageName)
	}

	return installedApps
}

func uninstallAppAndroid(device *models.Device, packageName string) error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "uninstall", packageName)

	if err := cmd.Run(); err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("Failed executing adb uninstall for package name `%s` - %v", packageName, err))
		return err
	}

	return nil
}

func installAppAndroid(device *models.Device, appName string) error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.UDID, "install", "-r", fmt.Sprintf("%s/apps/%s/", config.Config.EnvConfig.ProviderFolder, appName))

	if err := cmd.Run(); err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("Failed executing adb install for app `%s` - %v", appName, err))
		return err
	}

	return nil
}

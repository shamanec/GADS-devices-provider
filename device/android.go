package device

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/shamanec/GADS-devices-provider/util"
)

// Check if adb is available on the host by starting the server
func adbAvailable() bool {
	cmd := exec.Command("adb", "start-server")
	util.ProviderLogger.LogInfo("provider", "Checking if adb is available on host")

	if err := cmd.Run(); err != nil {
		util.ProviderLogger.LogDebug("provider", fmt.Sprintf("adb is not available or command failed - %s", err))
		return false
	}
	return true
}

// Remove all adb forwarded ports(if any) on provider start
func removeAdbForwardedPorts() {
	util.ProviderLogger.LogInfo("provider", "Attempting to remove all `adb` forwarded ports on provider start")

	cmd := exec.Command("adb", "forward", "--remove-all")
	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogDebug("provider", fmt.Sprintf("Could not remove `adb` forwarded ports, there was an error or no devices are connected - %s", err))
	}
}

// Check if the GADS-stream service is running on the device
func (device *LocalDevice) isGadsStreamServiceRunning() (bool, error) {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "dumpsys", "activity", "services", "com.shamanec.stream/.ScreenCaptureService")
	util.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Checking if GADS-stream is already running on device `%v`", device.Device.UDID))

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
func (device *LocalDevice) installGadsStream() error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "install", "-r", "./apps/gads-stream.apk")
	util.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Installing GADS-stream apk on device `%v`", device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// Add recording permissions to gads-stream app to avoid popup on start
func (device *LocalDevice) addGadsStreamRecordingPermissions() error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "appops", "set", "com.shamanec.stream", "PROJECT_MEDIA", "allow")
	util.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Adding GADS-stream recording permissions on device `%v`", device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// Start the gads-stream app using adb
func (device *LocalDevice) startGadsStreamApp() error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "am", "start", "-n", "com.shamanec.stream/com.shamanec.stream.ScreenCaptureActivity")
	util.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Starting GADS-stream app on device `%v` with command `%v`", device.Device.UDID, cmd.Path))

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// Press the Home button using adb to hide the transparent gads-stream activity
func (device *LocalDevice) pressHomeButton() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "input", "keyevent", "KEYCODE_HOME")
	util.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Pressing Home button with adb on device `%v`", device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not 'press' Home button with `adb` on device - `%v`, you need to press it yourself to hide the transparent activity of GADS-stream:\n %v", device.Device.UDID, err))
	}
}

func (device *LocalDevice) forwardGadsStream() error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "forward", "tcp:"+device.Device.StreamPort, "tcp:1991")
	util.ProviderLogger.LogInfo("android_device_setup", fmt.Sprintf("Forwarding GADS-stream port(1991) to host port `%v` for device `%v`", device.Device.StreamPort, device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func updateAndroidScreenSizeADB(device *LocalDevice) error {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "wm", "size")

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error executing command - %s", err)
	}

	output := outBuffer.String()
	splitOutput := strings.Split(output, ": ")
	screenDimensions := strings.Split(splitOutput[1], "x")

	device.Device.ScreenWidth = strings.TrimSpace(screenDimensions[0])
	device.Device.ScreenHeight = strings.TrimSpace(screenDimensions[1])

	return nil
}

func getInstalledAppsAndroid(device *LocalDevice) []string {
	var installedApps = []string{}
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "cmd", "package", "list", "packages", "-3")

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	if err := cmd.Run(); err != nil {
		device.Logger.LogError("get_installed_apps", fmt.Sprintf("Failed running ios apps command to get installed apps - %v", device.Device.UDID, err))
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

package device

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/shamanec/GADS-devices-provider/util"
	log "github.com/sirupsen/logrus"
)

// Check if adb is available on the host by starting the server
func adbAvailable() bool {
	cmd := exec.Command("adb", "start-server")
	log.WithFields(log.Fields{
		"event": "provider",
	}).Debug("Checking if adb is available on host")

	if err := cmd.Run(); err != nil {
		log.WithFields(log.Fields{
			"event": "provider",
		}).Warn("adb is not available or command failed - " + err.Error())
		return false
	}
	return true
}

// Remove all adb forwarded ports(if any) on provider start
func removeAdbForwardedPorts() {
	util.ProviderLogger.LogDebug("provider", "Attempting to remove all `adb` forwarded ports on provider start")

	cmd := exec.Command("adb", "forward", "--remove-all")
	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogWarn("provider", "Could not remove `adb` forwarded ports, there was an error or no devices are connected - "+err.Error())
	}
}

func (device *LocalDevice) isGadsStreamServiceRunning() (bool, error) {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "dumpsys", "activity", "services", "com.shamanec.stream/.ScreenCaptureService")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Checking if GADS-stream is already running on Android device - %v", device.Device.UDID))

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
func (device *LocalDevice) installGadsStream() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "install", "-r", "./apps/gads-stream.apk")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Installing GADS-stream apk on Android device - %v", device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not install GADS-stream on Android device - %v:\n %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

// Add recording permissions to gads-stream app to avoid popup on start
func (device *LocalDevice) addGadsStreamRecordingPermissions() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "appops", "set", "com.shamanec.stream", "PROJECT_MEDIA", "allow")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Adding GADS-stream recording permissions on Android device - %v", device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not set GADS-stream recording permissions on Android device - %v:\n %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

// Start the gads-stream app using adb
func (device *LocalDevice) startGadsStreamApp() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "am", "start", "-n", "com.shamanec.stream/com.shamanec.stream.ScreenCaptureActivity")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Starting GADS-stream app on Android device - %v - with command - `%v`", device.Device.UDID, cmd.Path))

	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not start GADS-stream app on Android device - %v:\n %v", device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

// Press the Home button using adb to hide the transparent gads-stream activity
func (device *LocalDevice) pressHomeButton() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "shell", "input", "keyevent", "KEYCODE_HOME")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Pressing Home button with adb on Android device - %v", device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not 'press' Home button with `adb` on Android device - %v, you need to press it yourself to hide the transparent activity og GADS-stream:\n %v", device.Device.UDID, err))
	}
}

// Forward gads-stream socket to the host container
func (device *LocalDevice) forwardGadsStream() {
	cmd := exec.CommandContext(device.Context, "adb", "-s", device.Device.UDID, "forward", "tcp:"+device.Device.StreamPort, "tcp:1991")
	util.ProviderLogger.LogDebug("android_device_setup", fmt.Sprintf("Forwarding GADS-stream port to host port %v for Android device - %v", device.Device.StreamPort, device.Device.UDID))

	err := cmd.Run()
	if err != nil {
		util.ProviderLogger.LogError("android_device_setup", fmt.Sprintf("Could not forward GADS-stream port to host port %v for Android device - %v:\n %v", device.Device.StreamPort, device.Device.UDID, err))
		device.resetLocalDevice()
	}
}

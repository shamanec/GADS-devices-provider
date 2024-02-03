package util

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/shamanec/GADS-devices-provider/config"
	"github.com/shamanec/GADS-devices-provider/logger"
)

var ProjectDir string
var mu sync.Mutex
var usedPorts = make(map[int]bool)
var gadsStreamURL = "https://github.com/shamanec/GADS-Android-stream/releases/latest/download/gads-stream.apk"

// Convert an interface{}(struct) into an indented JSON string
func ConvertToJSONString(data interface{}) (string, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Unmarshal provided JSON string into a struct
func UnmarshalJSONString(jsonString string, v interface{}) error {
	bs := []byte(jsonString)

	err := json.Unmarshal(bs, v)
	if err != nil {
		return err
	}

	return nil
}

func GetFreePort() (port int, err error) {
	mu.Lock()
	defer mu.Unlock()

	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			port = l.Addr().(*net.TCPAddr).Port
			if _, ok := usedPorts[port]; ok {
				return GetFreePort()
			}
			usedPorts[port] = true
			return port, nil
		}
	}
	return
}

// Check if adb is available on the host by starting the server
func AdbAvailable() bool {
	logger.ProviderLogger.LogInfo("provider", "Checking if adb is available on host")

	cmd := exec.Command("adb", "start-server")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("adbAvailable: Error executing `adb start-server`, `adb` is not available on host or command failed - %s", err))
		return false
	}

	return true
}

// Check if xcodebuild is available on the host by checking its version
func XcodebuildAvailable() bool {
	logger.ProviderLogger.LogDebug("provider", "Checking if xcodebuild is available on host")

	cmd := exec.Command("xcodebuild", "-version")
	if err := cmd.Run(); err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("xcodebuildAvailable: xcodebuild is not available or command failed - %s", err))
		return false
	}
	return true
}

// Check if go-ios binary is available
func GoIOSAvailable() bool {
	logger.ProviderLogger.LogDebug("provider", "Checking if go-ios binary is available on host")

	cmd := exec.Command("ios", "-h")
	if err := cmd.Run(); err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("goIOSAvailable: go-ios is not available on host or command failed - %s", err))
		return false
	}
	return true
}

// Build WebDriverAgent for testing with `xcodebuild`
func BuildWebDriverAgent() error {
	cmd := exec.Command("xcodebuild", "-project", "WebDriverAgent.xcodeproj", "-scheme", "WebDriverAgentRunner", "-destination", "generic/platform=iOS", "build-for-testing", "-derivedDataPath", "./build")
	cmd.Dir = config.Config.EnvConfig.WdaRepoPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	logger.ProviderLogger.LogInfo("provider", fmt.Sprintf("Starting WebDriverAgent xcodebuild in path `%s` with command `%s` ", config.Config.EnvConfig.WdaRepoPath, cmd.String()))
	if err := cmd.Start(); err != nil {
		return err
	}

	// Create a scanner to read the command's output line by line
	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()
		logger.ProviderLogger.LogDebug("webdriveragent_xcodebuild", line)
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		logger.ProviderLogger.LogError("provider", fmt.Sprintf("buildWebDriverAgent: Error waiting for build WebDriverAgent with `xcodebuild` command to finish - %s", err))
		logger.ProviderLogger.LogError("provider", "buildWebDriverAgent: Building WebDriverAgent for testing was unsuccessful")
		os.Exit(1)
	}
	return nil
}

// Remove all adb forwarded ports(if any) on provider start
func RemoveAdbForwardedPorts() {
	logger.ProviderLogger.LogInfo("provider", "Attempting to remove all `adb` forwarded ports on provider start")

	cmd := exec.Command("adb", "forward", "--remove-all")
	err := cmd.Run()
	if err != nil {
		logger.ProviderLogger.LogDebug("provider", fmt.Sprintf("removeAdbForwardedPorts: Could not remove `adb` forwarded ports, there was an error or no devices are connected - %s", err))
	}
}

func CheckGadsStreamAndDownload() error {
	if isGadsStreamApkAvailable() {
		logger.ProviderLogger.LogInfo("provider", "GADS-stream apk is available in the provider `conf` folder, it will not be downloaded. If you want to get the latest release, delete the file from conf folder and re-run the provider")
		return nil
	}

	err := downloadGadsStreamApk()
	if err != nil {
		return err
	}

	if !isGadsStreamApkAvailable() {
		return fmt.Errorf("GADS-stream download was reported successful but the .apk was not actually downloaded")
	}

	logger.ProviderLogger.LogInfo("provider", "Latest GADS-stream release apk was successfully downloaded")
	return nil
}

func isGadsStreamApkAvailable() bool {
	_, err := os.Stat(fmt.Sprintf("%s/conf/gads-stream.apk", config.Config.EnvConfig.ProviderFolder))
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

func downloadGadsStreamApk() error {
	logger.ProviderLogger.LogInfo("provider", "Downloading latest GADS-stream release apk file")
	outFile, err := os.Create(fmt.Sprintf("%s/conf/gads-stream.apk", config.Config.EnvConfig.ProviderFolder))
	if err != nil {
		return fmt.Errorf("Could not create file at %s/conf/gads-stream.apk - %s", config.Config.EnvConfig.ProviderFolder, err)
	}
	defer outFile.Close()

	req, err := http.NewRequest(http.MethodGet, gadsStreamURL, nil)
	if err != nil {
		return fmt.Errorf("Could not create new request - %s", err)
	}

	var netClient = &http.Client{
		Timeout: time.Second * 240,
	}
	resp, err := netClient.Do(req)
	if err != nil {
		return fmt.Errorf("Could not execute request to download - %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP response error: %s", resp.Status)
	}

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("Could not copy the response data to the file at apps/gads-stream.apk - %s", err)
	}

	return nil
}

func GetAllAppFiles() []string {
	file, err := os.Open(fmt.Sprintf("%s/apps", config.Config.EnvConfig.ProviderFolder))
	if err != nil {
		logger.ProviderLogger.LogError("provider", fmt.Sprintf("Could not os.Open() apps directory - %s", err))
		return []string{}
	}
	defer file.Close()

	fileList, err := file.Readdir(-1)
	if err != nil {
		logger.ProviderLogger.LogError("provider", fmt.Sprintf("Could not Readdir on the apps directory - %s", err))
		return []string{}
	}

	var files []string
	for _, file := range fileList {
		files = append(files, file.Name())
		fmt.Println(file.Size())
	}

	return files
}

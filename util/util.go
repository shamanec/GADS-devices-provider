package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

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

func CheckGadsStreamAndDownload() error {
	if isGadsStreamApkAvailable() {
		logger.ProviderLogger.LogInfo("provider", "GADS-stream apk is available in the provider ./apps folder, it will not be downloaded. If you want to get the latest release, delete the file from ./apps folder and re-run the provider")
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
	_, err := os.Stat("./apps/gads-stream.apk")
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

func downloadGadsStreamApk() error {
	logger.ProviderLogger.LogInfo("provider", "Downloading latest GADS-stream release apk file")
	outFile, err := os.Create("./apps/gads-stream.apk")
	if err != nil {
		return fmt.Errorf("Could not create file at ./apps/gads-stream.apk - %s", err)
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
		return fmt.Errorf("Could not copy the response data to the file at ./apps/gads-stream.apk - %s", err)
	}

	return nil
}

func GetAllAppFiles() []string {
	file, err := os.Open("./apps")
	if err != nil {
		logger.ProviderLogger.LogError("provider", fmt.Sprintf("Could not os.Open() ./apps directory - %s", err))
		return []string{}
	}
	defer file.Close()

	fileList, err := file.Readdir(-1)
	if err != nil {
		logger.ProviderLogger.LogError("provider", fmt.Sprintf("Could not Readdir on the ./apps directory - %s", err))
		return []string{}
	}

	var files []string
	for _, file := range fileList {
		files = append(files, file.Name())
		fmt.Println(file.Size())
	}

	return files
}

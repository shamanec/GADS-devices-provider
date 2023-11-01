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

	"github.com/shamanec/GADS-devices-provider/models"
	log "github.com/sirupsen/logrus"
)

var ProjectDir string
var Config models.ConfigJsonData
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

func SetupConfig() {
	var err error
	ProjectDir, err = os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Could not get project dir with os.Getwd() - %s", err))
	}

	err = getConfigJsonData()
	if err != nil {
		panic(fmt.Sprintf("Could not get config data from config.json - %s", err))
	}
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

// Read the config.json file and initialize the configuration struct
func getConfigJsonData() error {
	bs, err := getConfigJsonBytes()
	if err != nil {
		return err
	}

	err = json.Unmarshal(bs, &Config)
	if err != nil {
		return err
	}

	return nil
}

// Read the config.json file into a byte slice
func getConfigJsonBytes() ([]byte, error) {
	jsonFile, err := os.Open("./config/config.json")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not open config file: " + err.Error())
		return nil, err
	}
	defer jsonFile.Close()

	bs, err := io.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not read config file to byte slice: " + err.Error())
		return nil, err
	}

	return bs, err
}

func CheckGadsStreamAndDownload() error {
	if isGadsStreamApkAvailable() {
		ProviderLogger.LogInfo("provider", "GADS-stream apk is available in the provider ./apps folder, it will not be downloaded. If you want to get the latest release, delete the file from ./apps folder and re-run the provider")
		return nil
	}

	err := downloadGadsStreamApk()
	if err != nil {
		return err
	}

	if !isGadsStreamApkAvailable() {
		return fmt.Errorf("GADS-stream download was reported successful but the .apk was not actually downloaded")
	}

	ProviderLogger.LogInfo("provider", "Latest GADS-stream release apk was successfully downloaded")
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
	ProviderLogger.LogInfo("provider", "Downloading latest GADS-stream release apk file")
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

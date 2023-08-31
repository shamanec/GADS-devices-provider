package device

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

type ConfigJsonData struct {
	AppiumConfig AppiumConfig `json:"appium-config"`
	EnvConfig    EnvConfig    `json:"env-config"`
	Devices      []*Device    `json:"devices-config"`
}

type AppiumConfig struct {
	SeleniumHubHost         string `json:"selenium_hub_host"`
	SeleniumHubPort         string `json:"selenium_hub_port"`
	SeleniumHubProtocolType string `json:"selenium_hub_protocol_type"`
	UseAppium               bool   `json:"useAppium"`
}

type EnvConfig struct {
	DevicesHost         string `json:"devices_host"`
	ConnectSeleniumGrid string `json:"connect_selenium_grid"`
	SupervisionPassword string `json:"supervision_password"`
	WDABundleID         string `json:"wda_bundle_id"`
	RethinkDB           string `json:"rethink_db"`
}

type cancelFunc func()

type Device struct {
	Container            *DeviceContainer `json:"container,omitempty"`
	Connected            bool             `json:"connected,omitempty"`
	Healthy              bool             `json:"healthy,omitempty"`
	LastHealthyTimestamp int64            `json:"last_healthy_timestamp,omitempty"`
	UDID                 string           `json:"udid"`
	OS                   string           `json:"os"`
	AppiumPort           string           `json:"appium_port"`
	StreamPort           string           `json:"stream_port"`
	ContainerServerPort  string           `json:"container_server_port"`
	WDAPort              string           `json:"wda_port,omitempty"`
	Name                 string           `json:"name"`
	OSVersion            string           `json:"os_version"`
	ScreenSize           string           `json:"screen_size"`
	Model                string           `json:"model"`
	Image                string           `json:"image,omitempty"`
	Host                 string           `json:"host"`
	AppiumSessionID      string           `json:"appiumSessionID,omitempty"`
	WDASessionID         string           `json:"wdaSessionID,omitempty"`
	ProviderState        string           `json:"provider_state,omitempty"`
	Mu                   sync.Mutex       `json:"-"`
	Ctx                  context.Context  `json:"-"`
	GoIOSDevice          ios.DeviceEntry  `json:"-"`
}

type DeviceContainer struct {
	ContainerID     string `json:"id"`
	ContainerStatus string `json:"status"`
	ImageName       string `json:"image_name"`
	ContainerName   string `json:"container_name"`
}

var projectDir string
var Config ConfigJsonData

// Set up the configuration for the provider
// Get the data from config.json, start a DB connection and update the devices
func SetupConfig() error {
	var err error

	// Get the current project folder
	projectDir, err = os.Getwd()
	if err != nil {
		return err
	}

	// Read the config.json file into Config
	err = getConfigJsonData()
	if err != nil {
		return err
	}

	// Create a connection to the DB
	newDBConn()

	// Initialize the devices from config.json and update them in the DB
	err = updateDevicesFromConfig()
	if err != nil {
		return err
	}

	return nil
}

// Loop through the devices from config.json and initialize them
func updateDevicesFromConfig() error {
	// Get the currently connected devices from /dev
	connectedDevices, err := getConnectedDevices()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_listener",
		}).Error("Could not get the devices from /dev: " + err.Error())
		return err
	}

	// Loop through the devices from the config
	for index, device := range Config.Devices {
		// Update each device Connected field
		device.Connected = false
		for _, connectedDevice := range connectedDevices {
			if strings.Contains(connectedDevice, device.UDID) {
				device.Connected = true
			}
		}

		// Update the other fields
		wdaPort := ""
		if device.OS == "ios" {
			wdaPort = strconv.Itoa(20001 + index)
		}
		device.Container = nil
		device.AppiumPort = strconv.Itoa(4841 + index)
		device.StreamPort = strconv.Itoa(20101 + index)
		device.ContainerServerPort = strconv.Itoa(20201 + index)
		device.WDAPort = wdaPort
		device.Host = Config.EnvConfig.DevicesHost
		device.ProviderState = "init"
	}

	// Insert the devices to the DB if they are not already inserted
	// or update them if they are
	err = insertDevicesDB()
	if err != nil {
		return err
	}

	return nil
}

// Read the config.json file and initialize the configuration struct
func getConfigJsonData() error {
	bs, err := getConfigJsonBytes()
	if err != nil {
		return err
	}

	err = json.Unmarshal(bs, &Config)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not unmarshal config file: " + err.Error())
		return err
	}

	return nil
}

// Read the config.json file into a byte slice
func getConfigJsonBytes() ([]byte, error) {
	jsonFile, err := os.Open("./configs/config.json")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not open config file: " + err.Error())
		return nil, err
	}
	defer jsonFile.Close()

	bs, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not read config file to byte slice: " + err.Error())
		return nil, err
	}

	return bs, err
}

package device

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/danielpaulus/go-ios/ios"
	log "github.com/sirupsen/logrus"
)

type ConfigJsonData struct {
	AppiumConfig AppiumConfig `json:"appium-config" bson:"appium-config"`
	EnvConfig    EnvConfig    `json:"env-config" bson:"env-config"`
	Devices      []*Device    `json:"devices-config" bson:"devices-config"`
}

type AppiumConfig struct {
	SeleniumHubHost         string `json:"selenium_hub_host" bson:"selenium_hub_host"`
	SeleniumHubPort         string `json:"selenium_hub_port" bson:"selenium_hub_port"`
	SeleniumHubProtocolType string `json:"selenium_hub_protocol_type" bson:"selenium_hub_protocol_type"`
}

type EnvConfig struct {
	DevicesHost         string `json:"devices_host" bson:"devices_host"`
	ConnectSeleniumGrid string `json:"connect_selenium_grid" bson:"connect_selenium_grid"`
	SupervisionPassword string `json:"supervision_password" bson:"supervision_password"`
	WDABundleID         string `json:"wda_bundle_id" bson:"wda_bundle_id"`
	RethinkDB           string `json:"rethink_db" bson:"rethink_db"`
	WDAPath             string `json:"wda_repo_path" bson:"wda_repo_path"`
}

type Device struct {
	Container            *DeviceContainer `json:"container,omitempty" bson:"container,omitempty"`
	Connected            bool             `json:"connected,omitempty" bson:"connected,omitempty"`
	Healthy              bool             `json:"healthy,omitempty" bson:"healthy,omitempty"`
	LastHealthyTimestamp int64            `json:"last_healthy_timestamp,omitempty" bson:"last_healthy_timestamp,omitempty"`
	UDID                 string           `json:"udid" bson:"_id"`
	OS                   string           `json:"os" bson:"os"`
	AppiumPort           string           `json:"appium_port" bson:"appium_port"`
	StreamPort           string           `json:"stream_port" bson:"stream_port"`
	ContainerServerPort  string           `json:"container_server_port" bson:"container_server_port"`
	WDAPort              string           `json:"wda_port,omitempty" bson:"wda_port,omitempty"`
	Name                 string           `json:"name" bson:"name"`
	OSVersion            string           `json:"os_version" bson:"os_version"`
	ScreenSize           string           `json:"screen_size" bson:"screen_size"`
	Model                string           `json:"model" bson:"model"`
	Image                string           `json:"image,omitempty" bson:"image,omitempty"`
	Host                 string           `json:"host" bson:"host"`
	AppiumSessionID      string           `json:"appiumSessionID,omitempty" bson:"appiumSessionID,omitempty"`
	WDASessionID         string           `json:"wdaSessionID,omitempty" bson:"wdaSessionID,omitempty"`
}

type LocalDevice struct {
	Device           *Device
	ProviderState    string
	WdaReadyChan     chan bool
	Context          context.Context
	CtxCancel        context.CancelFunc
	GoIOSDeviceEntry ios.DeviceEntry
}

type DeviceContainer struct {
	ContainerID     string `json:"id"`
	ContainerStatus string `json:"status"`
	ImageName       string `json:"image_name"`
	ContainerName   string `json:"container_name"`
}

var projectDir string
var Config ConfigJsonData
var DeviceMap = make(map[string]*Device)

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

	createMongoLogCollectionsForAllDevices()

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

		DeviceMap[device.UDID] = device
	}

	// Start periodic update of device data in the DB
	go updateDevicesMongo()

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

	bs, err := io.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not read config file to byte slice: " + err.Error())
		return nil, err
	}

	return bs, err
}

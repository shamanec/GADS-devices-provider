package device

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

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
}

type EnvConfig struct {
	DevicesHost         string `json:"devices_host"`
	ConnectSeleniumGrid string `json:"connect_selenium_grid"`
	SupervisionPassword string `json:"supervision_password"`
	RemoteControl       string `json:"remote_control"`
	WDABundleID         string `json:"wda_bundle_id"`
	RethinkDB           string `json:"rethink_db"`
}

type Device struct {
	Container             *DeviceContainer `json:"container,omitempty"`
	Connected             bool             `json:"connected,omitempty"`
	Healthy               bool             `json:"healthy,omitempty"`
	LastHealthyTimestamp  int64            `json:"last_healthy_timestamp,omitempty"`
	UDID                  string           `json:"udid"`
	OS                    string           `json:"os"`
	AppiumPort            string           `json:"appium_port"`
	StreamPort            string           `json:"stream_port"`
	ContainerServerPort   string           `json:"container_server_port"`
	WDAPort               string           `json:"wda_port,omitempty"`
	Name                  string           `json:"name"`
	OSVersion             string           `json:"os_version"`
	ScreenSize            string           `json:"screen_size"`
	Model                 string           `json:"model"`
	Image                 string           `json:"image,omitempty"`
	Host                  string           `json:"host"`
	MinicapFPS            string           `json:"minicap_fps,omitempty"`
	MinicapHalfResolution string           `json:"minicap_half_resolution,omitempty"`
	UseMinicap            string           `json:"use_minicap,omitempty"`
}

type DeviceContainer struct {
	ContainerID     string `json:"id"`
	ContainerStatus string `json:"status"`
	ImageName       string `json:"image_name"`
	ContainerName   string `json:"container_name"`
}

var projectDir string
var Config ConfigJsonData

// Set up the configuration data for the provider
func SetupConfig() error {
	var err error

	projectDir, err = os.Getwd()
	if err != nil {
		return err
	}

	err = getConfigJsonData()
	if err != nil {
		return err
	}

	newDBConn()

	err = updateDevicesFromConfig()
	if err != nil {
		return err
	}

	return nil
}

// Loop through the devices from config.json and initialize the empty values
func updateDevicesFromConfig() error {
	// Get the currently connected devices from /dev
	connectedDevices, err := getConnectedDevices()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_listener",
		}).Error("Could not get the devices from /dev, err: " + err.Error())
		return err
	}

	// Loop through the devices from the config and the currently connected devices and update the Connected field
	for index, device := range Config.Devices {
		device.Connected = false
		for _, connectedDevice := range connectedDevices {
			if strings.Contains(connectedDevice, device.UDID) {
				device.Connected = true
			}
		}

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

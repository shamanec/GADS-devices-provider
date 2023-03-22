package config

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"
)

type ConfigJsonData struct {
	AppiumConfig AppiumConfig `json:"appium-config"`
	EnvConfig    EnvConfig    `json:"env-config"`
}

type AppiumConfig struct {
	DevicesHost             string `json:"devices_host"`
	SeleniumHubHost         string `json:"selenium_hub_host"`
	SeleniumHubPort         string `json:"selenium_hub_port"`
	SeleniumHubProtocolType string `json:"selenium_hub_protocol_type"`
	WDABundleID             string `json:"wda_bundle_id"`
}

type EnvConfig struct {
	ConnectSeleniumGrid  string `json:"connect_selenium_grid"`
	SupervisionPassword  string `json:"supervision_password"`
	ContainerizedUsbmuxd string `json:"containerized_usbmuxd"`
	RemoteControl        string `json:"remote_control"`
}

var ProviderPort string
var HomeDir string
var ProjectDir string
var err error
var ConfigData ConfigJsonData

func SetupConfig() {
	HomeDir, err = os.UserHomeDir()
	if err != nil {
		panic("Could not get home dir: " + err.Error())
	}

	ProjectDir, err = os.Getwd()
	if err != nil {
		panic("Could not get project dir: " + err.Error())
	}

	port_flag := flag.String("port", "10001", "The port to run the server on")
	flag.Parse()

	ConfigData, err = getConfigJsonData()
	if err != nil {
		panic("Could not get config data from config.json: " + err.Error())
	}

	ProviderPort = *port_flag
}

func getConfigJsonData() (ConfigJsonData, error) {
	var data ConfigJsonData
	jsonFile, err := os.Open("./configs/config.json")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not open config file: " + err.Error())
		return data, err
	}
	defer jsonFile.Close()

	bs, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not read config file to byte slice: " + err.Error())
		return data, err
	}

	err = json.Unmarshal(bs, &data)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not unmarshal config file: " + err.Error())
		return data, err
	}

	return data, nil
}

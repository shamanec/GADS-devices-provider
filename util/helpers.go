package util

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"
)

type ConfigJsonData struct {
	AppiumConfig AppiumConfig   `json:"appium-config"`
	EnvConfig    EnvConfig      `json:"env-config"`
	DeviceConfig []DeviceConfig `json:"devices-config"`
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

type DeviceConfig struct {
	OS                    string `json:"os"`
	AppiumPort            string `json:"appium_port"`
	DeviceName            string `json:"device_name"`
	DeviceOSVersion       string `json:"device_os_version"`
	DeviceUDID            string `json:"device_udid"`
	StreamPort            string `json:"stream_port"`
	WDAPort               string `json:"wda_port"`
	ScreenSize            string `json:"screen_size"`
	ContainerServerPort   string `json:"container_server_port"`
	DeviceModel           string `json:"device_model"`
	DeviceImage           string `json:"device_image"`
	DeviceHost            string `json:"device_host"`
	MinicapFPS            string `json:"minicap_fps"`
	MinicapHalfResolution string `json:"minicap_half_resolution"`
}

//=======================//
//=======FUNCTIONS=======//

// Get a ConfigJsonData pointer with the current configuration from config.json
func GetConfigJsonData() (*ConfigJsonData, error) {
	var data ConfigJsonData
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

	err = json.Unmarshal(bs, &data)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not unmarshal config file: " + err.Error())
		return nil, err
	}

	return &data, nil
}

// Convert interface into JSON string
func ConvertToJSONString(data interface{}) (string, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "convert_interface_to_json",
		}).Error("Could not marshal interface to json: " + err.Error())
		return "", err
	}
	return string(b), nil
}

// Unmarshal request body into a struct
func UnmarshalReader(body io.ReadCloser, v interface{}) error {
	reqBody, err := ioutil.ReadAll(body)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "unmarshal_reader",
		}).Error("Could not read reader into byte slice: " + err.Error())
		return err
	}

	err = json.Unmarshal(reqBody, v)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "unmarshal_reader",
		}).Error("Could not unmarshal reader: " + err.Error())
		return err
	}

	return nil
}

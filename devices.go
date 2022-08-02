package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	log "github.com/sirupsen/logrus"
)

type AvailableDevicesInfo struct {
	DevicesInfo []DeviceInfo `json:"devices-info"`
}

type DeviceInfo struct {
	DeviceModel               string `json:"device_model"`
	DeviceOSVersion           string `json:"device_os_version"`
	DeviceOS                  string `json:"device_os"`
	DeviceContainerServerPort string `json:"container_server_port"`
	DeviceUDID                string `json:"device_udid"`
	DeviceImage               string `json:"device_image"`
	DeviceHost                string `json:"device_host"`
	WdaPort                   string `json:"wda_port"`
	StreamPort                string `json:"stream_port"`
	ScreenSize                string `json:"screen_size"`
	AppiumPort                string `json:"appium_port"`
}

//=======================//
//=====API FUNCTIONS=====//

func GetAvailableDevicesInfo(w http.ResponseWriter, r *http.Request) {
	runningContainerNames, err := getRunningDeviceContainerNames()
	if err != nil {
		JSONError(w, "get_available_devices", "Could not get available devices", 500)
		return
	}

	devicesInfo, err := getAvailableDevicesInfo(runningContainerNames)
	if err != nil {
		JSONError(w, "get_available_devices", "Could not get available devices", 500)
		return
	}

	var info = AvailableDevicesInfo{
		DevicesInfo: devicesInfo,
	}

	responseData, err := ConvertToJSONString(info)
	if err != nil {
		JSONError(w, "get_available_devices", "Could not get available devices", 500)
		return
	}
	fmt.Fprintf(w, responseData)
}

//=======================//
//=======FUNCTIONS=======//

func getAvailableDevicesInfo(runningContainers []string) ([]DeviceInfo, error) {
	var combinedInfo []DeviceInfo

	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not get config data when getting devices info")
		return nil, err
	}

	for _, containerName := range runningContainers {
		// Extract the device UDID from the container name
		re := regexp.MustCompile("[^_]*$")
		device_udid := re.FindStringSubmatch(containerName)

		// Get the info for the respective device from config.json
		var device_config *DeviceInfo
		device_config, err := getDeviceInfo(device_udid[0], configData)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "ios_container_create",
			}).Error("Could not get info for device " + device_udid[0] + " from config data")
			return nil, err
		}

		// Append the respective device info to the combined info
		combinedInfo = append(combinedInfo, *device_config)
	}

	return combinedInfo, nil
}

// Get all running containers on host and filter them out for iOS and Android containers
func getRunningDeviceContainerNames() ([]string, error) {
	var containerNames []string

	// Create a new docker client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_running_container_names",
		}).Error("Could not create new docker client: " + err.Error())
		return nil, err
	}

	defer cli.Close()

	// Get the current containers list
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_running_container_names",
		}).Error("Could not get docker containers list: " + err.Error())
		return nil, err
	}

	// Loop through the containers list
	for _, container := range containers {
		// Parse plain container name
		containerName := strings.Replace(container.Names[0], "/", "", -1)

		// Check if container is for ios or android device and its status is 'Up'
		if (strings.Contains(containerName, "iosDevice") || strings.Contains(containerName, "androidDevice")) && strings.Contains(container.Status, "Up") {
			containerNames = append(containerNames, containerName)
		}
	}
	return containerNames, nil
}

func getDeviceInfo(device_udid string, configData *ConfigJsonData) (*DeviceInfo, error) {
	// Loop through the device configs and find the one that corresponds to the provided device UDID
	var deviceConfig DeviceConfig
	for _, v := range configData.DeviceConfig {
		if v.DeviceUDID == device_udid {
			deviceConfig = v
		}
	}

	if deviceConfig == (DeviceConfig{}) {
		log.WithFields(log.Fields{
			"event": "get_device_info_from_config",
		}).Error("Device with udid " + device_udid + " was not found in config data.")
		return nil, errors.New("")
	}

	// Return the info for the device
	return &DeviceInfo{
		DeviceModel:               deviceConfig.DeviceModel,
		DeviceOSVersion:           deviceConfig.DeviceOSVersion,
		DeviceOS:                  deviceConfig.OS,
		DeviceContainerServerPort: deviceConfig.ContainerServerPort,
		DeviceUDID:                deviceConfig.DeviceUDID,
		DeviceImage:               deviceConfig.DeviceImage,
		DeviceHost:                configData.AppiumConfig.DevicesHost,
		WdaPort:                   deviceConfig.WDAPort,
		StreamPort:                deviceConfig.StreamPort,
		AppiumPort:                deviceConfig.AppiumPort,
		ScreenSize:                deviceConfig.ScreenSize,
	}, nil
}

package main

import (
	"context"
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
	DeviceContainerServerPort int    `json:"container_server_port"`
	DeviceUDID                string `json:"device_udid"`
	DeviceImage               string `json:"device_image"`
	DeviceHost                string `json:"device_host"`
}

func getAvailableDevicesInfo(runningContainers []string) []DeviceInfo {
	var combinedInfo []DeviceInfo

	for _, containerName := range runningContainers {
		// Extract the device UDID from the container name
		re := regexp.MustCompile("[^_]*$")
		device_udid := re.FindStringSubmatch(containerName)

		var device_config *DeviceInfo
		device_config = getDeviceInfo(device_udid[0])

		combinedInfo = append(combinedInfo, *device_config)
	}

	return combinedInfo
}

func getRunningDeviceContainerNames() []string {
	var containerNames []string

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return containerNames
	}

	// Get the current containers list
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return containerNames
	}

	// Loop through the containers list
	for _, container := range containers {
		// Parse plain container name
		containerName := strings.Replace(container.Names[0], "/", "", -1)
		if (strings.Contains(containerName, "iosDevice") || strings.Contains(containerName, "androidDevice")) && strings.Contains(container.Status, "Up") {
			containerNames = append(containerNames, containerName)
		}
	}
	return containerNames
}

func GetAvailableDevicesInfo(w http.ResponseWriter, r *http.Request) {
	var runningContainerNames = getRunningDeviceContainerNames()
	var info = AvailableDevicesInfo{
		DevicesInfo: getAvailableDevicesInfo(runningContainerNames),
	}
	fmt.Fprintf(w, PrettifyJSON(ConvertToJSONString(info)))
}

func getDeviceInfo(device_udid string) *DeviceInfo {
	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not unmarshal config.json file when trying to create a container for device with udid: " + device_udid)
		return nil
	}

	var deviceConfig DeviceConfig
	for _, v := range configData.DeviceConfig {
		if v.DeviceUDID == device_udid {
			deviceConfig = v
		}
	}

	return &DeviceInfo{
		DeviceModel:               deviceConfig.DeviceModel,
		DeviceOSVersion:           deviceConfig.DeviceOSVersion,
		DeviceOS:                  deviceConfig.OS,
		DeviceContainerServerPort: deviceConfig.ContainerServerPort,
		DeviceUDID:                deviceConfig.DeviceUDID,
		DeviceImage:               deviceConfig.DeviceImage,
		DeviceHost:                configData.AppiumConfig.DevicesHost,
	}
}

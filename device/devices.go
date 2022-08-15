package device

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/shamanec/GADS-devices-provider/provider"
	"github.com/shamanec/GADS-devices-provider/util"

	log "github.com/sirupsen/logrus"
)

type DevicesInfo struct {
	DevicesInfo []util.DeviceConfig `json:"devices-info"`
}

//=======================//
//=======FUNCTIONS=======//

func AvailableDevicesInfo(runningContainers []string) ([]util.DeviceConfig, error) {
	var combinedInfo []util.DeviceConfig

	for _, containerName := range runningContainers {
		// Extract the device UDID from the container name
		re := regexp.MustCompile("[^_]*$")
		deviceUDID := re.FindStringSubmatch(containerName)

		// Get the info for the respective device from config.json
		var deviceInformation *util.DeviceConfig
		deviceInformation, err := DeviceInfo(deviceUDID[0], provider.ConfigData)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "get_available_devices_info",
			}).Error("Could not get info for device " + deviceUDID[0] + " from config data")
			return nil, err
		}

		// Append the respective device info to the combined info
		combinedInfo = append(combinedInfo, *deviceInformation)
	}

	return combinedInfo, nil
}

// Get all running containers on host and filter them out for iOS and Android containers
func RunningDeviceContainerNames() ([]string, error) {
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

func DeviceInfo(device_udid string, configData util.ConfigJsonData) (*util.DeviceConfig, error) {
	// Loop through the device configs and find the one that corresponds to the provided device UDID
	var deviceConfig util.DeviceConfig
	for _, v := range configData.DeviceConfig {
		if v.DeviceUDID == device_udid {
			deviceConfig = v
		}
	}

	if deviceConfig == (util.DeviceConfig{}) {
		log.WithFields(log.Fields{
			"event": "get_device_info_from_config",
		}).Error("Device with udid " + device_udid + " was not found in config data.")
		return nil, errors.New("")
	}

	// Return the info for the device
	return &util.DeviceConfig{
		OS:                    deviceConfig.OS,
		AppiumPort:            deviceConfig.AppiumPort,
		DeviceName:            deviceConfig.DeviceName,
		DeviceOSVersion:       deviceConfig.DeviceOSVersion,
		DeviceUDID:            deviceConfig.DeviceUDID,
		StreamPort:            deviceConfig.StreamPort,
		WDAPort:               deviceConfig.WDAPort,
		ScreenSize:            deviceConfig.ScreenSize,
		ContainerServerPort:   deviceConfig.ContainerServerPort,
		DeviceModel:           deviceConfig.DeviceModel,
		DeviceImage:           deviceConfig.DeviceImage,
		DeviceHost:            configData.AppiumConfig.DevicesHost,
		MinicapFPS:            deviceConfig.MinicapFPS,
		MinicapHalfResolution: deviceConfig.MinicapHalfResolution,
	}, nil
}

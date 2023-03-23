package device

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/shamanec/GADS-devices-provider/config"
	log "github.com/sirupsen/logrus"
)

func getDeviceJsonData() ([]*Device, error) {
	var devices []*Device
	bs, err := getDeviceJsonBytes()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bs, &devices)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not unmarshal config file: " + err.Error())
		return nil, err
	}

	return devices, err
}

func getDeviceJsonBytes() ([]byte, error) {
	jsonFile, err := os.Open("./configs/devices.json")
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

	return bs, nil
}

// Create initial devices from the json config
func updateDevicesFromConfig() ([]*Device, error) {
	devices, err := getDeviceJsonData()
	if err != nil {
		return nil, err
	}

	for index, configDevice := range devices {
		wdaPort := ""
		if configDevice.OS == "ios" {
			wdaPort = strconv.Itoa(20001 + index)
		}

		configDevice.Container = nil
		configDevice.State = "Disconnected"
		configDevice.AppiumPort = strconv.Itoa(4841 + index)
		configDevice.StreamPort = strconv.Itoa(20101 + index)
		configDevice.ContainerServerPort = strconv.Itoa(20201 + index)
		configDevice.WDAPort = wdaPort
		configDevice.Host = config.ConfigData.AppiumConfig.DevicesHost
	}

	return devices, nil
}

func getConnectedDevices() ([]string, error) {
	// Get all files/symlinks/folders in /dev
	var connectedDevices []string = []string{}
	devFiles, err := filepath.Glob("/dev/*")
	if err != nil {
		fmt.Println("Error listing files in /dev:", err)
		return nil, err
	}

	for _, devFile := range devFiles {
		// Split the devFile to get only the file name
		_, fileName := filepath.Split(devFile)
		// If the filename is a device symlink
		// Add it to the connected devices list
		if strings.Contains(fileName, "device") {
			connectedDevices = append(connectedDevices, fileName)
		}
	}

	return connectedDevices, nil
}

// Get list of containers on host
func getHostContainers() ([]types.Container, error) {
	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_list",
		}).Error(". Error: " + err.Error())
		return nil, errors.New("Could not create docker client")
	}
	defer cli.Close()

	// Get the list of containers
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_list",
		}).Error(". Error: " + err.Error())
		return nil, errors.New("Could not get container list")
	}
	return containers, nil
}

// Check if device is connected to the host
func (device *Device) isDeviceConnected(connectedDevices []string) (bool, error) {
	for _, connectedDevice := range connectedDevices {
		if strings.Contains(connectedDevice, device.UDID) {
			return true, nil
		}
	}
	return false, nil
}

func (device *Device) hasContainer(allContainers []types.Container) (bool, error) {
	for _, container := range allContainers {
		// Parse plain container name
		containerName := strings.Replace(container.Names[0], "/", "", -1)

		if strings.Contains(containerName, device.UDID) {
			deviceContainer := DeviceContainer{
				ContainerID:     container.ID,
				ContainerStatus: container.Status,
				ImageName:       container.Image,
				ContainerName:   containerName,
			}
			device.Container = &deviceContainer
			return true, nil
		}
	}
	return false, nil
}

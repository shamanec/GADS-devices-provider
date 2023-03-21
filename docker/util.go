package docker

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/shamanec/GADS-devices-provider/provider"
	log "github.com/sirupsen/logrus"
)

type DeviceContainerInfo struct {
	ContainerID     string
	ImageName       string
	ContainerStatus string
	ContainerPorts  string
	ContainerName   string
	DeviceUDID      string
}

// Create initial devices from the json config
func createDevicesFromConfig() []*Device {
	var devices []*Device
	for index, configDevice := range provider.ConfigData.DeviceConfig {
		wdaPort := ""
		if configDevice.OS == "ios" {
			wdaPort = strconv.Itoa(20001 + index)
		}

		device := &Device{
			Container:             nil,
			State:                 "Disconnected",
			UDID:                  configDevice.DeviceUDID,
			OS:                    configDevice.OS,
			AppiumPort:            strconv.Itoa(4841 + index),
			StreamPort:            strconv.Itoa(20101 + index),
			ContainerServerPort:   strconv.Itoa(20201 + index),
			WDAPort:               wdaPort,
			Name:                  configDevice.DeviceName,
			OSVersion:             configDevice.DeviceOSVersion,
			ScreenSize:            configDevice.ScreenSize,
			Model:                 configDevice.DeviceModel,
			Image:                 configDevice.DeviceImage,
			Host:                  provider.ConfigData.AppiumConfig.DevicesHost,
			MinicapFPS:            configDevice.MinicapFPS,
			MinicapHalfResolution: configDevice.MinicapHalfResolution,
			UseMinicap:            configDevice.UseMinicap,
		}
		devices = append(devices, device)
	}

	return devices
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

func GenerateDevicePorts(udid string) (string, string, string, string) {
	for index, deviceConfig := range provider.ConfigData.DeviceConfig {
		configUDID := deviceConfig.DeviceUDID
		if configUDID == udid {
			appiumPort := strconv.Itoa(4841 + index)
			streamPort := strconv.Itoa(20101 + index)
			containerServerPort := strconv.Itoa(20201 + index)
			wdaPort := strconv.Itoa(20001 + index)
			return appiumPort, streamPort, containerServerPort, wdaPort
		}
	}

	return "", "", "", ""
}

// Generate the data for device containers table in the UI
func DeviceContainerRows() ([]DeviceContainerInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	// Get the current containers list
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var rows []DeviceContainerInfo

	// Loop through the containers list
	for _, container := range containers {
		// Parse plain container name
		containerName := strings.Replace(container.Names[0], "/", "", -1)

		// Get all the container ports from the returned array into string
		containerPorts := ""
		for i, s := range container.Ports {
			if i > 0 {
				containerPorts += "\n"
			}
			containerPorts += "{" + s.IP + ", " + strconv.Itoa(int(s.PrivatePort)) + ", " + strconv.Itoa(int(s.PublicPort)) + ", " + s.Type + "}"
		}

		// Extract the device UDID from the container name
		re := regexp.MustCompile("[^_]*$")
		match := re.FindStringSubmatch(containerName)

		// Create a table row data and append it to the slice
		var containerRow = DeviceContainerInfo{ContainerID: container.ID, ImageName: container.Image, ContainerStatus: container.Status, ContainerPorts: containerPorts, ContainerName: containerName, DeviceUDID: match[0]}
		rows = append(rows, containerRow)
	}
	return rows, nil
}

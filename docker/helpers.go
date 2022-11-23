package docker

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

// Check if container exists by name and also return container_id
func CheckContainerExistsByName(deviceUDID string) (bool, string, string) {
	// Get all the containers
	containers, _ := getDeviceContainersList()
	containerExists := false
	containerID := ""
	containerStatus := ""

	// Loop through the available containers
	// If a container which name contains the device udid exists
	// return true and also return the container ID and status
	for _, container := range containers {
		containerName := strings.Replace(container.Names[0], "/", "", -1)
		if strings.Contains(containerName, deviceUDID) {
			containerExists = true
			containerID = container.ID
			containerStatus = container.Status
		}
	}
	return containerExists, containerID, containerStatus
}

// Get list of containers on host
func getContainersList() ([]types.Container, error) {
	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_list",
		}).Error(". Error: " + err.Error())
		return nil, errors.New("Could not create docker client")
	}

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

func getDeviceContainersList() ([]types.Container, error) {
	allContainers, err := getContainersList()
	if err != nil {
		return nil, err
	}

	var deviceContainers []types.Container
	for _, container := range allContainers {
		containerName := strings.Replace(container.Names[0], "/", "", -1)
		if strings.Contains(containerName, "iosDevice") || strings.Contains(containerName, "androidDevice") {
			deviceContainers = append(deviceContainers, container)
		}
	}

	return deviceContainers, nil
}

type DeviceContainerInfo struct {
	ContainerID     string
	ImageName       string
	ContainerStatus string
	ContainerPorts  string
	ContainerName   string
	DeviceUDID      string
}

// Generate the data for device containers table in the UI
func DeviceContainerRows() ([]DeviceContainerInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

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

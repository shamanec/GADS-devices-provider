package docker

import (
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
)

type DeviceContainer struct {
	ContainerID     string `json:"id"`
	ContainerStatus string `json:"status"`
	ImageName       string `json:"image_name"`
	ContainerName   string `json:"container_name"`
}

type Device struct {
	Container             *DeviceContainer `json:"container,omitempty"`
	State                 string           `json:"state"`
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

var mutex sync.Mutex
var configDevices []*Device

func UpdateDevices() {
	configDevices = createDevicesFromConfig()
	if configDevices == nil {
		log.WithFields(log.Fields{
			"event": "device_listener",
		}).Warn("There are no devices registered in config.json. Please add devices and restart the provider!")
	}

OUTER:
	for {
		// Get a list of the connected device symlinks from /dev
		connectedDevices, err := getConnectedDevices()
		if err != nil {
			log.WithFields(log.Fields{
				"event": "device_listener",
			}).Error("Could not get the devices from /dev, err: " + err.Error())
			break OUTER
		}

		// Get the containers running on the host
		allContainers, err := getHostContainers()
		if err != nil {
			log.WithFields(log.Fields{
				"event": "device_listener",
			}).Error("Could not get host containers, err: " + err.Error())
			break OUTER
		}

		// Loop through the devices registered from the config
	INNER:
		for _, configDevice := range configDevices {
			// Check if the current device is connected to the host
			connected, err := configDevice.isDeviceConnected(connectedDevices)
			if err != nil {
				log.WithFields(log.Fields{
					"event": "device_listener",
				}).Error("Could not check if device " + configDevice.UDID + " is connected to the host, err: " + err.Error())
				continue INNER
			}

			if connected {
				// Set the initial state to Connected
				configDevice.State = "Connected"

				// Check if the device has an already created container
				// Also append the container data to the device struct if it does
				hasContainer, err := configDevice.hasContainer(allContainers)
				if err != nil {
					log.WithFields(log.Fields{
						"event": "device_listener",
					}).Error("Could not check if device " + configDevice.UDID + " has a container, err: " + err.Error())
					continue INNER
				}

				// If the device has container
				if hasContainer {
					// If the container is not Up
					if !strings.Contains(configDevice.Container.ContainerStatus, "Up") {
						// Restart the container
						go configDevice.restartContainer()
						continue INNER
					}
					// If the container is Up set the state to Available
					configDevice.State = "Available"
					continue INNER
				}

				if configDevice.OS == "ios" {
					go configDevice.createIOSContainer()
					continue INNER
				}

				if configDevice.OS == "android" {
					go configDevice.createAndroidContainer()
					continue INNER
				}
				continue INNER
			}

			// If the device is not connected
			if !connected {
				// Check if it has an existing container
				hasContainer, err := configDevice.hasContainer(allContainers)
				if err != nil {
					log.WithFields(log.Fields{
						"event": "device_listener",
					}).Error("Could not check if device " + configDevice.UDID + " has a container, err: " + err.Error())
					continue INNER
				}
				// If it has a container - remove it
				if hasContainer {
					configDevice.removeContainer()
				}
			}
		}

		time.Sleep(5 * time.Second)
	}
}

func GetConfigDevices() []*Device {
	return configDevices
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

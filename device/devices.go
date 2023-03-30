package device

import (
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func UpdateDevices() {
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
		for _, configDevice := range Config.Devices {
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
				configDevice.Connected = true
				configDevice.updateDB()

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
					configDevice.updateDB()
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
				configDevice.Connected = false
				configDevice.updateDB()
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
	return Config.Devices
}

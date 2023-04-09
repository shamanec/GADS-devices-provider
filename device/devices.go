package device

import (
	"fmt"
	"strings"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

func UpdateDevices() {
	fmt.Println("Initial device update")
	updateDevicesConnectedStatus()
	updateDevices()

	fmt.Println("Starting devices healthcheck")
	go devicesHealthCheck()

	fmt.Println("Starting /dev watcher")
	go devicesWatcher()
}

// Update the Connected status of the devices both locally and in DB each second
func updateDevicesConnectedStatus() {
	connectedDevices, err := getConnectedDevices()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_listener",
		}).Error("Could not get the devices from /dev: " + err.Error())
		panic("Could not get the devices from /dev: " + err.Error())
	}

	for _, device := range Config.Devices {
		device.Connected = false
		for _, connectedDevice := range connectedDevices {
			if strings.Contains(connectedDevice, device.UDID) {
				device.Connected = true
			}
		}
		device.updateDB()
	}
}

func updateDevices() {
	// Get the containers running on the host
	allContainers, err := getHostContainers()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_update",
		}).Error("Could not get host containers: " + err.Error())
		return
	}

	// Loop through the devices registered from the config
	for _, device := range Config.Devices {

		if device.Connected {
			device.updateDB()

			// Check if the device has an already created container
			// Also append the container data to the device struct if it does
			hasContainer, err := device.hasContainer(allContainers)
			if err != nil {
				log.WithFields(log.Fields{
					"event": "device_update",
				}).Error("Could not check if device " + device.UDID + " has a container: " + err.Error())
				continue
			}

			// If the device has container
			if hasContainer {
				// If the container is not Up
				if !strings.Contains(device.Container.ContainerStatus, "Up") {
					// Restart the container
					go device.restartContainer()
					continue
				}

				continue
			}

			if device.OS == "ios" {
				go device.createIOSContainer()
				continue
			}

			if device.OS == "android" {
				go device.createAndroidContainer()
				continue
			}
			continue
		}

		// If the device is not connected
		if !device.Connected {
			device.updateDB()
			// Check if it has an existing container
			hasContainer, err := device.hasContainer(allContainers)
			if err != nil {
				log.WithFields(log.Fields{
					"event": "device_update",
				}).Error("Could not check if device " + device.UDID + " has a container: " + err.Error())
				continue
			}
			// If it has a container - remove it
			if hasContainer {
				go device.removeContainer()
			}
		}
	}
}

func GetConfigDevices() []*Device {
	return Config.Devices
}

func devicesWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic("Could not create watcher when preparing to watch /dev folder: " + err.Error())
	}
	defer watcher.Close()

	err = watcher.Add("/dev")
	if err != nil {
		panic("Could not add /dev folder to watcher when preparing to watch it: " + err.Error())
	}

	fmt.Println("Started listening for events in /dev folder")
	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// If we have a Create event in /dev (device was connected)
				if event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
					// Get the name of the created file
					fileName := event.Name

					// Check if the created file was a symlink for a device
					if strings.HasPrefix(fileName, "/dev/device_") {
						updateDevicesConnectedStatus()
						updateDevices()
					}
				}

				// If we have a Remove event in /dev (device was disconnected)
				if event.Has(fsnotify.Remove) {

				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.WithFields(log.Fields{
					"event": "device_watcher",
				}).Info("There was an error with the /dev watcher: " + err.Error())
			}
		}
	}()

	// Block the deviceWatcher() goroutine forever
	<-done
}

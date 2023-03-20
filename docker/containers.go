package docker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/shamanec/GADS-devices-provider/provider"
	log "github.com/sirupsen/logrus"
)

var connectedDevices map[string]Device
var mutex sync.Mutex

type DeviceContainer struct {
	ContainerID     string
	ContainerPorts  []types.Port
	ContainerStatus string
	ImageName       string
	ContainerName   string
}

type Device struct {
	Container             DeviceContainer `json:"container"`
	State                 string          `json:"state"`
	UDID                  string          `json:"udid"`
	OS                    string          `json:"os"`
	AppiumPort            string          `json:"appium_port"`
	StreamPort            string          `json:"stream_port"`
	ContainerServerPort   string          `json:"container_server_port"`
	WDAPort               string          `json:"wda_port,omitempty"`
	Name                  string          `json:"name"`
	OSVersion             string          `json:"os_version"`
	ScreenSize            string          `json:"screen_size"`
	Model                 string          `json:"model"`
	Image                 string          `json:"image,omitempty"`
	Host                  string          `json:"host"`
	MinicapFPS            string          `json:"minicap_fps,omitempty"`
	MinicapHalfResolution string          `json:"minicap_half_resolution,omitempty"`
	UseMinicap            string          `json:"use_minicap,omitempty"`
}

func UpdateDevices() {
	configDevices := createDevicesFromConfig()
	if configDevices == nil {
		log.WithFields(log.Fields{
			"event": "device_listener",
		}).Warn("There are no devices registered in config.json. Please add devices and restart the provider!")
	}

	go func() {
		for {
			for _, device := range configDevices {
				fmt.Printf("Device: %v %v\n", device.UDID, device.State)
			}
			time.Sleep(2 * time.Second)
		}
	}()

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
						fmt.Printf("Restarting container for %v \n", configDevice)
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

// Create initial devices from the json config
func createDevicesFromConfig() []*Device {
	var devices []*Device
	for index, configDevice := range provider.ConfigData.DeviceConfig {
		device := &Device{
			State:                 "Disconnected",
			UDID:                  configDevice.DeviceUDID,
			OS:                    configDevice.OS,
			AppiumPort:            strconv.Itoa(4841 + index),
			StreamPort:            strconv.Itoa(20101 + index),
			ContainerServerPort:   strconv.Itoa(20201 + index),
			WDAPort:               strconv.Itoa(20001 + index),
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
				ContainerPorts:  container.Ports,
				ContainerStatus: container.Status,
				ImageName:       container.Image,
				ContainerName:   containerName,
			}
			device.Container = deviceContainer
			return true, nil
		}
	}
	return false, nil
}

func (device *Device) restartContainer() {
	if device.State != "Restarting" {
		// Set the current device state to Restarting
		device.State = "Restarting"

		// Get the container ID of the device container
		containerID := device.Container.ContainerID
		ctx := context.Background()

		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "docker_container_restart",
			}).Error("Could not create docker client while attempting to restart container with ID: " + containerID + ". Error: " + err.Error())
			device.State = "Failed restart"
			return
		}
		defer cli.Close()

		// Try to restart the container
		if err := cli.ContainerRestart(ctx, containerID, nil); err != nil {
			log.WithFields(log.Fields{
				"event": "docker_container_restart",
			}).Error("Could not restart container with ID: " + containerID + ". Error: " + err.Error())
			device.State = "Failed restart"
			return
		}

		log.WithFields(log.Fields{
			"event": "docker_container_restart",
		}).Info("Successfully attempted to restart container with ID: " + containerID)
		device.State = "Available"
		return
	}
}

func (device *Device) removeContainer() {
	containerID := device.Container.ContainerID

	if device.State != "Removing" {
		device.State = "Removing"
	}
	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Attempting to remove container with ID: " + containerID)

	// Create a new context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not create docker client while attempting to remove container with ID: " + containerID + ". Error: " + err.Error())
		device.State = "Failed removing"
		return
	}
	defer cli.Close()

	// Stop the container by the provided container ID
	if err := cli.ContainerStop(ctx, containerID, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + containerID + ". Error: " + err.Error())
		device.State = "Failed removing"
		return
	}

	// Remove the stopped container
	if err := cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{}); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + containerID + ". Error: " + err.Error())
		device.State = "Failed removing"
		return
	}

	device.State = "Disconnected"
	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Successfully removed container with ID: " + containerID)
}

func (device *Device) createIOSContainer() {
	mutex.Lock()
	defer mutex.Unlock()

	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Attempting to create a container for iOS device with udid: " + device.UDID)

	time.Sleep(2 * time.Second)

	// Create docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + device.UDID)
		return
	}
	defer cli.Close()

	// Create the container config
	config := &container.Config{
		Image: "ios-appium",
		ExposedPorts: nat.PortSet{
			nat.Port("4723"):                     struct{}{},
			nat.Port("8100"):                     struct{}{},
			nat.Port("9100"):                     struct{}{},
			nat.Port(device.ContainerServerPort): struct{}{},
		},
		Env: []string{"ON_GRID=" + provider.ConfigData.EnvConfig.ConnectSeleniumGrid,
			"APPIUM_PORT=" + device.AppiumPort,
			"DEVICE_UDID=" + device.UDID,
			"DEVICE_OS_VERSION=" + device.OSVersion,
			"DEVICE_NAME=" + device.Name,
			"WDA_BUNDLEID=" + provider.ConfigData.AppiumConfig.WDABundleID,
			"SUPERVISION_PASSWORD=" + provider.ConfigData.EnvConfig.SupervisionPassword,
			"SELENIUM_HUB_PORT=" + provider.ConfigData.AppiumConfig.SeleniumHubPort,
			"SELENIUM_HUB_HOST=" + provider.ConfigData.AppiumConfig.SeleniumHubHost,
			"DEVICES_HOST=" + provider.ConfigData.AppiumConfig.DevicesHost,
			"HUB_PROTOCOL=" + provider.ConfigData.AppiumConfig.SeleniumHubProtocolType,
			"CONTAINERIZED_USBMUXD=" + provider.ConfigData.EnvConfig.ContainerizedUsbmuxd,
			"SCREEN_SIZE=" + device.ScreenSize,
			"CONTAINER_SERVER_PORT=" + device.ContainerServerPort,
			"DEVICE_MODEL=" + device.Model,
			"DEVICE_OS=ios"},
	}

	var mounts []mount.Mount
	var resources container.Resources

	if provider.ConfigData.EnvConfig.ContainerizedUsbmuxd == "false" {
		// Mount all iOS devices on the host to the container with /var/run/usbmuxd
		// Mount /var/lib/lockdown so you don't have to trust device on each new container
		mounts = []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: "/var/lib/lockdown",
				Target: "/var/lib/lockdown",
			},
			{
				Type:   mount.TypeBind,
				Source: "/var/run/usbmuxd",
				Target: "/var/run/usbmuxd",
			},
		}
	} else {
		resources = container.Resources{
			Devices: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/device_ios_" + device.UDID,
					PathInContainer:   "/dev/bus/usb/003/011",
					CgroupPermissions: "rwm",
				},
			},
		}
	}

	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: provider.ProjectDir + "/logs/container_" + device.Name + "-" + device.UDID,
		Target: "/opt/logs",
	})

	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: provider.ProjectDir + "/apps",
		Target: "/opt/ipa",
	})

	// Create the host config
	hostConfig := &container.HostConfig{
		Privileged:    true,
		RestartPolicy: container.RestartPolicy{Name: "on-failure", MaximumRetryCount: 3},
		PortBindings: nat.PortMap{
			nat.Port("4723"): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: device.AppiumPort,
				},
			},
			nat.Port("8100"): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: device.WDAPort,
				},
			},
			nat.Port("9100"): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: device.StreamPort,
				},
			},
			nat.Port(device.ContainerServerPort): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: device.ContainerServerPort,
				},
			},
		},
		Mounts:    mounts,
		Resources: resources,
	}

	// Create a folder for logging for the container
	err = os.MkdirAll("./logs/container_"+device.Name+"-"+device.UDID, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create logs folder when attempting to create a container for device with udid: " + device.UDID + ". Error: " + err.Error())
		return
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "iosDevice_"+device.UDID)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create a container for device with udid: " + device.UDID + ". Error: " + err.Error())
		return
	}

	// Start the container
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not start container for device with udid: " + device.UDID + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Successfully created a container for iOS device with udid: " + device.UDID)
}

func (device *Device) createAndroidContainer() {
	mutex.Lock()
	defer mutex.Unlock()

	log.WithFields(log.Fields{
		"event": "android_container_create",
	}).Info("Attempting to create a container for Android device with udid: " + device.UDID)

	// Get the device config data
	deviceName := device.Name
	deviceOSVersion := device.OSVersion
	seleniumHubPort := provider.ConfigData.AppiumConfig.SeleniumHubPort
	seleniumHubHost := provider.ConfigData.AppiumConfig.SeleniumHubHost
	devicesHost := provider.ConfigData.AppiumConfig.DevicesHost
	hubProtocol := provider.ConfigData.AppiumConfig.SeleniumHubProtocolType
	deviceModel := device.Model
	remoteControl := provider.ConfigData.EnvConfig.RemoteControl
	minicapFPS := device.MinicapFPS
	minicapHalfResolution := device.MinicapHalfResolution
	screenSize := device.ScreenSize
	screenSizeValues := strings.Split(screenSize, "x")
	useMinicap := device.UseMinicap

	// Create the docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + device.UDID)
		return
	}
	defer cli.Close()

	environmentVars := []string{"ON_GRID=" + provider.ConfigData.EnvConfig.ConnectSeleniumGrid,
		"APPIUM_PORT=" + device.AppiumPort,
		"DEVICE_UDID=" + device.UDID,
		"DEVICE_OS_VERSION=" + deviceOSVersion,
		"DEVICE_NAME=" + deviceName,
		"SELENIUM_HUB_PORT=" + seleniumHubPort,
		"SELENIUM_HUB_HOST=" + seleniumHubHost,
		"DEVICES_HOST=" + devicesHost,
		"HUB_PROTOCOL=" + hubProtocol,
		"CONTAINER_SERVER_PORT=" + device.ContainerServerPort,
		"DEVICE_MODEL=" + deviceModel,
		"REMOTE_CONTROL=" + remoteControl,
		"MINICAP_FPS=" + minicapFPS,
		"MINICAP_HALF_RESOLUTION=" + minicapHalfResolution,
		"SCREEN_WIDTH=" + screenSizeValues[0],
		"SCREEN_HEIGHT=" + screenSizeValues[1],
		"SCREEN_SIZE=" + screenSize,
		"DEVICE_OS=android"}

	if useMinicap != "" {
		environmentVars = append(environmentVars, "USE_MINICAP="+useMinicap)
	}

	// Create the container config
	config := &container.Config{
		Image: "android-appium",
		ExposedPorts: nat.PortSet{
			nat.Port("4723"):                     struct{}{},
			nat.Port(device.ContainerServerPort): struct{}{},
		},
		Env: environmentVars,
	}

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: provider.ProjectDir + "/logs/container_" + deviceName + "-" + device.UDID,
			Target: "/opt/logs",
		},
		{
			Type:   mount.TypeBind,
			Source: provider.ProjectDir + "/apps",
			Target: "/opt/apk",
		},
		{
			Type:   mount.TypeBind,
			Source: provider.HomeDir + "/.android",
			Target: "/root/.android",
		},
		{
			Type:        mount.TypeBind,
			Source:      "/dev/device_android_" + device.UDID,
			Target:      "/dev/device_android_" + device.UDID,
			BindOptions: &mount.BindOptions{Propagation: "shared"},
		},
	}

	if remoteControl == "true" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: provider.ProjectDir + "/minicap",
			Target: "/root/minicap",
		})
	}

	resources := container.Resources{
		Devices: []container.DeviceMapping{
			{
				PathOnHost:        "/dev/device_android_" + device.UDID,
				PathInContainer:   "/dev/bus/usb/003/011",
				CgroupPermissions: "rwm",
			},
		},
	}

	// Create the host config
	host_config := &container.HostConfig{
		Privileged:    true,
		RestartPolicy: container.RestartPolicy{Name: "on-failure", MaximumRetryCount: 3},
		PortBindings: nat.PortMap{
			nat.Port("4723"): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: device.AppiumPort,
				},
			},
			nat.Port(device.ContainerServerPort): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: device.ContainerServerPort,
				},
			},
		},
		Mounts:    mounts,
		Resources: resources,
	}

	// Create a folder for logging for the container
	err = os.MkdirAll("./logs/container_"+deviceName+"-"+device.UDID, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create logs folder when attempting to create a container for device with udid: " + device.UDID + ". Error: " + err.Error())
		return
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, config, host_config, nil, nil, "androidDevice_"+device.UDID)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create a container for device with udid: " + device.UDID + ". Error: " + err.Error())
		return
	}

	// Start the container
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not start container for device with udid: " + device.UDID + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "android_container_create",
	}).Info("Successfully created a container for Android device with udid: " + device.UDID)
}

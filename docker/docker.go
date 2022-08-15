package docker

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/shamanec/GADS-devices-provider/provider"
	"github.com/shamanec/GADS-devices-provider/util"
	log "github.com/sirupsen/logrus"
)

// Create an iOS container for a specific device(by UDID) using data from config.json so if device is not registered there it will not attempt to create a container for it
func CreateIOSContainer(deviceUDID string) {
	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Attempting to create a container for iOS device with udid: " + deviceUDID)

	time.Sleep(2 * time.Second)

	// Check if device is registered in config data
	var deviceConfig util.DeviceConfig
	for _, v := range provider.ConfigData.DeviceConfig {
		if v.DeviceUDID == deviceUDID {
			deviceConfig = v
		}
	}

	// Stop execution if device not in config data
	if deviceConfig == (util.DeviceConfig{}) {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Device with UDID:" + deviceUDID + " is not registered in the 'config.json' file. No container will be created.")
		return
	}

	// Get the device specific config data
	appiumPort := deviceConfig.AppiumPort
	deviceName := deviceConfig.DeviceName
	deviceOSVersion := deviceConfig.DeviceOSVersion
	wdaMjpegPort := deviceConfig.StreamPort
	wdaPort := deviceConfig.WDAPort
	wdaBundleID := provider.ConfigData.AppiumConfig.WDABundleID
	seleniumHubPort := provider.ConfigData.AppiumConfig.SeleniumHubPort
	seleniumHubHost := provider.ConfigData.AppiumConfig.SeleniumHubHost
	devicesHost := provider.ConfigData.AppiumConfig.DevicesHost
	hubProtocol := provider.ConfigData.AppiumConfig.SeleniumHubProtocolType
	containerizedUsbmuxd := provider.ConfigData.EnvConfig.ContainerizedUsbmuxd
	screenSize := deviceConfig.ScreenSize
	containerServerPort := deviceConfig.ContainerServerPort
	deviceModel := deviceConfig.DeviceModel

	// Create docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + deviceUDID)
		return
	}

	// Create the container config
	config := &container.Config{
		Image: "ios-appium",
		ExposedPorts: nat.PortSet{
			nat.Port("4723"):              struct{}{},
			nat.Port(wdaPort):             struct{}{},
			nat.Port(wdaMjpegPort):        struct{}{},
			nat.Port(containerServerPort): struct{}{},
		},
		Env: []string{"ON_GRID=" + provider.ConfigData.EnvConfig.ConnectSeleniumGrid,
			"APPIUM_PORT=" + appiumPort,
			"DEVICE_UDID=" + deviceUDID,
			"WDA_PORT=" + wdaPort,
			"MJPEG_PORT=" + wdaMjpegPort,
			"DEVICE_OS_VERSION=" + deviceOSVersion,
			"DEVICE_NAME=" + deviceName,
			"WDA_BUNDLEID=" + wdaBundleID,
			"SUPERVISION_PASSWORD=" + provider.ConfigData.EnvConfig.SupervisionPassword,
			"SELENIUM_HUB_PORT=" + seleniumHubPort,
			"SELENIUM_HUB_HOST=" + seleniumHubHost,
			"DEVICES_HOST=" + devicesHost,
			"HUB_PROTOCOL=" + hubProtocol,
			"CONTAINERIZED_USBMUXD=" + containerizedUsbmuxd,
			"SCREEN_SIZE=" + screenSize,
			"CONTAINER_SERVER_PORT=" + containerServerPort,
			"DEVICE_MODEL=" + deviceModel,
			"DEVICE_OS=ios"},
	}

	var mounts []mount.Mount
	var resources container.Resources

	if containerizedUsbmuxd == "false" {
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
					PathOnHost:        "/dev/device_" + deviceUDID,
					PathInContainer:   "/dev/bus/usb/003/011",
					CgroupPermissions: "rwm",
				},
			},
		}
	}

	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: provider.ProjectDir + "/logs/container_" + deviceName + "-" + deviceUDID,
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
					HostPort: appiumPort,
				},
			},
			nat.Port(wdaPort): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: wdaPort,
				},
			},
			nat.Port(wdaMjpegPort): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: wdaMjpegPort,
				},
			},
			nat.Port(containerServerPort): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: containerServerPort,
				},
			},
		},
		Mounts:    mounts,
		Resources: resources,
	}

	// Create a folder for logging for the container
	err = os.MkdirAll("./logs/container_"+deviceName+"-"+deviceUDID, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create logs folder when attempting to create a container for device with udid: " + deviceUDID + ". Error: " + err.Error())
		return
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "iosDevice_"+deviceUDID)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create a container for device with udid: " + deviceUDID + ". Error: " + err.Error())
		return
	}

	// Start the container
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not start container for device with udid: " + deviceUDID + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Successfully created a container for iOS device with udid: " + deviceUDID)
}

// Create an Android container for a specific device(by UDID) using data from config.json so if device is not registered there it will not attempt to create a container for it
// If container already exists for this device it will do nothing
func CreateAndroidContainer(deviceUDID string) {
	log.WithFields(log.Fields{
		"event": "android_container_create",
	}).Info("Attempting to create a container for Android device with udid: " + deviceUDID)

	// Check if device is registered in config data
	var deviceConfig util.DeviceConfig
	for _, v := range provider.ConfigData.DeviceConfig {
		if v.DeviceUDID == deviceUDID {
			deviceConfig = v
		}
	}

	// Stop execution if device not in config data
	if deviceConfig == (util.DeviceConfig{}) {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Device with UDID:" + deviceUDID + " is not registered in the 'config.json' file. No container will be created.")
		return
	}

	// Get the device config data
	appiumPort := deviceConfig.AppiumPort
	deviceName := deviceConfig.DeviceName
	deviceOSVersion := deviceConfig.DeviceOSVersion
	seleniumHubPort := provider.ConfigData.AppiumConfig.SeleniumHubPort
	seleniumHubHost := provider.ConfigData.AppiumConfig.SeleniumHubHost
	devicesHost := provider.ConfigData.AppiumConfig.DevicesHost
	hubProtocol := provider.ConfigData.AppiumConfig.SeleniumHubProtocolType
	containerServerPort := deviceConfig.ContainerServerPort
	deviceModel := deviceConfig.DeviceModel
	remoteControl := provider.ConfigData.EnvConfig.RemoteControl
	minicapFPS := deviceConfig.MinicapFPS
	minicapHalfResolution := deviceConfig.MinicapHalfResolution
	screenSize := deviceConfig.ScreenSize
	screenSizeValues := strings.Split(screenSize, "x")

	// Create the docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + deviceUDID)
		return
	}

	// Create the container config
	config := &container.Config{
		Image: "android-appium",
		ExposedPorts: nat.PortSet{
			nat.Port("4723"):              struct{}{},
			nat.Port(containerServerPort): struct{}{},
		},
		Env: []string{"ON_GRID=" + provider.ConfigData.EnvConfig.ConnectSeleniumGrid,
			"APPIUM_PORT=" + appiumPort,
			"DEVICE_UDID=" + deviceUDID,
			"DEVICE_OS_VERSION=" + deviceOSVersion,
			"DEVICE_NAME=" + deviceName,
			"SELENIUM_HUB_PORT=" + seleniumHubPort,
			"SELENIUM_HUB_HOST=" + seleniumHubHost,
			"DEVICES_HOST=" + devicesHost,
			"HUB_PROTOCOL=" + hubProtocol,
			"CONTAINER_SERVER_PORT=" + containerServerPort,
			"DEVICE_MODEL=" + deviceModel,
			"REMOTE_CONTROL=" + remoteControl,
			"MINICAP_FPS=" + minicapFPS,
			"MINICAP_HALF_RESOLUTION=" + minicapHalfResolution,
			"SCREEN_WIDTH=" + screenSizeValues[0],
			"SCREEN_HEIGHT=" + screenSizeValues[1],
			"SCREEN_SIZE=" + screenSize,
			"DEVICE_OS=android"},
	}

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: provider.ProjectDir + "/logs/container_" + deviceName + "-" + deviceUDID,
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
			Source:      "/dev/device_" + deviceUDID,
			Target:      "/dev/device_" + deviceUDID,
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
				PathOnHost:        "/dev/device_" + deviceUDID,
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
					HostPort: appiumPort,
				},
			},
			nat.Port(containerServerPort): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: containerServerPort,
				},
			},
		},
		Mounts:    mounts,
		Resources: resources,
	}

	// Create a folder for logging for the container
	err = os.MkdirAll("./logs/container_"+deviceName+"-"+deviceUDID, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create logs folder when attempting to create a container for device with udid: " + deviceUDID + ". Error: " + err.Error())
		return
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, config, host_config, nil, nil, "androidDevice_"+deviceUDID)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create a container for device with udid: " + deviceUDID + ". Error: " + err.Error())
		return
	}

	// Start the container
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not start container for device with udid: " + deviceUDID + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "android_container_create",
	}).Info("Successfully created a container for Android device with udid: " + deviceUDID)
}

// Check if container exists by name and also return container_id
func CheckContainerExistsByName(deviceUDID string) (bool, string, string) {
	// Get all the containers
	containers, _ := getContainersList()
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

// Restart a docker container by provided container ID
func RestartContainer(container_id string) error {
	// Create a new context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_restart",
		}).Error("Could not create docker client while attempting to restart container with ID: " + container_id + ". Error: " + err.Error())
		return err
	}

	// Try to restart the container
	if err := cli.ContainerRestart(ctx, container_id, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_restart",
		}).Error("Could not restart container with ID: " + container_id + ". Error: " + err.Error())
		return err
	}

	log.WithFields(log.Fields{
		"event": "docker_container_restart",
	}).Info("Successfully attempted to restart container with ID: " + container_id)

	return nil
}

// Remove any docker container by container ID
func RemoveContainerByID(containerID string) {
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
		return
	}

	// Stop the container by the provided container ID
	if err := cli.ContainerStop(ctx, containerID, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + containerID + ". Error: " + err.Error())
		return
	}

	// Remove the stopped container
	if err := cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{}); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + containerID + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Successfully removed container with ID: " + containerID)
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

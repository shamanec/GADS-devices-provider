package device

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

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
	// Remove the container info from the device
	// Regardless of the removal outcome
	defer func() {
		device.Container = nil
	}()

	// Get the ID of the device container
	containerID := device.Container.ContainerID

	// Check if the container is not already being removed by checking the state
	if device.State != "Removing" {
		// If device is not already being removed - set state to Removing
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
		device.State = "Failed stopping"
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
	containerConfig := &container.Config{
		Image: "ios-appium",
		ExposedPorts: nat.PortSet{
			nat.Port("4723"):                     struct{}{},
			nat.Port("8100"):                     struct{}{},
			nat.Port("9100"):                     struct{}{},
			nat.Port(device.ContainerServerPort): struct{}{},
		},
		Env: []string{"ON_GRID=" + Config.EnvConfig.ConnectSeleniumGrid,
			"APPIUM_PORT=" + device.AppiumPort,
			"DEVICE_UDID=" + device.UDID,
			"DEVICE_OS_VERSION=" + device.OSVersion,
			"DEVICE_NAME=" + device.Name,
			"WDA_BUNDLEID=" + Config.AppiumConfig.WDABundleID,
			"SUPERVISION_PASSWORD=" + Config.EnvConfig.SupervisionPassword,
			"SELENIUM_HUB_PORT=" + Config.AppiumConfig.SeleniumHubPort,
			"SELENIUM_HUB_HOST=" + Config.AppiumConfig.SeleniumHubHost,
			"DEVICES_HOST=" + Config.AppiumConfig.DevicesHost,
			"HUB_PROTOCOL=" + Config.AppiumConfig.SeleniumHubProtocolType,
			"CONTAINERIZED_USBMUXD=" + Config.EnvConfig.ContainerizedUsbmuxd,
			"SCREEN_SIZE=" + device.ScreenSize,
			"CONTAINER_SERVER_PORT=" + device.ContainerServerPort,
			"DEVICE_MODEL=" + device.Model,
			"DEVICE_OS=ios"},
	}

	var mounts []mount.Mount
	var resources container.Resources

	if Config.EnvConfig.ContainerizedUsbmuxd == "false" {
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
		Source: projectDir + "/logs/container_" + device.Name + "-" + device.UDID,
		Target: "/opt/logs",
	})

	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: projectDir + "/apps",
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
	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "iosDevice_"+device.UDID)
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
	seleniumHubPort := Config.AppiumConfig.SeleniumHubPort
	seleniumHubHost := Config.AppiumConfig.SeleniumHubHost
	devicesHost := Config.AppiumConfig.DevicesHost
	hubProtocol := Config.AppiumConfig.SeleniumHubProtocolType
	deviceModel := device.Model
	remoteControl := Config.EnvConfig.RemoteControl
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

	environmentVars := []string{"ON_GRID=" + Config.EnvConfig.ConnectSeleniumGrid,
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
	containerConfig := &container.Config{
		Image: "android-appium",
		ExposedPorts: nat.PortSet{
			nat.Port("4723"):                     struct{}{},
			nat.Port(device.ContainerServerPort): struct{}{},
		},
		Env: environmentVars,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not get OS home dir, will try to fallback to $HOME, err: " + err.Error())
		homeDir = "$HOME"
	}

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: projectDir + "/logs/container_" + deviceName + "-" + device.UDID,
			Target: "/opt/logs",
		},
		{
			Type:   mount.TypeBind,
			Source: projectDir + "/apps",
			Target: "/opt/apk",
		},
		{
			Type:   mount.TypeBind,
			Source: homeDir + "/.android",
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
			Source: projectDir + "/minicap",
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
	resp, err := cli.ContainerCreate(ctx, containerConfig, host_config, nil, nil, "androidDevice_"+device.UDID)
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

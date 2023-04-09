package device

import (
	"context"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
)

var restartedContainers = make(map[string]int)
var removedContainers = make(map[string]int)
var createdContainers = make(map[string]int)

// Restart a device container
func (device *Device) restartContainer() {
	// Get the container ID of the device container
	containerID := device.Container.ContainerID

	// Delete the container from the map with containers being restarted when the function returns
	defer delete(restartedContainers, containerID)

	// Check if the container is already in the map with containers being restarted
	_, ok := restartedContainers[containerID]

	// If its not in the map, then its not being restarted
	// So we can initiate a restart
	if !ok {
		// Add the container to the map with containers being restarted
		restartedContainers[containerID] = 1

		// Check the Docker client
		if cli == nil {
			err := initDockerClient()
			if err != nil {
				log.WithFields(log.Fields{
					"event": "docker_container_restart",
				}).Error("Could not create docker client while attempting to restart container with ID: " + containerID + ": " + err.Error())
			}
		}

		ctx := context.Background()

		// Try to restart the container
		if err := cli.ContainerRestart(ctx, containerID, nil); err != nil {
			log.WithFields(log.Fields{
				"event": "docker_container_restart",
			}).Error("Could not restart container with ID: " + containerID + ": " + err.Error())
			return
		}

		log.WithFields(log.Fields{
			"event": "docker_container_restart",
		}).Info("Successfully attempted to restart container with ID: " + containerID)
		return
	}

	// Delete the container from the map with containers being restarted
	delete(restartedContainers, containerID)
}

// Remove a device container
func (device *Device) removeContainer() {
	// Get the ID of the device container
	containerID := device.Container.ContainerID

	// Delete the container from the map with containers being removed when the function returns
	defer delete(removedContainers, containerID)

	// If its not in the map, then its not being removed
	// So we can initiate a removal
	_, ok := removedContainers[containerID]
	if !ok {
		// Add the container to the map with containers being removed
		removedContainers[containerID] = 1

		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Info("Attempting to remove container with ID: " + containerID)

		// Check the Docker client
		if cli == nil {
			err := initDockerClient()
			if err != nil {
				log.WithFields(log.Fields{
					"event": "docker_container_remove",
				}).Error("Could not create docker client while attempting to remove container with ID: " + containerID + ": " + err.Error())
				return
			}
		}

		ctx := context.Background()

		// Stop the container by the provided container ID
		if err := cli.ContainerStop(ctx, containerID, nil); err != nil {
			log.WithFields(log.Fields{
				"event": "docker_container_remove",
			}).Error("Could not remove container with ID: " + containerID + ": " + err.Error())
			return
		}

		// Remove the stopped container
		if err := cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{}); err != nil {
			log.WithFields(log.Fields{
				"event": "docker_container_remove",
			}).Error("Could not remove container with ID: " + containerID + ": " + err.Error())
			return
		}

		// Remove the container from the device pointer and update the DB
		device.Container = nil
		device.updateDB()
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Info("Successfully removed container with ID: " + containerID)
	}

	// Delete the container from the map with containers being removed
	delete(removedContainers, containerID)
}

// Create an iOS device container
func (device *Device) createIOSContainer() {
	// Delete the container from the map with containers being created when the function returns
	defer delete(createdContainers, device.UDID)

	_, ok := createdContainers[device.UDID]
	if !ok {
		// Add the container to the map with containers being removed
		createdContainers[device.UDID] = 1

		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Info("Attempting to create a container for iOS device with udid: " + device.UDID)

		time.Sleep(2 * time.Second)

		ctx := context.Background()
		if cli == nil {
			err := initDockerClient()
			if err != nil {
				log.WithFields(log.Fields{
					"event": "ios_container_create",
				}).Error("Could not create docker client when attempting to create a container for device with udid: " + device.UDID)
				return
			}
		}

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
				"WDA_BUNDLEID=" + Config.EnvConfig.WDABundleID,
				"SUPERVISION_PASSWORD=" + Config.EnvConfig.SupervisionPassword,
				"SELENIUM_HUB_PORT=" + Config.AppiumConfig.SeleniumHubPort,
				"SELENIUM_HUB_HOST=" + Config.AppiumConfig.SeleniumHubHost,
				"DEVICES_HOST=" + Config.EnvConfig.DevicesHost,
				"HUB_PROTOCOL=" + Config.AppiumConfig.SeleniumHubProtocolType,
				"SCREEN_SIZE=" + device.ScreenSize,
				"CONTAINER_SERVER_PORT=" + device.ContainerServerPort,
				"DEVICE_MODEL=" + device.Model,
				"DEVICE_OS=ios"},
		}

		var mounts []mount.Mount
		var resources container.Resources

		resources = container.Resources{
			Devices: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/device_ios_" + device.UDID,
					PathInContainer:   "/dev/bus/usb/003/011",
					CgroupPermissions: "rwm",
				},
			},
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
		err := os.MkdirAll("./logs/container_"+device.Name+"-"+device.UDID, os.ModePerm)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "ios_container_create",
			}).Error("Could not create logs folder when attempting to create a container for device with udid: " + device.UDID + ": " + err.Error())
			return
		}

		// Create the container
		resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "iosDevice_"+device.UDID)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "ios_container_create",
			}).Error("Could not create a container for device with udid: " + device.UDID + ": " + err.Error())
			return
		}

		// Start the container
		err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
		if err != nil {
			log.WithFields(log.Fields{
				"event": "ios_container_create",
			}).Error("Could not start container for device with udid: " + device.UDID + ": " + err.Error())
			return
		}

		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Info("Successfully created a container for iOS device with udid: " + device.UDID)
	}
	// Delete the container from the map with containers being created
	delete(createdContainers, device.UDID)
}

// Create an Android device container
func (device *Device) createAndroidContainer() {
	// Delete the container from the map with containers being created when the function returns
	defer delete(createdContainers, device.UDID)

	_, ok := createdContainers[device.UDID]
	if !ok {
		// Add the container to the map with containers being removed
		createdContainers[device.UDID] = 1

		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Info("Attempting to create a container for Android device with udid: " + device.UDID)

		// Get the device config data
		screenSizeValues := strings.Split(device.ScreenSize, "x")

		// Create the docker client

		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "android_container_create",
			}).Error("Could not create docker client when attempting to create a container for device with udid: " + device.UDID)
			return
		}

		ctx := context.Background()

		environmentVars := []string{"ON_GRID=" + Config.EnvConfig.ConnectSeleniumGrid,
			"APPIUM_PORT=" + device.AppiumPort,
			"DEVICE_UDID=" + device.UDID,
			"DEVICE_OS_VERSION=" + device.OSVersion,
			"DEVICE_NAME=" + device.Name,
			"SELENIUM_HUB_PORT=" + Config.AppiumConfig.SeleniumHubPort,
			"SELENIUM_HUB_HOST=" + Config.AppiumConfig.SeleniumHubHost,
			"DEVICES_HOST=" + Config.EnvConfig.DevicesHost,
			"HUB_PROTOCOL=" + Config.AppiumConfig.SeleniumHubProtocolType,
			"CONTAINER_SERVER_PORT=" + device.ContainerServerPort,
			"DEVICE_MODEL=" + device.Model,
			"REMOTE_CONTROL=" + Config.EnvConfig.RemoteControl,
			"MINICAP_FPS=" + device.MinicapFPS,
			"MINICAP_HALF_RESOLUTION=" + device.MinicapHalfResolution,
			"SCREEN_WIDTH=" + screenSizeValues[0],
			"SCREEN_HEIGHT=" + screenSizeValues[1],
			"SCREEN_SIZE=" + device.ScreenSize,
			"DEVICE_OS=android"}

		if device.UseMinicap != "" {
			environmentVars = append(environmentVars, "USE_MINICAP="+device.UseMinicap)
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
			}).Warn("Could not get home dir using os.UserHomeDir: " + err.Error())
			user, err := user.Current()
			if err != nil {
				log.WithFields(log.Fields{
					"event": "android_container_create",
				}).Error("Could not get home dir through current user: " + err.Error())
				return
			}
			homeDir = user.HomeDir
		}

		mounts := []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: projectDir + "/logs/container_" + device.Name + "-" + device.UDID,
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

		if Config.EnvConfig.RemoteControl == "true" {
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
		err = os.MkdirAll("./logs/container_"+device.Name+"-"+device.UDID, os.ModePerm)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "android_container_create",
			}).Error("Could not create logs folder when attempting to create a container for device with udid: " + device.UDID + ": " + err.Error())
			return
		}

		// Create the container
		resp, err := cli.ContainerCreate(ctx, containerConfig, host_config, nil, nil, "androidDevice_"+device.UDID)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "android_container_create",
			}).Error("Could not create a container for device with udid: " + device.UDID + ": " + err.Error())
			return
		}

		// Start the container
		err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
		if err != nil {
			log.WithFields(log.Fields{
				"event": "android_container_create",
			}).Error("Could not start container for device with udid: " + device.UDID + ": " + err.Error())
			return
		}

		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Info("Successfully created a container for Android device with udid: " + device.UDID)
	}
	// Delete the container from the map with containers being created
	delete(createdContainers, device.UDID)
}

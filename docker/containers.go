package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
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

var createContainerUDIDs = make(map[string]struct{})
var removeContainerIDs = make(map[string]struct{})
var restartContainerIDs = make(map[string]struct{})
var containerMutex sync.Mutex

func CheckDevices() {
	for {
		// Get all files in /dev (we create symlinks for devices through udev rules)
		filesInDev, err := ioutil.ReadDir("/dev")
		if err != nil {
			log.Fatal(err)
		}

		// Get all connected devices UDIDs (from /dev symlinks) into a slice
		containerMutex.Lock()
		var deviceUDIDs []string
		for _, fileInDev := range filesInDev {
			if strings.HasPrefix(fileInDev.Name(), "device") {
				deviceUDIDs = append(deviceUDIDs, strings.Split(fileInDev.Name(), "_")[1])
			}
		}
		containerMutex.Unlock()

		// Get a slice of running containers
		containers, _ := getDeviceContainersList()

		// If we have less connected devices than running containers
		if len(deviceUDIDs) < len(containers) {
			handleDisconnectedDeviceContainers(containers, deviceUDIDs)
		}

		if len(deviceUDIDs) >= len(containers) {
			var deviceContainerID string
			var deviceContainerStatus string

			for _, udid := range deviceUDIDs {
				device_has_container := false
				for _, container := range containers {
					containerName := container.Names[0]
					if strings.Contains(containerName, udid) {
						deviceContainerID = container.ID
						deviceContainerStatus = container.Status
						device_has_container = true
					}

				}

				if device_has_container && !strings.Contains(deviceContainerStatus, "Up") {
					containerMutex.Lock()
					if _, ok := restartContainerIDs[deviceContainerID]; ok {
						log.WithFields(log.Fields{
							"event": "restart_container",
						}).Info("Container for device with UDID:" + udid + " already being restarted.")
						containerMutex.Unlock()
						continue
					}
					containerMutex.Unlock()

					handleConnectedDeviceExistingContainer(deviceContainerID)
				}

				if !device_has_container {
					handleConnectedDeviceNewContainer(udid)
				}
			}
		}

		time.Sleep(15 * time.Second)
	}
}

func handleConnectedDeviceExistingContainer(deviceContainerID string) {
	// Check if container for this device is already being restarted (its in the map)
	containerMutex.Lock()
	defer containerMutex.Unlock()

	// If the container was not in the map
	// we add it to the map and initiate a restart
	// container will be removed from the map regardless of the restart result
	restartContainerIDs[deviceContainerID] = struct{}{}

	go RestartContainer(deviceContainerID)
}

func handleConnectedDeviceNewContainer(udid string) {
	for _, value := range provider.ConfigData.DeviceConfig {
		if value.DeviceUDID == udid {
			osType := value.OS

			// Check if a container for the device is already being created (its in the map)
			// and continue to next iteration if it is
			containerMutex.Lock()
			if _, ok := createContainerUDIDs[udid]; ok {
				log.WithFields(log.Fields{
					"event": "restart_container",
				}).Info("Container for device with UDID:" + udid + " already being created.")
				containerMutex.Unlock()
				continue
			}

			createContainerUDIDs[udid] = struct{}{}
			containerMutex.Unlock()

			if osType == "ios" {
				fmt.Println("Creating container: " + udid)
				go CreateIOSContainer(udid)
			} else if osType == "android" {
				fmt.Println("Creating container: " + udid)
				go CreateAndroidContainer(udid)
			}
		}
	}
}

func handleDisconnectedDeviceContainers(containers []types.Container, deviceUDIDs []string) {
	// Loop through the available device containers
	for _, container := range containers {
		device_for_container := false
		// Get the current container name
		containerName := container.Names[0]

		// Loop through the connected devices UDIDs
		// if we have a device connected for the current container
		// we set device_for_container to `true``
		for _, udid := range deviceUDIDs {
			if strings.Contains(containerName, udid) {
				device_for_container = true
			}
		}

		// If we don't have a connected device for a specific container
		if !device_for_container {
			// Check if container for this device is already being removed (its in the map)
			containerMutex.Lock()
			if _, ok := removeContainerIDs[container.ID]; ok {
				// if it was in the map
				// then we just continue the containers loop
				containerMutex.Unlock()
				continue
			}

			// If the container is not already being removed
			// We add it to the map
			// And we start the goroutine to remove the container
			removeContainerIDs[container.ID] = struct{}{}
			containerMutex.Unlock()

			go RemoveContainerByID(container.ID)
		}
	}
}

// Create an iOS container for a specific device(by UDID) using data from config.json so if device is not registered there it will not attempt to create a container for it
func CreateIOSContainer(deviceUDID string) {
	defer func() {
		containerMutex.Lock()
		defer containerMutex.Unlock()

		delete(createContainerUDIDs, deviceUDID)
	}()

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
			nat.Port("8100"):              struct{}{},
			nat.Port("9100"):              struct{}{},
			nat.Port(containerServerPort): struct{}{},
		},
		Env: []string{"ON_GRID=" + provider.ConfigData.EnvConfig.ConnectSeleniumGrid,
			"APPIUM_PORT=" + appiumPort,
			"DEVICE_UDID=" + deviceUDID,
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
			nat.Port("8100"): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: wdaPort,
				},
			},
			nat.Port("9100"): []nat.PortBinding{
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
	defer func() {
		containerMutex.Lock()
		defer containerMutex.Unlock()

		delete(createContainerUDIDs, deviceUDID)
	}()

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

// Restart a docker container by provided container ID
func RestartContainer(container_id string) error {
	fmt.Println("restarting")
	defer func() {
		containerMutex.Lock()
		defer containerMutex.Unlock()

		delete(restartContainerIDs, container_id)
	}()

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
	defer func() {
		containerMutex.Lock()
		defer containerMutex.Unlock()

		delete(removeContainerIDs, containerID)
	}()

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

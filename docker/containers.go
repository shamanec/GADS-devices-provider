package docker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/fsnotify/fsnotify"
	"github.com/shamanec/GADS-devices-provider/provider"
	"github.com/shamanec/GADS-devices-provider/util"
	log "github.com/sirupsen/logrus"
)

func DevicesWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic("Could not create watcher when preparing to watch /dev folder, err:" + err.Error())
	}
	defer watcher.Close()

	err = watcher.Add("/dev")
	if err != nil {
		panic("Could not add /dev folder to watcher when preparing to watch it, err:" + err.Error())
	}

	fmt.Println("Started listening for events in /dev folder")
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// If we have a Create event in /dev (device was connected)
				if event.Has(fsnotify.Create) {
					// Get the name of the created file
					fileName := event.Name

					// Check if the created file was a symlink for a device
					if strings.HasPrefix(fileName, "device_") {
						// Get the device OS and UDID from the symlink name
						deviceOS := strings.Split(fileName, "_")[1]
						deviceUDID := strings.Split(fileName, "_")[2]

						// Check if we have a container for the connected device
						containerExists, containerID, containerStatus := CheckContainerExistsByName(deviceUDID)

						// If we have a container and it is not `Up`
						// we restart it
						if containerExists && !strings.Contains(containerStatus, "Up") {
							fmt.Println("restarting container")
							go RestartContainer(containerID)
							continue
						}

						// If we don't have a container for the device and it is iOS
						// Create a new iOS device container for it
						if deviceOS == "ios" {
							go CreateIOSContainer(deviceUDID)
							continue
						}

						// If we don't have a container for the device and it is Android
						// Create a new Android device container for it
						if deviceOS == "android" {
							go CreateAndroidContainer(deviceUDID)
							continue
						}
					}
				}

				// If we have a Remove event in /dev (device was disconnected)
				if event.Has(fsnotify.Remove) {
					// Get the name of the removed file
					fileName := event.Name

					// Check if the removed file was a symlink for a device
					if strings.HasPrefix(fileName, "device_") {
						// Get the device UDID from the symlink name
						deviceUDID := strings.Split(fileName, "_")[2]

						// Check if container exists for the disconnected device
						containerExists, containerID, _ := CheckContainerExistsByName(deviceUDID)

						// If there is a container for the disconnected device
						// we remove it
						if containerExists {
							go RemoveContainerByID(containerID)
							continue
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.WithFields(log.Fields{
					"event": "dev_watcher",
				}).Info("There was an error with the /dev watcher: " + err.Error())
			}
		}
	}()

	// Block the DeviceWatcher() goroutine forever
	<-make(chan struct{})
}

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
					PathOnHost:        "/dev/device_ios_" + deviceUDID,
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
			Source:      "/dev/device_android_" + deviceUDID,
			Target:      "/dev/device_android_" + deviceUDID,
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
				PathOnHost:        "/dev/device_android_" + deviceUDID,
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

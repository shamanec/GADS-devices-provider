package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

var devicesList []string
var deviceContainers []types.Container
var mutex sync.Mutex

func StartDevicesListener() {
	go GetDeviceAndContainers()
	go UpdateContainers()
}

func UpdateContainers() {
	for {
		mutex.Lock()
		defer mutex.Unlock()
		if len(devicesList) >= len(deviceContainers) {
			for _, device := range devicesList {
				os := strings.Split(device, "_")[1]
				udid := strings.Split(device, "_")[2]
				hasContainer := false

			containersLoop:
				for _, container := range deviceContainers {
					containerName := strings.Replace(container.Names[0], "/", "", -1)
					if strings.Contains(containerName, udid) {
						hasContainer = true
						containerStatus := container.Status
						if !strings.Contains(containerStatus, "Up") {
							containerID := container.ID
							go RestartContainer(containerID)
							break containersLoop
						}
					}
				}

				if !hasContainer {
					appiumPort, streamPort, containerServerPort, wdaPort := GenerateDevicePorts(udid)
					if os == "ios" {
						go CreateIOSContainer(udid, appiumPort, streamPort, containerServerPort, wdaPort)
						continue
					}

					if os == "android" {
						go CreateAndroidContainer(udid, appiumPort, containerServerPort)
						continue
					}
				}
			}
		}

		if len(devicesList) < len(deviceContainers) {
			for _, container := range deviceContainers {
				containerName := strings.Replace(container.Names[0], "/", "", -1)
				hasDevice := false
				for _, device := range devicesList {
					udid := strings.Split(device, "_")[1]
					if strings.Contains(containerName, udid) {
						hasDevice = true
					}
				}

				if !hasDevice {
					containerID := container.ID
					go RemoveContainerByID(containerID)
				}
			}
		}
		mutex.Unlock()

		time.Sleep(5 * time.Second)
	}
}

func GetDeviceAndContainers() {
	for {
		mutex.Lock()
		defer mutex.Unlock()
		// Empty the deviceList var
		devicesList = []string{}

		// Get the files in /dev folder
		files, err := filepath.Glob("/dev/*")
		if err != nil {
			fmt.Println("Error listing files in /dev:", err)
			return
		}

		// Loop through the found files
		for _, file := range files {
			// Split the file to get only the file name
			_, fileName := filepath.Split(file)
			// If the filename is a device symlink
			// Add it to the devices list
			if strings.Contains(fileName, "device") {
				devicesList = append(devicesList, fileName)
			}
		}

		deviceContainers, err = getDeviceContainersList()
		if err != nil {
			fmt.Println("Could not get device containers list, err: " + err.Error())
		}

		mutex.Unlock()
		time.Sleep(2 * time.Second)
	}
}

// Create an iOS container for a specific device(by UDID) using data from config.json so if device is not registered there it will not attempt to create a container for it
func CreateIOSContainer(udid string, appiumPort string, streamPort string, containerServerPort string, wdaPort string) {
	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Attempting to create a container for iOS device with udid: " + udid)

	time.Sleep(2 * time.Second)

	// Check if device is registered in config data
	var deviceConfig util.DeviceConfig
	for _, v := range provider.ConfigData.DeviceConfig {
		if v.DeviceUDID == udid {
			deviceConfig = v
		}
	}

	// Stop execution if device not in config data
	if deviceConfig == (util.DeviceConfig{}) {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Device with UDID:" + udid + " is not registered in the 'config.json' file. No container will be created.")
		return
	}

	// Get the device specific config data
	deviceName := deviceConfig.DeviceName
	deviceOSVersion := deviceConfig.DeviceOSVersion
	wdaBundleID := provider.ConfigData.AppiumConfig.WDABundleID
	seleniumHubPort := provider.ConfigData.AppiumConfig.SeleniumHubPort
	seleniumHubHost := provider.ConfigData.AppiumConfig.SeleniumHubHost
	devicesHost := provider.ConfigData.AppiumConfig.DevicesHost
	hubProtocol := provider.ConfigData.AppiumConfig.SeleniumHubProtocolType
	containerizedUsbmuxd := provider.ConfigData.EnvConfig.ContainerizedUsbmuxd
	screenSize := deviceConfig.ScreenSize
	deviceModel := deviceConfig.DeviceModel

	// Create docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + udid)
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
			"DEVICE_UDID=" + udid,
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
					PathOnHost:        "/dev/device_ios_" + udid,
					PathInContainer:   "/dev/bus/usb/003/011",
					CgroupPermissions: "rwm",
				},
			},
		}
	}

	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: provider.ProjectDir + "/logs/container_" + deviceName + "-" + udid,
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
					HostPort: streamPort,
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
	err = os.MkdirAll("./logs/container_"+deviceName+"-"+udid, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create logs folder when attempting to create a container for device with udid: " + udid + ". Error: " + err.Error())
		return
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "iosDevice_"+udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create a container for device with udid: " + udid + ". Error: " + err.Error())
		return
	}

	// Start the container
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not start container for device with udid: " + udid + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Successfully created a container for iOS device with udid: " + udid)
}

// Create an Android container for a specific device(by UDID) using data from config.json so if device is not registered there it will not attempt to create a container for it
// If container already exists for this device it will do nothing
func CreateAndroidContainer(udid string, appiumPort string, containerServerPort string) {
	log.WithFields(log.Fields{
		"event": "android_container_create",
	}).Info("Attempting to create a container for Android device with udid: " + udid)

	// Check if device is registered in config data
	var deviceConfig util.DeviceConfig
	for _, v := range provider.ConfigData.DeviceConfig {
		if v.DeviceUDID == udid {
			deviceConfig = v
		}
	}

	// Stop execution if device not in config data
	if deviceConfig == (util.DeviceConfig{}) {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Device with UDID:" + udid + " is not registered in the 'config.json' file. No container will be created.")
		return
	}

	// Get the device config data
	deviceName := deviceConfig.DeviceName
	deviceOSVersion := deviceConfig.DeviceOSVersion
	seleniumHubPort := provider.ConfigData.AppiumConfig.SeleniumHubPort
	seleniumHubHost := provider.ConfigData.AppiumConfig.SeleniumHubHost
	devicesHost := provider.ConfigData.AppiumConfig.DevicesHost
	hubProtocol := provider.ConfigData.AppiumConfig.SeleniumHubProtocolType
	deviceModel := deviceConfig.DeviceModel
	remoteControl := provider.ConfigData.EnvConfig.RemoteControl
	minicapFPS := deviceConfig.MinicapFPS
	minicapHalfResolution := deviceConfig.MinicapHalfResolution
	screenSize := deviceConfig.ScreenSize
	screenSizeValues := strings.Split(screenSize, "x")
	useMinicap := deviceConfig.UseMinicap

	// Create the docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + udid)
		return
	}

	environmentVars := []string{"ON_GRID=" + provider.ConfigData.EnvConfig.ConnectSeleniumGrid,
		"APPIUM_PORT=" + appiumPort,
		"DEVICE_UDID=" + udid,
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
		"DEVICE_OS=android"}

	if useMinicap != "" {
		environmentVars = append(environmentVars, "USE_MINICAP="+useMinicap)
	}

	// Create the container config
	config := &container.Config{
		Image: "android-appium",
		ExposedPorts: nat.PortSet{
			nat.Port("4723"):              struct{}{},
			nat.Port(containerServerPort): struct{}{},
		},
		Env: environmentVars,
	}

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: provider.ProjectDir + "/logs/container_" + deviceName + "-" + udid,
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
			Source:      "/dev/device_android_" + udid,
			Target:      "/dev/device_android_" + udid,
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
				PathOnHost:        "/dev/device_android_" + udid,
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
	err = os.MkdirAll("./logs/container_"+deviceName+"-"+udid, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create logs folder when attempting to create a container for device with udid: " + udid + ". Error: " + err.Error())
		return
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, config, host_config, nil, nil, "androidDevice_"+udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create a container for device with udid: " + udid + ". Error: " + err.Error())
		return
	}

	// Start the container
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not start container for device with udid: " + udid + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "android_container_create",
	}).Info("Successfully created a container for Android device with udid: " + udid)
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

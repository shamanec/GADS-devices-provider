package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var project_dir, _ = os.Getwd()
var on_grid = GetEnvValue("connect_selenium_grid")

type CreateDeviceContainerRequest struct {
	DeviceType string `json:"device_type"`
	Udid       string `json:"udid"`
}

type RemoveDeviceContainerData struct {
	Udid string `json:"udid"`
}

// @Summary      Restart container
// @Description  Restarts container by provided container ID
// @Tags         containers
// @Produce      json
// @Param        container_id path string true "Container ID"
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /containers/{container_id}/restart [post]
func RestartContainer(w http.ResponseWriter, r *http.Request) {
	// Get the request path vars
	vars := mux.Vars(r)
	container_id := vars["container_id"]

	log.WithFields(log.Fields{
		"event": "docker_container_restart",
	}).Info("Attempting to restart container with ID: " + container_id)

	// Call the internal function to restart the container
	err := RestartContainerInternal(container_id)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_restart",
		}).Error("Restarting container with ID: " + container_id + " failed.")
		JSONError(w, "docker_container_restart", "Could not restart container with ID: "+container_id, 500)
		return
	}

	SimpleJSONResponse(w, "Successfully attempted to restart container with ID: "+container_id, 200)
}

// @Summary      Remove container for device
// @Description  Removes a running container for a disconnected registered device by device UDID
// @Tags         device-containers
// @Param        config body RemoveDeviceContainerData true "Remove container for device"
// @Success      202
// @Router       /device-containers/remove [post]
func RemoveDeviceContainer(w http.ResponseWriter, r *http.Request) {
	var data RemoveDeviceContainerData

	// Read the request data
	err := UnmarshalReader(r.Body, &data)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_container_remove",
		}).Error("Could not unmarshal request body when removing container: " + err.Error())
		return
	}

	// Check if container exists and get the container ID
	container_exists, container_id, _ := checkContainerExistsByName(data.Udid)

	if container_exists {
		// Start removing the container in a goroutine and immediately reply with Accepted
		go removeContainerByID(container_id)
	}
	w.WriteHeader(http.StatusAccepted)
}

// @Summary      Get container logs
// @Description  Get logs of container by provided container ID
// @Tags         containers
// @Produce      json
// @Param        container_id path string true "Container ID"
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /containers/{container_id}/logs [get]
func GetContainerLogs(w http.ResponseWriter, r *http.Request) {
	// Get the request path vars
	vars := mux.Vars(r)
	container_id := vars["container_id"]

	// Create the context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_logs",
		}).Error("Could not create docker client while attempting to get logs for container with ID: " + container_id + ". Error: " + err.Error())
		JSONError(w, "get_container_logs", "Could not get logs for container with ID: "+container_id, 500)
		return
	}

	// Create the options for the container logs function
	options := types.ContainerLogsOptions{ShowStdout: true}

	// Get the container logs
	out, err := cli.ContainerLogs(ctx, container_id, options)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_logs",
		}).Error("Could not get logs for container with ID: " + container_id + ". Error: " + err.Error())
		JSONError(w, "get_container_logs", "Could not get logs for container with ID: "+container_id, 500)
		return
	}

	// Get the ReadCloser of the logs into a buffer
	// And convert it to string
	buf := new(bytes.Buffer)
	buf.ReadFrom(out)
	newStr := buf.String()

	// If there are any logs - reply with them
	// Or reply with a generic string
	if newStr != "" {
		SimpleJSONResponse(w, newStr, 200)
	} else {
		SimpleJSONResponse(w, "There are no existing logs for this container.", 200)
	}
}

// @Summary      Create container for device
// @Description  Creates a container for a connected registered device
// @Tags         device-containers
// @Param        config body CreateDeviceContainerRequest true "Create container for device"
// @Success      202
// @Router       /device-containers/create [post]
func CreateDeviceContainer(w http.ResponseWriter, r *http.Request) {
	var data CreateDeviceContainerRequest

	// Read the request data
	err := UnmarshalReader(r.Body, &data)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_container_create",
		}).Error("Could not unmarshal request body when creating container: " + err.Error())
		return
	}

	os_type := data.DeviceType
	device_udid := data.Udid

	// Start creating a device container in a goroutine and immediately reply with Accepted
	go func() {
		// Check if container exists and get the container ID and current status
		container_exists, container_id, _ := checkContainerExistsByName(device_udid)

		// Create a container if no container exists for this device
		// or restart a non-running container that already exists for this device
		// this is useful after restart and reconnecting devices
		if !container_exists {
			if os_type == "android" {
				go CreateAndroidContainer(device_udid)
			} else if os_type == "ios" {
				go CreateIOSContainer(device_udid)
			}
			return
		} else {
			go RestartContainerInternal(container_id)
			return
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}

// @Summary      Remove container
// @Description  Removes container by provided container ID
// @Tags         containers
// @Produce      json
// @Param        container_id path string true "Container ID"
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /containers/{container_id}/remove [post]
func RemoveContainer(w http.ResponseWriter, r *http.Request) {
	// Get the request path vars
	vars := mux.Vars(r)
	key := vars["container_id"]

	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Attempting to remove container with ID: " + key)

	// Create a new context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not create docker client while attempting to remove container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "docker_container_remove", "Could not remove container with ID: "+key, 500)
		return
	}

	// Try to stop the container
	if err := cli.ContainerStop(ctx, key, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not stop container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "docker_container_remove", "Could not remove container with ID: "+key, 500)
		return
	}

	// Try to remove the stopped container
	if err := cli.ContainerRemove(ctx, key, types.ContainerRemoveOptions{}); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + key + ". Error: " + err.Error())
		JSONError(w, "docker_container_remove", "Could not remove container with ID: "+key, 500)
		return
	}

	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Successfully removed container with ID: " + key)
	SimpleJSONResponse(w, "Successfully removed container with ID: "+key, 200)
}

// @Summary      Refresh the device-containers data
// @Description  Refreshes the device-containers data by returning an updated HTML table
// @Produce      html
// @Success      200
// @Failure      500
// @Router       /refresh-device-containers [post]
func RefreshDeviceContainers(w http.ResponseWriter, r *http.Request) {
	// Generate the data for each device container row in a slice of ContainerRow
	rows, err := deviceContainerRows()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Make functions available in html template
	funcMap := template.FuncMap{
		// The name "title" is what the function will be called in the template text.
		"contains": strings.Contains,
	}

	// Parse the template and return response with the container table rows
	// This will generate only the device table, not the whole page
	var tmpl = template.Must(template.New("device_containers_table").Funcs(funcMap).ParseFiles("static/device_containers_table.html"))

	// Reply with the new table
	if err := tmpl.ExecuteTemplate(w, "device_containers_table", rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Create an iOS container for a specific device(by UDID) using data from config.json so if device is not registered there it will not attempt to create a container for it
func CreateIOSContainer(device_udid string) {
	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Attempting to create a container for iOS device with udid: " + device_udid)

	time.Sleep(2 * time.Second)

	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not unmarshal config.json file when trying to create a container for device with udid: " + device_udid)
		return
	}

	// Check if device is registered in config data
	var deviceConfig DeviceConfig
	for _, v := range configData.DeviceConfig {
		if v.DeviceUDID == device_udid {
			deviceConfig = v
		}
	}

	// Stop execution if device not in config data
	if deviceConfig == (DeviceConfig{}) {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Device with UDID:" + device_udid + " is not registered in the 'config.json' file. No container will be created.")
		return
	}

	// Get the device specific config data
	appium_port := deviceConfig.AppiumPort
	device_name := deviceConfig.DeviceName
	device_os_version := deviceConfig.DeviceOSVersion
	wda_mjpeg_port := deviceConfig.StreamPort
	wda_port := deviceConfig.WDAPort
	wda_bundle_id := configData.AppiumConfig.WDABundleID
	selenium_hub_port := configData.AppiumConfig.SeleniumHubPort
	selenium_hub_host := configData.AppiumConfig.SeleniumHubHost
	devices_host := configData.AppiumConfig.DevicesHost
	hub_protocol := configData.AppiumConfig.SeleniumHubProtocolType
	containerized_usbmuxd := configData.EnvConfig.ContainerizedUsbmuxd
	screen_size := deviceConfig.ScreenSize
	container_server_port := deviceConfig.ContainerServerPort
	device_model := deviceConfig.DeviceModel

	// Create docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + device_udid)
		return
	}

	// Create the container config
	config := &container.Config{
		Image: "ios-appium",
		ExposedPorts: nat.PortSet{
			nat.Port("4723"):                struct{}{},
			nat.Port(wda_port):              struct{}{},
			nat.Port(wda_mjpeg_port):        struct{}{},
			nat.Port(container_server_port): struct{}{},
		},
		Env: []string{"ON_GRID=" + on_grid,
			"APPIUM_PORT=" + appium_port,
			"DEVICE_UDID=" + device_udid,
			"WDA_PORT=" + wda_port,
			"MJPEG_PORT=" + wda_mjpeg_port,
			"DEVICE_OS_VERSION=" + device_os_version,
			"DEVICE_NAME=" + device_name,
			"WDA_BUNDLEID=" + wda_bundle_id,
			"SUPERVISION_PASSWORD=" + GetEnvValue("supervision_password"),
			"SELENIUM_HUB_PORT=" + selenium_hub_port,
			"SELENIUM_HUB_HOST=" + selenium_hub_host,
			"DEVICES_HOST=" + devices_host,
			"HUB_PROTOCOL=" + hub_protocol,
			"CONTAINERIZED_USBMUXD=" + containerized_usbmuxd,
			"SCREEN_SIZE=" + screen_size,
			"CONTAINER_SERVER_PORT=" + container_server_port,
			"DEVICE_MODEL=" + device_model,
			"DEVICE_OS=ios"},
	}

	var mounts []mount.Mount
	var resources container.Resources

	if containerized_usbmuxd == "false" {
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
					PathOnHost:        "/dev/device_" + device_udid,
					PathInContainer:   "/dev/bus/usb/003/011",
					CgroupPermissions: "rwm",
				},
			},
		}
	}

	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: project_dir + "/logs/container_" + device_name + "-" + device_udid,
		Target: "/opt/logs",
	})

	mounts = append(mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: project_dir + "/apps",
		Target: "/opt/ipa",
	})

	// Create the host config
	host_config := &container.HostConfig{
		Privileged:    true,
		RestartPolicy: container.RestartPolicy{Name: "on-failure", MaximumRetryCount: 3},
		PortBindings: nat.PortMap{
			nat.Port("4723"): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: appium_port,
				},
			},
			nat.Port(wda_port): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: wda_port,
				},
			},
			nat.Port(wda_mjpeg_port): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: wda_mjpeg_port,
				},
			},
			nat.Port(container_server_port): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: container_server_port,
				},
			},
		},
		Mounts:    mounts,
		Resources: resources,
	}

	// Create a folder for logging for the container
	err = os.MkdirAll("./logs/container_"+device_name+"-"+device_udid, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create logs folder when attempting to create a container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, config, host_config, nil, nil, "iosDevice_"+device_udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not create a container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	// Start the container
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "ios_container_create",
		}).Error("Could not start container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "ios_container_create",
	}).Info("Successfully created a container for iOS device with udid: " + device_udid)
}

// Create an Android container for a specific device(by UDID) using data from config.json so if device is not registered there it will not attempt to create a container for it
// If container already exists for this device it will do nothing
func CreateAndroidContainer(device_udid string) {
	log.WithFields(log.Fields{
		"event": "android_container_create",
	}).Info("Attempting to create a container for Android device with udid: " + device_udid)

	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not unmarshal config.json file when trying to create a container for device with udid: " + device_udid)
		return
	}

	// Check if device is registered in config data
	var deviceConfig DeviceConfig
	for _, v := range configData.DeviceConfig {
		if v.DeviceUDID == device_udid {
			deviceConfig = v
		}
	}

	// Stop execution if device not in config data
	if deviceConfig == (DeviceConfig{}) {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Device with UDID:" + device_udid + " is not registered in the 'config.json' file. No container will be created.")
		return
	}

	// Get the device config data
	appium_port := deviceConfig.AppiumPort
	device_name := deviceConfig.DeviceName
	device_os_version := deviceConfig.DeviceOSVersion
	selenium_hub_port := configData.AppiumConfig.SeleniumHubPort
	selenium_hub_host := configData.AppiumConfig.SeleniumHubHost
	devices_host := configData.AppiumConfig.DevicesHost
	hub_protocol := configData.AppiumConfig.SeleniumHubProtocolType
	container_server_port := deviceConfig.ContainerServerPort
	device_model := deviceConfig.DeviceModel
	remote_control := configData.EnvConfig.RemoteControl

	// Create the docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create docker client when attempting to create a container for device with udid: " + device_udid)
		return
	}

	// Create the container config
	config := &container.Config{
		Image: "android-appium",
		ExposedPorts: nat.PortSet{
			nat.Port("4723"):                struct{}{},
			nat.Port(container_server_port): struct{}{},
		},
		Env: []string{"ON_GRID=" + on_grid,
			"APPIUM_PORT=" + appium_port,
			"DEVICE_UDID=" + device_udid,
			"DEVICE_OS_VERSION=" + device_os_version,
			"DEVICE_NAME=" + device_name,
			"SELENIUM_HUB_PORT=" + selenium_hub_port,
			"SELENIUM_HUB_HOST=" + selenium_hub_host,
			"DEVICES_HOST=" + devices_host,
			"HUB_PROTOCOL=" + hub_protocol,
			"CONTAINER_SERVER_PORT=" + container_server_port,
			"DEVICE_MODEL=" + device_model,
			"REMOTE_CONTROL=" + remote_control,
			"DEVICE_OS=android"},
	}

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: project_dir + "/logs/container_" + device_name + "-" + device_udid,
			Target: "/opt/logs",
		},
		{
			Type:   mount.TypeBind,
			Source: project_dir + "/apps",
			Target: "/opt/apk",
		},
		{
			Type:   mount.TypeBind,
			Source: "/home/shamanec/.android",
			Target: "/root/.android",
		},
		{
			Type:        mount.TypeBind,
			Source:      "/dev/device_" + device_udid,
			Target:      "/dev/device_" + device_udid,
			BindOptions: &mount.BindOptions{Propagation: "shared"},
		},
	}

	if remote_control == "true" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: project_dir + "/minicap",
			Target: "/root/minicap",
		})
	}

	resources := container.Resources{
		Devices: []container.DeviceMapping{
			{
				PathOnHost:        "/dev/device_" + device_udid,
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
					HostPort: appium_port,
				},
			},
			nat.Port(container_server_port): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: container_server_port,
				},
			},
		},
		Mounts:    mounts,
		Resources: resources,
	}

	// Create a folder for logging for the container
	err = os.MkdirAll("./logs/container_"+device_name+"-"+device_udid, os.ModePerm)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create logs folder when attempting to create a container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, config, host_config, nil, nil, "androidDevice_"+device_udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not create a container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	// Start the container
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "android_container_create",
		}).Error("Could not start container for device with udid: " + device_udid + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "android_container_create",
	}).Info("Successfully created a container for Android device with udid: " + device_udid)
}

// Check if container exists by name and also return container_id
func checkContainerExistsByName(device_udid string) (bool, string, string) {
	// Get all the containers
	containers, _ := getContainersList()
	container_exists := false
	container_id := ""
	container_status := ""

	// Loop through the available containers
	// If a container which name contains the device udid exists
	// return true and also return the container ID and status
	for _, container := range containers {
		containerName := strings.Replace(container.Names[0], "/", "", -1)
		if strings.Contains(containerName, device_udid) {
			container_exists = true
			container_id = container.ID
			container_status = container.Status
		}
	}
	return container_exists, container_id, container_status
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
func RestartContainerInternal(container_id string) error {
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
func removeContainerByID(container_id string) {
	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Attempting to remove container with ID: " + container_id)

	// Create a new context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not create docker client while attempting to remove container with ID: " + container_id + ". Error: " + err.Error())
		return
	}

	// Stop the container by the provided container ID
	if err := cli.ContainerStop(ctx, container_id, nil); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + container_id + ". Error: " + err.Error())
		return
	}

	// Remove the stopped container
	if err := cli.ContainerRemove(ctx, container_id, types.ContainerRemoveOptions{}); err != nil {
		log.WithFields(log.Fields{
			"event": "docker_container_remove",
		}).Error("Could not remove container with ID: " + container_id + ". Error: " + err.Error())
		return
	}

	log.WithFields(log.Fields{
		"event": "docker_container_remove",
	}).Info("Successfully removed container with ID: " + container_id)
}

// @Summary      Refresh the device-containers data
// @Description  Refreshes the device-containers data by returning an updated HTML table
// @Produce      html
// @Success      200
// @Failure      500
// @Router       /device-containers [post]
func GetDeviceContainers(w http.ResponseWriter, r *http.Request) {
	deviceContainers, err := deviceContainerRows()
	if err != nil {
		fmt.Fprintf(w, "Could not get device containers")
		return
	}

	json, err := ConvertToJSONString(deviceContainers)

	fmt.Fprintf(w, json)
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
func deviceContainerRows() ([]DeviceContainerInfo, error) {
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

package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
	"github.com/shamanec/GADS-devices-provider/device"
	"github.com/shamanec/GADS-devices-provider/docker"
	"github.com/shamanec/GADS-devices-provider/provider"
	"github.com/shamanec/GADS-devices-provider/util"

	log "github.com/sirupsen/logrus"
)

type JsonErrorResponse struct {
	EventName    string `json:"event"`
	ErrorMessage string `json:"error_message"`
}

type JsonResponse struct {
	Message string `json:"message"`
}

type CreateDeviceContainerRequest struct {
	DeviceType string `json:"device_type"`
	Udid       string `json:"udid"`
}

type RemoveDeviceContainerData struct {
	Udid string `json:"udid"`
}

// Write to a ResponseWriter an event and message with a response code
func JSONError(w http.ResponseWriter, event string, error_string string, code int) {
	var errorMessage = JsonErrorResponse{
		EventName:    event,
		ErrorMessage: error_string}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorMessage)
}

// Write to a ResponseWriter an event and message with a response code
func SimpleJSONResponse(w http.ResponseWriter, response_message string, code int) {
	var message = JsonResponse{
		Message: response_message,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(message)
}

func GetAvailableDevicesInfo(w http.ResponseWriter, r *http.Request) {
	runningContainerNames, err := device.RunningDeviceContainerNames()
	if err != nil {
		JSONError(w, "get_available_devices", "Could not get available devices", 500)
		return
	}

	devicesInfo, err := device.AvailableDevicesInfo(runningContainerNames)
	if err != nil {
		JSONError(w, "get_available_devices", "Could not get available devices", 500)
		return
	}

	var info = device.DevicesInfo{
		DevicesInfo: devicesInfo,
	}

	responseData, err := util.ConvertToJSONString(info)
	if err != nil {
		JSONError(w, "get_available_devices", "Could not get available devices", 500)
		return
	}
	fmt.Fprintf(w, responseData)
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
	err := docker.RestartContainer(container_id)
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
	err := util.UnmarshalReader(r.Body, &data)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "device_container_remove",
		}).Error("Could not unmarshal request body when removing container: " + err.Error())
		return
	}

	// Check if container exists and get the container ID
	container_exists, container_id, _ := docker.CheckContainerExistsByName(data.Udid)

	if container_exists {
		// Start removing the container in a goroutine and immediately reply with Accepted
		go docker.RemoveContainerByID(container_id)
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
	err := util.UnmarshalReader(r.Body, &data)
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
		container_exists, container_id, status := docker.CheckContainerExistsByName(device_udid)

		// Create a container if no container exists for this device
		// or restart a non-running container that already exists for this device
		// this is useful after restart and reconnecting devices
		if !container_exists {
			if os_type == "android" {
				go docker.CreateAndroidContainer(device_udid)
			} else if os_type == "ios" {
				go docker.CreateIOSContainer(device_udid)
			}
			return
		} else if !strings.Contains(status, "Up") {
			go docker.RestartContainer(container_id)
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
	rows, err := docker.DeviceContainerRows()
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

// @Summary      Creates the udev rules for device symlink and container creation
// @Description  Creates 90-device.rules file to be used by udev
// @Tags         configuration
// @Produce      json
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /configuration/create-udev-rules [post]
func CreateUdevRules(w http.ResponseWriter, r *http.Request) {
	// Open /lib/systemd/system/systemd-udevd.service
	// Add IPAddressAllow=127.0.0.1 at the bottom
	// This is to allow curl calls from the udev rules to the GADS server
	err := provider.CreateUdevRules()
	if err != nil {
		JSONError(w, "create_udev_rules", "Could not create udev rules file", 500)
		return
	}

	SimpleJSONResponse(w, "Successfully created 90-device.rules file in project dir", 200)
}

// @Summary      Refresh the device-containers data
// @Description  Refreshes the device-containers data by returning an updated HTML table
// @Produce      html
// @Success      200
// @Failure      500
// @Router       /device-containers [post]
func GetDeviceContainers(w http.ResponseWriter, r *http.Request) {
	deviceContainers, err := docker.DeviceContainerRows()
	if err != nil {
		fmt.Fprintf(w, "Could not get device containers")
		return
	}

	json, err := util.ConvertToJSONString(deviceContainers)

	fmt.Fprintf(w, json)
}

// @Summary      Get provider logs
// @Description  Gets provider logs as plain text response
// @Tags         provider-logs
// @Produces	 text
// @Success      200
// @Failure      200
// @Router       /provider-logs [get]
func GetLogs(w http.ResponseWriter, r *http.Request) {
	// Create the command string to read the last 1000 lines of provider.log
	commandString := "tail -n 1000 ./logs/provider.log"

	// Create the command
	cmd := exec.Command("bash", "-c", commandString)

	// Create a buffer for the output
	var out bytes.Buffer

	// Pipe the Stdout of the command to the buffer pointer
	cmd.Stdout = &out

	// Execute the command
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_project_logs",
		}).Warning("Attempted to get project logs but no logs available.")

		// Reply with generic message on error
		fmt.Fprintf(w, "No logs available.")
		return
	}

	// Reply with the read logs lines
	fmt.Fprintf(w, out.String())
}

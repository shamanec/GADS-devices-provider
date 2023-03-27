package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
	"github.com/shamanec/GADS-devices-provider/device"
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
func SimpleJSONResponse(w http.ResponseWriter, responseMessage string, code int) {
	var message = JsonResponse{
		Message: responseMessage,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(message)
}

func GetProviderDevices(w http.ResponseWriter, r *http.Request) {
	responseData, err := util.ConvertToJSONString(device.GetConfigDevices())
	if err != nil {
		JSONError(w, "get_available_devices", "Could not get available devices", 500)
		return
	}
	fmt.Fprintf(w, responseData)
}

func DeviceHealth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	udid := vars["udid"]

	bool, err := device.GetDeviceHealth(udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "check_device_health",
		}).Error("Could not check device health, err: " + err.Error())
		JSONError(w, "check_device_health", "Could not check device health", 500)
		return
	}

	if bool {
		w.WriteHeader(200)
		return
	}

	w.WriteHeader(500)
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
	containerID := vars["container_id"]

	// Create the context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_logs",
		}).Error("Could not create docker client while attempting to get logs for container with ID: " + containerID + ". Error: " + err.Error())
		JSONError(w, "get_container_logs", "Could not get logs for container with ID: "+containerID, 500)
		return
	}

	// Create the options for the container logs function
	options := types.ContainerLogsOptions{ShowStdout: true}

	// Get the container logs
	out, err := cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_logs",
		}).Error("Could not get logs for container with ID: " + containerID + ". Error: " + err.Error())
		JSONError(w, "get_container_logs", "Could not get logs for container with ID: "+containerID, 500)
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

// @Summary      Creates the udev rules for device symlink and container creation
// @Description  Creates 90-device.rules file to be used by udev
// @Tags         device
// @Produce      json
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /device/create-udev-rules [post]
func CreateUdevRules(w http.ResponseWriter, r *http.Request) {
	err := device.CreateUdevRules()
	if err != nil {
		JSONError(w, "create_udev_rules", "Could not create udev rules file", 500)
		return
	}

	SimpleJSONResponse(w, "Successfully created 90-device.rules file in project dir", 200)
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

package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
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

func GetProviderDevices(c *gin.Context) {
	responseData, err := util.ConvertToJSONString(device.GetConfigDevices())
	if err != nil {
		JSONError(c.Writer, "get_available_devices", "Could not get available devices", 500)
		return
	}
	fmt.Fprintf(c.Writer, responseData)
}

func DeviceHealth(c *gin.Context) {
	udid := c.Param("udid")
	bool, err := device.GetDeviceHealth(udid)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "check_device_health",
		}).Error("Could not check device health, err: " + err.Error())
		JSONError(c.Writer, "check_device_health", "Could not check device health", 500)
		return
	}

	if bool {
		c.Writer.WriteHeader(200)
		return
	}

	c.Writer.WriteHeader(500)
}

func GetContainerLogs(c *gin.Context) {
	containerID := c.Param("containerID")

	// Create the context and Docker client
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_container_logs",
		}).Error("Could not create docker client while attempting to get logs for container with ID: " + containerID + ". Error: " + err.Error())
		JSONError(c.Writer, "get_container_logs", "Could not get logs for container with ID: "+containerID, 500)
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
		JSONError(c.Writer, "get_container_logs", "Could not get logs for container with ID: "+containerID, 500)
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
		SimpleJSONResponse(c.Writer, newStr, 200)
	} else {
		SimpleJSONResponse(c.Writer, "There are no existing logs for this container.", 200)
	}
}

func CreateUdevRules(c *gin.Context) {
	err := device.CreateUdevRules()
	if err != nil {
		JSONError(c.Writer, "create_udev_rules", "Could not create udev rules file", 500)
		return
	}

	SimpleJSONResponse(c.Writer, "Successfully created 90-device.rules file in project dir", 200)
}

func GetLogs(c *gin.Context) {
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
		fmt.Fprintf(c.Writer, "No logs available.")
		return
	}

	// Reply with the read logs lines
	fmt.Fprintf(c.Writer, out.String())
}

//=======================================
// TAP

type actionData struct {
	X         float64 `json:"x,omitempty"`
	Y         float64 `json:"y,omitempty"`
	EndX      float64 `json:"endX,omitempty"`
	EndY      float64 `json:"endY,omitempty`
	SessionID string  `json:"sessionID,omitempty"`
}

func DeviceTap(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	var requestURL string

	if device.OS == "android" {
		requestURL = "http://localhost:" + device.AppiumPort + "/session/" + requestBody.SessionID + "/actions"
	}

	if device.OS == "ios" {
		requestURL = "http://localhost:" + device.WDAPort + "/session/" + requestBody.SessionID + "/actions"
	}

	action := devicePointerActions{
		[]devicePointerAction{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: deviceActionParameters{
					PointerType: "touch",
				},
				Actions: []deviceAction{
					{
						Type:     "pointerMove",
						Duration: 0,
						X:        requestBody.X,
						Y:        requestBody.Y,
					},
					{
						Type:   "pointerDown",
						Button: 0,
					},
					{
						Type:     "pause",
						Duration: 200,
					},
					{
						Type:     "pointerUp",
						Duration: 0,
					},
				},
			},
		},
	}

	fmt.Println("REQUEST URL: ")
	fmt.Println(requestURL)

	actionJSON, err := util.ConvertToJSONString(action)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println(actionJSON)

	// Create a new HTTP client
	client := http.DefaultClient

	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewBuffer([]byte(actionJSON)))
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	res, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer res.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println("OPF")
	fmt.Println(string(body))
	fmt.Println(res.StatusCode)

	c.Writer.WriteHeader(res.StatusCode)
	fmt.Fprintf(c.Writer, string(body))
}

func DeviceSwipe(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	var requestURL string

	if device.OS == "android" {
		requestURL = "http://localhost:" + device.AppiumPort + "/session/" + requestBody.SessionID + "/actions"
	}

	if device.OS == "ios" {
		requestURL = "http://localhost:" + device.WDAPort + "/session/" + requestBody.SessionID + "/actions"
	}

	action := devicePointerActions{
		[]devicePointerAction{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: deviceActionParameters{
					PointerType: "touch",
				},
				Actions: []deviceAction{
					{
						Type:     "pointerMove",
						Duration: 0,
						X:        requestBody.X,
						Y:        requestBody.Y,
					},
					{
						Type:   "pointerDown",
						Button: 0,
					},
					{
						Type:     "pointerMove",
						Duration: 750,
						Origin:   "viewport",
						X:        requestBody.EndX,
						Y:        requestBody.EndY,
					},
					{
						Type:     "pointerUp",
						Duration: 0,
					},
				},
			},
		},
	}

	fmt.Println("Request URL is: ")
	fmt.Println(requestURL)

	actionJSON, err := util.ConvertToJSONString(action)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	// Create a new HTTP client
	client := http.DefaultClient

	req, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewBuffer([]byte(actionJSON)))
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	res, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer res.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	c.Writer.WriteHeader(res.StatusCode)
	fmt.Fprintf(c.Writer, string(body))
}

type deviceAction struct {
	Type     string  `json:"type"`
	Duration int     `json:"duration"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Button   int     `json:"button"`
	Origin   string  `json:"origin,omitempty"`
}

type deviceActionParameters struct {
	PointerType string `json:"pointerType"`
}

type devicePointerAction struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Parameters deviceActionParameters `json:"parameters"`
	Actions    []deviceAction         `json:"actions"`
}

type devicePointerActions struct {
	Actions []devicePointerAction `json:"actions"`
}

// =======================================
func DeviceHome(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	host := "http://localhost:"

	var deviceHomeURL string
	if device.OS == "android" {
		var requestBody actionData
		if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
			http.Error(c.Writer, err.Error(), http.StatusBadRequest)
			return
		}

		deviceHomeURL = host + device.AppiumPort + "/session/" + requestBody.SessionID + "/appium/device/press_keycode"
	}

	if device.OS == "ios" {
		deviceHomeURL = host + device.WDAPort + "/wda/homescreen"
	}

	// Create a new HTTP client
	client := http.DefaultClient

	homeRequestBody := ""
	if device.OS == "android" {
		homeRequestBody = `{"keycode": 3}`
	}

	fmt.Println("Will request " + deviceHomeURL)
	req, err := http.NewRequest(http.MethodPost, deviceHomeURL, bytes.NewBuffer([]byte(homeRequestBody)))
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	homeResponse, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer homeResponse.Body.Close()

	// Read the response body
	homeResponseBody, err := ioutil.ReadAll(homeResponse.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	c.Writer.WriteHeader(homeResponse.StatusCode)
	fmt.Fprintf(c.Writer, string(homeResponseBody))
}

// =======================================
func DeviceLock(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	host := "http://localhost:"
	var deviceHomeURL string
	if device.OS == "android" {
		deviceHomeURL = host + device.AppiumPort + "/session/" + requestBody.SessionID + "/appium/device/lock"
	}

	if device.OS == "ios" {
		deviceHomeURL = host + device.WDAPort + "/session/" + requestBody.SessionID + "/wda/lock"
	}

	// Create a new HTTP client
	client := http.DefaultClient

	fmt.Println("Will request " + deviceHomeURL)
	req, err := http.NewRequest(http.MethodPost, deviceHomeURL, nil)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	lockResponse, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := ioutil.ReadAll(lockResponse.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	c.Writer.WriteHeader(lockResponse.StatusCode)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

func DeviceUnlock(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	host := "http://localhost:"
	var deviceHomeURL string
	if device.OS == "android" {
		deviceHomeURL = host + device.AppiumPort + "/session/" + requestBody.SessionID + "/appium/device/unlock"
	}

	if device.OS == "ios" {
		deviceHomeURL = host + device.WDAPort + "/session/" + requestBody.SessionID + "/wda/unlock"
	}

	// Create a new HTTP client
	client := http.DefaultClient

	fmt.Println("Will request " + deviceHomeURL)
	req, err := http.NewRequest(http.MethodPost, deviceHomeURL, nil)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	lockResponse, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := ioutil.ReadAll(lockResponse.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	c.Writer.WriteHeader(lockResponse.StatusCode)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

// ========================
func DeviceScreenshot(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	var requestBody actionData
	if err := json.NewDecoder(c.Request.Body).Decode(&requestBody); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	host := "http://localhost:"
	var deviceHomeURL string
	if device.OS == "android" {
		deviceHomeURL = host + device.AppiumPort + "/session/" + requestBody.SessionID + "/screenshot"
	}

	if device.OS == "ios" {
		deviceHomeURL = host + device.WDAPort + "/session/" + requestBody.SessionID + "/screenshot"
	}

	// Create a new HTTP client
	client := http.DefaultClient

	fmt.Println("Will request " + deviceHomeURL)
	req, err := http.NewRequest(http.MethodGet, deviceHomeURL, nil)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	// Send the request
	lockResponse, err := client.Do(req)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	defer lockResponse.Body.Close()

	// Read the response body
	lockResponseBody, err := ioutil.ReadAll(lockResponse.Body)
	if err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	c.Writer.WriteHeader(lockResponse.StatusCode)
	fmt.Fprintf(c.Writer, string(lockResponseBody))
}

const mjpegFrameFooter = "\r\n\r\n"
const mjpegFrameHeader = "--BoundaryString\r\nContent-type: image/jpg\r\nContent-Length: %d\r\n\r\n"

func DeviceStream(c *gin.Context) {
	udid := c.Param("udid")
	device := device.GetDeviceByUDID(udid)

	deviceStreamURL := ""
	if device.OS == "android" {
		deviceStreamURL = "http://localhost:" + device.ContainerServerPort + "/stream"
	}

	if device.OS == "ios" {
		deviceStreamURL = "http://localhost:" + device.StreamPort
	}
	client := http.Client{}

	// Replace this URL with the actual endpoint URL serving the JPEG stream
	resp, err := client.Get(deviceStreamURL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error connecting to the stream")
		return
	}
	defer resp.Body.Close()

	c.Status(resp.StatusCode)
	copyHeaders(c.Writer.Header(), resp.Header)
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		return
	}
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

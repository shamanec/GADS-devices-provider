package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

type ConfigJsonData struct {
	AppiumConfig AppiumConfig   `json:"appium-config"`
	EnvConfig    EnvConfig      `json:"env-config"`
	DeviceConfig []DeviceConfig `json:"devices-config"`
}

type AppiumConfig struct {
	DevicesHost             string `json:"devices_host"`
	SeleniumHubHost         string `json:"selenium_hub_host"`
	SeleniumHubPort         string `json:"selenium_hub_port"`
	SeleniumHubProtocolType string `json:"selenium_hub_protocol_type"`
	WDABundleID             string `json:"wda_bundle_id"`
}

type EnvConfig struct {
	SudoPassword         string `json:"sudo_password"`
	ConnectSeleniumGrid  bool   `json:"connect_selenium_grid"`
	SupervisionPassword  string `json:"supervision_password"`
	ContainerizedUsbmuxd string `json:"containerized_usbmuxd"`
}

type DeviceConfig struct {
	OS                  string `json:"os"`
	AppiumPort          int    `json:"appium_port"`
	DeviceName          string `json:"device_name"`
	DeviceOSVersion     string `json:"device_os_version"`
	DeviceUDID          string `json:"device_udid"`
	WDAMjpegPort        int    `json:"wda_mjpeg_port"`
	WDAPort             int    `json:"wda_port"`
	ScreenSize          string `json:"screen_size"`
	StreamPort          int    `json:"stream_port"`
	ContainerServerPort int    `json:"container_server_port"`
	DeviceModel         string `json:"device_model"`
	DeviceImage         string `json:"device_image"`
	DeviceHost          string `json:"device_host"`
}

type JsonErrorResponse struct {
	EventName    string `json:"event"`
	ErrorMessage string `json:"error_message"`
}

type JsonResponse struct {
	Message string `json:"message"`
}

// Get a ConfigJsonData pointer with the current configuration from config.json
func GetConfigJsonData() (*ConfigJsonData, error) {
	var data ConfigJsonData
	jsonFile, err := os.Open("./configs/config.json")
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	bs, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bs, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

// Convert interface into JSON string
func ConvertToJSONString(data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return string(b)
}

// Prettify JSON with indentation and stuff
func PrettifyJSON(data string) string {
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, []byte(data), "", "  ")
	return prettyJSON.String()
}

// Unmarshal request body into a struct
func UnmarshalRequestBody(body io.ReadCloser, v interface{}) error {
	reqBody, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(reqBody, v)
	if err != nil {
		return err
	}

	return nil
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

// Delete file using shell, needed when deleting from a protected folder. Needs `sudo_password` set in configs/config.json
func DeleteFileShell(filePath string, sudoPassword string) error {
	commandString := "echo '" + sudoPassword + "' | sudo -S rm " + filePath
	cmd := exec.Command("bash", "-c", commandString)
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "delete_file_shell",
		}).Error("Could not delete file:" + filePath + " with shell. Error:" + err.Error())
		return errors.New("Could not delete file: " + filePath + "with shell")
	}
	return nil
}

// Copy file using shell, needed when copying to a protected folder. Needs `sudo_password` set in configs/config.json
func CopyFileShell(currentFilePath string, newFilePath string, sudoPassword string) error {
	commandString := "echo '" + sudoPassword + "' | sudo -S cp " + currentFilePath + " " + newFilePath
	cmd := exec.Command("bash", "-c", commandString)
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "delete_file_shell",
		}).Error("Could not copy file:" + currentFilePath + " to:" + newFilePath + ". Error:" + err.Error())
		return errors.New("Could not copy file:" + currentFilePath + " with shell.")
	}
	return nil
}

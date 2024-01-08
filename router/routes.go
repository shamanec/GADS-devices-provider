package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shamanec/GADS-devices-provider/config"
	"github.com/shamanec/GADS-devices-provider/db"
	"github.com/shamanec/GADS-devices-provider/devices"
	"github.com/shamanec/GADS-devices-provider/models"
	"github.com/shamanec/GADS-devices-provider/util"
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

func AppiumReverseProxy(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				fmt.Println("Appium Reverse Proxy panic:", err)
			} else {
				fmt.Println("Appium Reverse Proxy panic:", r)
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal Server Error",
			})
		}
	}()

	udid := c.Param("udid")
	device := devices.DeviceMap[udid]

	target := "http://localhost:" + device.AppiumPort
	path := c.Param("proxyPath")

	proxy := newAppiumProxy(target, path)
	proxy.ServeHTTP(c.Writer, c.Request)
}

func newAppiumProxy(target string, path string) *httputil.ReverseProxy {
	targetURL, _ := url.Parse(target)

	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = targetURL.Path + path
			req.Host = targetURL.Host
			req.Header.Del("Access-Control-Allow-Origin")
		},
	}
}

func UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	allowedExtensions := []string{"ipa", "zip", "apk"}
	// Check file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	isAllowed := false
	for _, allowedExt := range allowedExtensions {
		if ext == "."+allowedExt {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File extension `" + ext + "` not allowed"})
		return
	}

	// Specify the upload directory
	uploadDir := "./apps/"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.Mkdir(uploadDir, os.ModePerm)
	}

	// Save the uploaded file to the specified directory
	dst := uploadDir + file.Filename
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully", "status": "success", "apps": util.GetAllAppFiles()})
}

func GetProviderData(c *gin.Context) {

	connectedDevices := devices.GetConnectedDevicesCommon()
	var providerData models.ProviderData

	deviceData := []*models.Device{}
	for _, device := range devices.DeviceMap {
		deviceData = append(deviceData, device)
	}

	providerData.ConnectedDevices = connectedDevices
	providerData.ProviderData = config.Config.EnvConfig
	providerData.DeviceData = deviceData

	c.JSON(http.StatusOK, providerData)
}

func DeviceInfo(c *gin.Context) {
	udid := c.Param("udid")

	if dev, ok := devices.DeviceMap[udid]; ok {
		devices.UpdateInstalledApps(dev)
		dev.InstallableApps = util.GetAllAppFiles()
		c.JSON(http.StatusOK, dev)
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Did not find device with udid `%s`", udid)})
}

func DevicesInfo(c *gin.Context) {
	deviceList := []*models.Device{}

	for _, device := range devices.DeviceMap {
		deviceList = append(deviceList, device)
	}

	c.JSON(http.StatusOK, deviceList)
}

type ProcessApp struct {
	App string `json:"app"`
}

func UninstallApp(c *gin.Context) {
	udid := c.Param("udid")

	if dev, ok := devices.DeviceMap[udid]; ok {
		payload, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		var payloadJson ProcessApp
		err = json.Unmarshal(payload, &payloadJson)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		if slices.Contains(dev.InstalledApps, payloadJson.App) {
			err = devices.UninstallApp(dev, payloadJson.App)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to uninstall app `%s`", payloadJson.App)})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Successfully uninstalled app `%s`", payloadJson.App)})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("App `%s` is not installed on device", payloadJson.App)})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with udid `%s` does not exist", udid)})
}

func InstallApp(c *gin.Context) {
	udid := c.Param("udid")

	if dev, ok := devices.DeviceMap[udid]; ok {
		payload, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		var payloadJson ProcessApp
		err = json.Unmarshal(payload, &payloadJson)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}
		err = devices.InstallApp(dev, payloadJson.App)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to install app `%s`", payloadJson.App)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Successfully installed app `%s`", payloadJson.App)})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with udid `%s` does not exist", udid)})
}

func ResetDevice(c *gin.Context) {
	udid := c.Param("udid")

	if device, ok := devices.DeviceMap[udid]; ok {
		device.CtxCancel()
		c.JSON(http.StatusOK, gin.H{"message": "Initiate setup reset on device"})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Device with udid `%s` does not exist", udid)})
}

func AddNewDevice(c *gin.Context) {
	var device models.Device

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	err = json.Unmarshal(payload, &device)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// If device is not configured in DB or DB failed, need to handle better
	configuredDevice, _ := db.GetConfiguredDevice(device.UDID)
	if configuredDevice.UDID != "" {
		// So if device is already configured just update the current provider data and dont change anything else
		configuredDevice.Provider = config.Config.EnvConfig.Nickname
		configuredDevice.HostAddress = config.Config.EnvConfig.HostAddress
		err = db.UpsertDeviceDB(configuredDevice)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert device in DB"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Updated device in DB for the current provider"})
		return
	}

	// If the device is not configured in DB
	device.Provider = config.Config.EnvConfig.Nickname
	device.HostAddress = config.Config.EnvConfig.HostAddress
	device.Connected = true

	err = db.UpsertDeviceDB(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert device in DB"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Added device in DB for the current provider"})
}

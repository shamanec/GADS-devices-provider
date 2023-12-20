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
	"github.com/shamanec/GADS-devices-provider/device"
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
	device := device.DeviceMap[udid]

	target := "http://localhost:" + device.Device.AppiumPort
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

type ProviderData struct {
	ProviderData            models.EnvConfig               `json:"provider"`
	ConnectedAndroidDevices []string                       `json:"connected_android_devices"`
	ConnectedIOSDevices     []string                       `json:"connected_ios_devices"`
	DeviceData              map[string]*device.LocalDevice `json:"device_data"`
}

func GetProviderData(c *gin.Context) {
	connectedAndroid := device.GetConnectedDevicesAndroid()
	connectedIOS := device.GetConnectedDevicesIOS()

	var providerData ProviderData

	providerData.ConnectedAndroidDevices = connectedAndroid
	providerData.ConnectedIOSDevices = connectedIOS
	providerData.ProviderData = util.Config.EnvConfig
	providerData.DeviceData = device.DeviceMap

	c.JSON(http.StatusOK, providerData)
}

func DeviceInfo(c *gin.Context) {
	udid := c.Param("udid")

	if device, ok := device.DeviceMap[udid]; ok {
		device.UpdateInstalledApps()
		device.InstallableApps = util.GetAllAppFiles()
		c.JSON(http.StatusOK, device)
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Did not find device with udid `%s`", udid)})
}

func DevicesInfo(c *gin.Context) {
	devices := []*device.LocalDevice{}

	for _, device := range device.DeviceMap {
		devices = append(devices, device)
	}

	c.JSON(http.StatusOK, devices)
}

type ProcessApp struct {
	App string `json:"app"`
}

func UninstallApp(c *gin.Context) {
	udid := c.Param("udid")

	if device, ok := device.DeviceMap[udid]; ok {
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

		if slices.Contains(device.Device.InstalledApps, payloadJson.App) {
			err = device.UninstallApp(payloadJson.App)
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

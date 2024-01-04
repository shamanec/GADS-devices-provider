package models

import (
	"context"

	"github.com/danielpaulus/go-ios/ios"
)

type CustomLogger interface {
	LogDebug(event_name string, message string)
	LogInfo(event_name string, message string)
	LogError(event_name string, message string)
	LogWarn(event_name string, message string)
	LogFatal(event_name string, message string)
	LogPanic(event_name string, message string)
}

type Device struct {
	Connected            bool     `json:"connected" bson:"connected"`
	UDID                 string   `json:"udid" bson:"udid"`
	OS                   string   `json:"os" bson:"os"`
	Name                 string   `json:"name" bson:"name"`
	OSVersion            string   `json:"os_version" bson:"os_version"`
	Model                string   `json:"model" bson:"model"`
	HostAddress          string   `json:"host_address" bson:"host_address"`
	Provider             string   `json:"provider" bson:"provider"`
	ScreenWidth          string   `json:"screen_width" bson:"screen_width"`
	ScreenHeight         string   `json:"screen_height" bson:"screen_height"`
	HardwareModel        string   `json:"hardware_model,omitempty" bson:"hardware_model,omitempty"`
	InstalledApps        []string `json:"installed_apps" bson:"-"`
	IOSProductType       string   `json:"ios_product_type,omitempty" bson:"ios_product_type,omitempty"`
	LastUpdatedTimestamp int64    `json:"last_updated_timestamp" bson:"last_updated_timestamp"`
}

type LocalDevice struct {
	Device           *Device
	ProviderState    string
	WdaReadyChan     chan bool          `json:"-"`
	Context          context.Context    `json:"-"`
	CtxCancel        context.CancelFunc `json:"-"`
	GoIOSDeviceEntry ios.DeviceEntry    `json:"-"`
	IsResetting      bool
	Logger           CustomLogger `json:"-"`
	InstallableApps  []string     `json:"installable_apps"`
	AppiumSessionID  string       `json:"appiumSessionID"`
	WDASessionID     string       `json:"wdaSessionID"`
	AppiumPort       string       `json:"appium_port"`
	StreamPort       string       `json:"stream_port"`
	WDAPort          string       `json:"wda_port"`
}

type IOSModelData struct {
	Width  string
	Height string
	Model  string
}

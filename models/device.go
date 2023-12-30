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
	Connected       bool     `json:"connected,omitempty" bson:"connected"`
	UDID            string   `json:"udid" bson:"_id"`
	OS              string   `json:"os" bson:"os"`
	AppiumPort      string   `json:"appium_port" bson:"appium_port"`
	StreamPort      string   `json:"stream_port" bson:"stream_port"`
	WDAPort         string   `json:"wda_port,omitempty" bson:"wda_port,omitempty"`
	Name            string   `json:"name" bson:"name"`
	OSVersion       string   `json:"os_version" bson:"os_version"`
	Model           string   `json:"model" bson:"model"`
	Image           string   `json:"image,omitempty" bson:"image,omitempty"`
	HostAddress     string   `json:"host_address" bson:"host_address"`
	AppiumSessionID string   `json:"appiumSessionID,omitempty" bson:"appiumSessionID,omitempty"`
	WDASessionID    string   `json:"wdaSessionID,omitempty" bson:"wdaSessionID,omitempty"`
	Provider        string   `json:"provider" bson:"provider"`
	ScreenWidth     string   `json:"screen_width" bson:"screen_width"`
	ScreenHeight    string   `json:"screen_height" bson:"screen_height"`
	HardwareModel   string   `json:"hardware_model,omitempty" bson:"hardware_model,omitempty"`
	InstalledApps   []string `json:"installed_apps" bson:"-"`
	IOSProductType  string   `json:"ios_product_type,omitempty" bson:"ios_product_type,omitempty"`
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
}

type IOSModelData struct {
	Width  string
	Height string
	Model  string
}

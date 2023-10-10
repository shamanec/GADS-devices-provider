package models

import (
	"context"

	"github.com/danielpaulus/go-ios/ios"
)

type ConfigJsonData struct {
	AppiumConfig AppiumConfig `json:"appium-config" bson:"appium-config"`
	EnvConfig    EnvConfig    `json:"env-config" bson:"env-config"`
	Devices      []*Device    `json:"devices-config" bson:"devices-config"`
}

type AppiumConfig struct {
	SeleniumHubHost         string `json:"selenium_hub_host" bson:"selenium_hub_host"`
	SeleniumHubPort         string `json:"selenium_hub_port" bson:"selenium_hub_port"`
	SeleniumHubProtocolType string `json:"selenium_hub_protocol_type" bson:"selenium_hub_protocol_type"`
}

type EnvConfig struct {
	DevicesHost         string `json:"devices_host" bson:"devices_host"`
	ConnectSeleniumGrid string `json:"connect_selenium_grid" bson:"connect_selenium_grid"`
	SupervisionPassword string `json:"supervision_password" bson:"supervision_password"`
	WDABundleID         string `json:"wda_bundle_id" bson:"wda_bundle_id"`
	RethinkDB           string `json:"rethink_db" bson:"rethink_db"`
	WDAPath             string `json:"wda_repo_path" bson:"wda_repo_path"`
}

type Device struct {
	Container            *DeviceContainer `json:"container,omitempty" bson:"container,omitempty"`
	Connected            bool             `json:"connected,omitempty" bson:"connected,omitempty"`
	Healthy              bool             `json:"healthy,omitempty" bson:"healthy,omitempty"`
	LastHealthyTimestamp int64            `json:"last_healthy_timestamp,omitempty" bson:"last_healthy_timestamp,omitempty"`
	UDID                 string           `json:"udid" bson:"_id"`
	OS                   string           `json:"os" bson:"os"`
	AppiumPort           string           `json:"appium_port" bson:"appium_port"`
	StreamPort           string           `json:"stream_port" bson:"stream_port"`
	ContainerServerPort  string           `json:"container_server_port" bson:"container_server_port"`
	WDAPort              string           `json:"wda_port,omitempty" bson:"wda_port,omitempty"`
	Name                 string           `json:"name" bson:"name"`
	OSVersion            string           `json:"os_version" bson:"os_version"`
	ScreenSize           string           `json:"screen_size" bson:"screen_size"`
	Model                string           `json:"model" bson:"model"`
	Image                string           `json:"image,omitempty" bson:"image,omitempty"`
	Host                 string           `json:"host" bson:"host"`
	AppiumSessionID      string           `json:"appiumSessionID,omitempty" bson:"appiumSessionID,omitempty"`
	WDASessionID         string           `json:"wdaSessionID,omitempty" bson:"wdaSessionID,omitempty"`
}

type LocalDevice struct {
	Device           *Device
	ProviderState    string
	WdaReadyChan     chan bool
	Context          context.Context
	CtxCancel        context.CancelFunc
	GoIOSDeviceEntry ios.DeviceEntry
}

type DeviceContainer struct {
	ContainerID     string `json:"id"`
	ContainerStatus string `json:"status"`
	ImageName       string `json:"image_name"`
	ContainerName   string `json:"container_name"`
}

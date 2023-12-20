package models

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
}

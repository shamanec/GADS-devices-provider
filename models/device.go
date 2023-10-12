package models

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
	Provider             string           `json:"provider" bson:"provider"`
}

type DeviceContainer struct {
	ContainerID     string `json:"id"`
	ContainerStatus string `json:"status"`
	ImageName       string `json:"image_name"`
	ContainerName   string `json:"container_name"`
}

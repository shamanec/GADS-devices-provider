package models

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
	MongoDB             string `json:"mongo_db" bson:"mongo_db"`
	WDAPath             string `json:"wda_repo_path" bson:"wda_repo_path"`
}

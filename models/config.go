package models

type ConfigJsonData struct {
	EnvConfig EnvConfig `json:"env-config" bson:"env-config"`
	Devices   []*Device `json:"devices-config" bson:"devices-config"`
}

type EnvConfig struct {
	HostAddress         string `json:"host_address" bson:"host_address"`
	HostPort            int    `json:"host_port" bson:"host_port"`
	UseSeleniumGrid     bool   `json:"use_selenium_grid" bson:"use_selenium_grid"`
	SupervisionPassword string `json:"supervision_password" bson:"-"`
	WDABundleID         string `json:"wda_bundle_id" bson:"-"`
	MongoDB             string `json:"mongo_db" bson:"-"`
	WDAPath             string `json:"wda_repo_path" bson:"-"`
	ProviderNickname    string `json:"provider_nickname" bson:"-"`
	SeleniumJar         string `json:"selenium_jar" bson:"-"`
	SeleniumGrid        string `json:"selenium_grid" bson:"selenium_grid"`
}

package models

type ConfigJsonData struct {
	EnvConfig ProviderDB `json:"env-config" bson:"env-config"`
	Devices   []*Device  `json:"devices-config" bson:"devices-config"`
}

type ProviderDB struct {
	OS                  string `json:"os" bson:"os"`
	Nickname            string `json:"nickname" bson:"nickname"`
	HostAddress         string `json:"host_address" bson:"host_address"`
	Port                int    `json:"port" bson:"port"`
	UseSeleniumGrid     bool   `json:"use_selenium_grid" bson:"use_selenium_grid"`
	SeleniumGrid        string `json:"selenium_grid" bson:"selenium_grid"`
	ProvideAndroid      bool   `json:"provide_android" bson:"provide_android"`
	ProvideIOS          bool   `json:"provide_ios" bson:"provide_ios"`
	WdaBundleID         string `json:"wda_bundle_id" bson:"wda_bundle_id"`
	SupervisionPassword string `json:"supervision_password" bson:"supervision_password"`
	WdaRepoPath         string `json:"wda_repo_path" bson:"wda_repo_path"`
	ProviderFolder      string `json:"-" bson:"-"`
}

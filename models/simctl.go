package models

type SimctlDevice struct {
	AvailabilityError    string `json:"availabilityError"`
	DataPath             string `json:"dataPath"`
	DataPathSize         int    `json:"dataPathSize"`
	LogPath              string `json:"logPath"`
	UDID                 string `json:"udid"`
	IsAvailable          bool   `json:"isAvailable"`
	DeviceTypeIdentifier string `json:"deviceTypeIdentifier"`
	State                string `json:"state"`
	Name                 string `json:"name"`
	LastBootedAt         string `json:"lastBootedAt,omitempty"`
	LogPathSize          int    `json:"logPathSize,omitempty"`
}

type SimctlDevices struct {
	SimctlDevice map[string][]SimctlDevice `json:"devices"`
}

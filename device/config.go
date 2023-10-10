package device

import (
	"context"
	"os"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/shamanec/GADS-devices-provider/models"
)

type ConfigJsonData struct {
}

type LocalDevice struct {
	Device           *models.Device
	ProviderState    string
	WdaReadyChan     chan bool
	Context          context.Context
	CtxCancel        context.CancelFunc
	GoIOSDeviceEntry ios.DeviceEntry
}

var projectDir string
var Config ConfigJsonData
var DeviceMap = make(map[string]*LocalDevice)

// Set up the configuration for the provider
// Get the data from config.json, start a DB connection and update the devices
func SetupConfig() error {
	var err error

	// Get the current project folder
	projectDir, err = os.Getwd()
	if err != nil {
		return err
	}

	createDeviceMap()

	createMongoLogCollectionsForAllDevices()

	return nil
}

func createDeviceMap() {
	getLocalDevices()
	for _, device := range localDevices {
		DeviceMap[device.Device.UDID] = device
	}
}

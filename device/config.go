package device

import (
	"context"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/shamanec/GADS-devices-provider/models"
)

type LocalDevice struct {
	Device           *models.Device
	ProviderState    string
	WdaReadyChan     chan bool
	Context          context.Context
	CtxCancel        context.CancelFunc
	GoIOSDeviceEntry ios.DeviceEntry
}

var DeviceMap = make(map[string]*LocalDevice)

// Create Mongo collections for all devices for logging
// Create a map of *device.LocalDevice for easier access across the code
func Setup() {
	getLocalDevices()
	createMongoLogCollectionsForAllDevices()
	createDeviceMap()
}

func createDeviceMap() {
	for _, device := range localDevices {
		DeviceMap[device.Device.UDID] = device
	}
}

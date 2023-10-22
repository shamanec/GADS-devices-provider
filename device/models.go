package device

import (
	"context"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/shamanec/GADS-devices-provider/models"
	"github.com/shamanec/GADS-devices-provider/util"
)

type LocalDevice struct {
	Device           *models.Device
	ProviderState    string
	WdaReadyChan     chan bool
	Context          context.Context
	CtxCancel        context.CancelFunc
	GoIOSDeviceEntry ios.DeviceEntry
	IsResetting      bool
	Logger           util.CustomLogger
}

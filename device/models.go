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
	WdaReadyChan     chan bool          `json:"-"`
	Context          context.Context    `json:"-"`
	CtxCancel        context.CancelFunc `json:"-"`
	GoIOSDeviceEntry ios.DeviceEntry    `json:"-"`
	IsResetting      bool
	Logger           util.CustomLogger `json:"-"`
}

package devices

import (
	"fmt"

	"github.com/danielpaulus/go-ios/ios/zipconduit"
	"github.com/shamanec/GADS-devices-provider/logger"
	"github.com/shamanec/GADS-devices-provider/models"
)

func InstallAppWithDevice(device *models.LocalDevice, fileName string) error {
	filePath := fileName

	logger.ProviderLogger.LogInfo("ios_device", fmt.Sprintf("Installing app `%s` on iOS device `%s`", filePath, device.Device.UDID))

	conn, err := zipconduit.New(device.GoIOSDeviceEntry)
	if err != nil {
		return fmt.Errorf("Failed creating zip conduit with go-ios - %s", err)
	}

	err = conn.SendFile(filePath)
	if err != nil {
		return fmt.Errorf("Failed installing application with go-ios - %s", err)
	}
	return nil
}

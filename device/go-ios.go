package device

import (
	"errors"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	log "github.com/sirupsen/logrus"
)

func InstallAppWithDevice(device ios.DeviceEntry, fileName string) error {
	filePath := "./apps/" + fileName

	conn, err := zipconduit.New(device)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "install_app",
		}).Error("Could not create zipconduit when installing app. Error: " + err.Error())
		return errors.New("Failed installing application:" + err.Error())
	}

	err = conn.SendFile(filePath)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "install_app",
		}).Error("Could not install app. Error: " + err.Error())
		return errors.New("Failed installing application:" + err.Error())
	}
	return nil
}

package provider

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// Generate the udev rules file
func CreateUdevRules() error {
	log.WithFields(log.Fields{
		"event": "create_udev_rules",
	}).Info("Creating udev rules")

	// Create the common devices udev rules file
	rulesFile, err := os.Create("./90-device.rules")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "create_udev_rules",
		}).Error("Could not create 90-device.rules file: " + err.Error())
		return err
	}
	defer rulesFile.Close()

	devicesList := ConfigData.DeviceConfig

	// For each device generate the respective rule lines
	for _, device := range devicesList {
		// Create a symlink when device is connected
		rule_line1 := `SUBSYSTEM=="usb", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", MODE="0666", SYMLINK+="device_` + device.DeviceUDID + `"`

		// Call provider server with udid when device is removed
		rule_line2 := `ACTION=="remove", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", RUN+="/usr/bin/curl -X POST http://localhost:` + ProviderPort + `/device-containers/remove/` + device.DeviceUDID + `"`

		// Call provider server with udid and device type when device is connected
		rule_line3 := `ACTION=="add", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", RUN+="/usr/bin/curl -X POST http://localhost:` + ProviderPort + `/device-containers/create/` + device.OS + `/` + device.DeviceUDID + `"`

		// Write the new lines for each device in the udev rules file
		if _, err := rulesFile.WriteString(rule_line1 + "\n"); err != nil {
			log.WithFields(log.Fields{
				"event": "create_udev_rules",
			}).Error("Could not write to 90-device.rules file: " + err.Error())
			return err
		}

		if _, err := rulesFile.WriteString(rule_line2 + "\n"); err != nil {
			log.WithFields(log.Fields{
				"event": "create_udev_rules",
			}).Error("Could not write to 90-device.rules file: " + err.Error())
			return err
		}

		if _, err := rulesFile.WriteString(rule_line3 + "\n"); err != nil {
			log.WithFields(log.Fields{
				"event": "create_udev_rules",
			}).Error("Could not write to 90-device.rules file: " + err.Error())
			return err
		}
	}

	return nil
}

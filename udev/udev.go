package udev

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
		symlink_line := `SUBSYSTEM=="usb", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", MODE="0666", SYMLINK+="device_` + device.OS + `_` + device.DeviceUDID + `"`

		// Write the new lines for each device in the udev rules file
		if _, err := rulesFile.WriteString(symlink_line + "\n"); err != nil {
			log.WithFields(log.Fields{
				"event": "create_udev_rules",
			}).Error("Could not write to 90-device.rules file: " + err.Error())
			return err
		}
	}

	return nil
}

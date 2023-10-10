package device

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// Generate the udev rules file
// These udev rules create symlinks for the devices in /dev
// Which we can then use to check device connectivity and attach devices to their respective containers
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

	// For each device in Config generate the respective rule lines
	for _, device := range localDevices {
		// Create a symlink when device is connected
		symlink_line := `SUBSYSTEM=="usb", ENV{ID_SERIAL_SHORT}=="` + device.Device.UDID + `", MODE="0666", SYMLINK+="device_` + device.Device.OS + `_` + device.Device.UDID + `"`

		// Write the rule line for each device in the udev rules file
		if _, err := rulesFile.WriteString(symlink_line + "\n"); err != nil {
			log.WithFields(log.Fields{
				"event": "create_udev_rules",
			}).Error("Could not write to 90-device.rules file: " + err.Error())
			return err
		}
	}

	return nil
}

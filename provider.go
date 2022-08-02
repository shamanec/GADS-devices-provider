package main

import (
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
)

//=======================//
//=====API FUNCTIONS=====//

// @Summary      Creates the udev rules for device symlink and container creation
// @Description  Creates 90-device.rules file to be used by udev
// @Tags         configuration
// @Produce      json
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /configuration/create-udev-rules [post]
func CreateUdevRules(w http.ResponseWriter, r *http.Request) {
	// Open /lib/systemd/system/systemd-udevd.service
	// Add IPAddressAllow=127.0.0.1 at the bottom
	// This is to allow curl calls from the udev rules to the GADS server
	err := CreateUdevRulesInternal()
	if err != nil {
		JSONError(w, "create_udev_rules", "Could not create udev rules file", 500)
		return
	}

	SimpleJSONResponse(w, "Successfully created 90-device.rules file in project dir", 200)
}

//=======================//
//=====FUNCTIONS=====//

// Generate the udev rules file
func CreateUdevRulesInternal() error {
	log.WithFields(log.Fields{
		"event": "create_udev_rules",
	}).Info("Creating udev rules")

	// Create the common devices udev rules file
	create_container_rules, err := os.Create("./90-device.rules")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "create_udev_rules",
		}).Error("Could not create 90-device.rules file: " + err.Error())
		return err
	}
	defer create_container_rules.Close()

	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "create_udev_rules",
		}).Error("Could not get config data when creating udev rules: " + err.Error())
		return err
	}

	devices_list := configData.DeviceConfig

	// For each device generate the respective rule lines
	for _, device := range devices_list {
		// Create a symlink when device is connected
		rule_line1 := `SUBSYSTEM=="usb", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", MODE="0666", SYMLINK+="device_` + device.DeviceUDID + `"`

		// Call provider server with udid when device is removed
		rule_line2 := `ACTION=="remove", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"udid\":\"` + device.DeviceUDID + `\"}' http://localhost:` + ProviderPort + `/device-containers/remove"`

		// Call provider server with udid and device type when device is connected
		rule_line3 := `ACTION=="add", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"device_type\":\"` + device.OS + `\", \"udid\":\"` + device.DeviceUDID + `\"}' http://localhost:` + ProviderPort + `/device-containers/create"`

		// Write the new lines for each device in the udev rules file
		if _, err := create_container_rules.WriteString(rule_line1 + "\n"); err != nil {
			log.WithFields(log.Fields{
				"event": "create_udev_rules",
			}).Error("Could not write to 90-device.rules file: " + err.Error())
			return err
		}

		if _, err := create_container_rules.WriteString(rule_line2 + "\n"); err != nil {
			log.WithFields(log.Fields{
				"event": "create_udev_rules",
			}).Error("Could not write to 90-device.rules file: " + err.Error())
			return err
		}

		if _, err := create_container_rules.WriteString(rule_line3 + "\n"); err != nil {
			log.WithFields(log.Fields{
				"event": "create_udev_rules",
			}).Error("Could not write to 90-device.rules file: " + err.Error())
			return err
		}
	}

	return nil
}

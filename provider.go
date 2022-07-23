package main

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	log "github.com/sirupsen/logrus"
)

var sudo_password = GetEnvValue("sudo_password")

// @Summary      Sets up udev devices listener
// @Description  Creates udev rules, moves them to /etc/udev/rules.d and reloads udev. Copies usbmuxd.service to /lib/systemd/system and enables it
// @Tags         configuration
// @Produce      json
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /configuration/setup-udev-listener [post]
func SetupUdevListener(w http.ResponseWriter, r *http.Request) {
	// Open /lib/systemd/system/systemd-udevd.service
	// Add IPAddressAllow=127.0.0.1 at the bottom
	// This is to allow curl calls from the udev rules to the GADS server
	if sudo_password == "undefined" {
		log.WithFields(log.Fields{
			"event": "setup_udev_listener",
		}).Error("Elevated permissions are required to perform this action. Please set your sudo password in './configs/config.json' or via the '/configuration/set-sudo-password' endpoint.")
		JSONError(w, "setup_udev_listener", "Elevated permissions are required to perform this action.", 500)
		return
	}
	err := SetupUdevListenerInternal()
	if err != nil {
		JSONError(w, "setup_udev_listener", "Could not setup udev rules", 500)
	}

	SimpleJSONResponse(w, "Successfully set udev rules.", 200)
}

// @Summary      Removes udev device listener
// @Description  Deletes udev rules from /etc/udev/rules.d and reloads udev
// @Tags         configuration
// @Produce      json
// @Success      200 {object} JsonResponse
// @Failure      500 {object} JsonErrorResponse
// @Router       /configuration/remove-device-listener [post]
func RemoveUdevListener(w http.ResponseWriter, r *http.Request) {
	err := RemoveUdevListenerInternal()
	if err != nil {
		JSONError(w, "remove_udev_listener", err.Error(), 500)
	}
}

//=======================//
//=====FUNCTIONS=====//

// Completely setup udev and usbmuxd
func SetupUdevListenerInternal() error {
	DeleteTempUdevFiles()

	err := CreateUdevRules()
	if err != nil {
		DeleteTempUdevFiles()
		return err
	}

	err = SetUdevRules()
	if err != nil {
		DeleteTempUdevFiles()
		return err
	}

	// err = CopyFileShell("./configs/usbmuxd.service", "/lib/systemd/system/", sudo_password)
	// if err != nil {
	// 	DeleteTempUdevFiles()
	// 	return err
	// }

	// err = EnableUsbmuxdService()
	// if err != nil {
	// 	DeleteTempUdevFiles()
	// 	return err
	// }

	DeleteTempUdevFiles()
	return nil
}

// Remove udev rules for the devices
func RemoveUdevListenerInternal() error {
	// Delete the rules file
	err := DeleteFileShell("/etc/udev/rules.d/90-device.rules", sudo_password)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "remove_udev_listener",
		}).Error("Could not delete udev rules file. Error: " + err.Error())
		return err
	}

	// Reload udev after removing the rules
	commandString := "echo '" + sudo_password + "' | sudo -S udevadm control --reload-rules"
	cmd := exec.Command("bash", "-c", commandString)
	err = cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "remove_udev_listener",
		}).Error("Could not reload udev rules file. Error: " + err.Error())
		return err
	}

	return nil
}

// Delete the temporary iOS udev rule file
func DeleteTempUdevFiles() {
	DeleteFileShell("./90-device.rules", sudo_password)
}

// Generate the temporary iOS udev rule file
func CreateUdevRules() error {
	log.WithFields(log.Fields{
		"event": "create_udev_rules",
	}).Info("Creating udev rules")

	// Create the common devices udev rules file
	create_container_rules, err := os.Create("./90-device.rules")
	if err != nil {
		return errors.New("Could not create 90-device.rules")
	}
	defer create_container_rules.Close()

	// Get the config data
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "create_udev_rules",
		}).Error("Could not unmarshal config.json file when creating udev rules")
		return err
	}

	devices_list := configData.DeviceConfig

	// For each device generate the respective rule lines
	for _, device := range devices_list {
		rule_line1 := `SUBSYSTEM=="usb", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", MODE="0666", SYMLINK+="device_` + device.DeviceUDID + `"`
		rule_line2 := `ACTION=="remove", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"udid\":\"` + device.DeviceUDID + `\"}' http://localhost:10000/device-containers/remove"`
		rule_line3 := `ACTION=="add", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUDID + `", RUN+="/usr/bin/curl -X POST -H \"Content-Type: application/json\" -d '{\"device_type\":\"` + device.OS + `\", \"udid\":\"` + device.DeviceUDID + `\"}' http://localhost:10000/device-containers/create"`
		//rule_line2 := `ACTION=="add", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUdid + `", RUN+="/usr/local/bin/docker-cli start-device-container --device_type=` + device.OS + ` --udid=` + device.DeviceUdid + `"`
		//rule_line3 := `ACTION=="remove", ENV{ID_SERIAL_SHORT}=="` + device.DeviceUdid + `", RUN+="/usr/local/bin/docker-cli remove-device-container --udid=` + device.DeviceUdid + `"`

		// Write the new lines for each device in the udev rules file
		if _, err := create_container_rules.WriteString(rule_line1 + "\n"); err != nil {
			return errors.New("Could not write to 90-device.rules")
		}

		if _, err := create_container_rules.WriteString(rule_line2 + "\n"); err != nil {
			return errors.New("Could not write to 90-device.rules")
		}

		if _, err := create_container_rules.WriteString(rule_line3 + "\n"); err != nil {
			return errors.New("Could not write to 90-device.rules")
		}
	}

	return nil
}

// Copy the iOS udev rules to /etc/udev/rules.d and reload udev
func SetUdevRules() error {
	// Copy the udev rules to /etc/udev/rules.d
	err := CopyFileShell("./90-device.rules", "/etc/udev/rules.d/90-device.rules", sudo_password)
	if err != nil {
		return err
	}

	// Reload the udev rules after updating them
	commandString := "echo '" + sudo_password + "' | sudo -S udevadm control --reload-rules"
	cmd := exec.Command("bash", "-c", commandString)
	err = cmd.Run()
	if err != nil {
		return errors.New("Could not reload udev rules")
	}
	return nil
}

// Get an env value from ./configs/config.json
func GetEnvValue(key string) string {
	configData, err := GetConfigJsonData()
	if err != nil {
		log.WithFields(log.Fields{
			"event": "check_sudo_password",
		}).Error("Could not unmarshal ./configs/config.json file when getting value")
	}

	if key == "sudo_password" {
		return configData.EnvConfig.SudoPassword
	} else if key == "supervision_password" {
		return configData.EnvConfig.SupervisionPassword
	} else if key == "connect_selenium_grid" {
		return strconv.FormatBool(configData.EnvConfig.ConnectSeleniumGrid)
	}
	return ""
}

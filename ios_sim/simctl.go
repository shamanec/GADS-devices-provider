package ios_sim

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/shamanec/GADS-devices-provider/logger"
	"github.com/shamanec/GADS-devices-provider/models"
	"os/exec"
	"strconv"
	"strings"
)

func XcrunExecGeneric(args ...string) ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "xcrun", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return []byte{}, err
	}

	return output, nil
}

func GetSimulatorsData() (models.SimctlDevices, error) {
	output, err := XcrunExecGeneric("simctl", "list", "devices", "-je")
	if err != nil {
		return models.SimctlDevices{}, err
	}

	var simData models.SimctlDevices
	err = json.Unmarshal(output, &simData)
	if err != nil {
		return models.SimctlDevices{}, err
	}
	return simData, nil
}

func GetBootedSimsUDIDs() ([]string, error) {
	simData, err := GetSimulatorsData()
	if err != nil {
		return []string{}, err
	}

	var bootedSims []string
	for _, devices := range simData.SimctlDevice {
		for _, device := range devices {
			if device.State == "Booted" {
				bootedSims = append(bootedSims, device.UDID)
			}
		}
	}
	return bootedSims, nil
}

func GetBootedSims() ([]models.SimctlDevice, error) {
	simData, err := GetSimulatorsData()
	if err != nil {
		return []models.SimctlDevice{}, err
	}

	var bootedSims []models.SimctlDevice
	for _, devices := range simData.SimctlDevice {
		for _, device := range devices {
			if device.State == "Booted" {
				bootedSims = append(bootedSims, device)
			}
		}
	}
	return bootedSims, nil
}

func GetAvailableSims() ([]models.SimctlDevice, error) {
	simData, err := GetSimulatorsData()
	if err != nil {
		return []models.SimctlDevice{}, err
	}

	var availableSims []models.SimctlDevice
	for _, devices := range simData.SimctlDevice {
		for _, device := range devices {
			if device.IsAvailable {
				availableSims = append(availableSims, device)
			}
		}
	}
	return availableSims, nil
}

func BootSim(udid string) error {
	logger.ProviderLogger.LogInfo("ios_sim", fmt.Sprintf("Booting simulator `%s`", udid))
	availableSims, err := GetAvailableSims()
	if err != nil {
		return err
	}

	for _, availableSim := range availableSims {
		if availableSim.UDID == udid && availableSim.State == "Booted" {
			return fmt.Errorf("Simulator `%s` is already booted", udid)
		}
	}

	_, err = XcrunExecGeneric("simctl", "boot", udid)
	if err != nil {
		return err
	}

	return nil
}

func ShutdownSim(udid string) error {
	logger.ProviderLogger.LogInfo("ios_sim", fmt.Sprintf("Shutting down simulator `%s`", udid))
	bootedSims, err := GetBootedSims()
	if err != nil {
		return err
	}

	for _, bootedSim := range bootedSims {
		if bootedSim.UDID == udid {
			_, err = XcrunExecGeneric("simctl", "shutdown", udid)
			if err != nil {
				return err
			}
		}
	}
	logger.ProviderLogger.LogInfo("ios_sims", fmt.Sprintf("Simulator `%s` is not booted", udid))
	return nil
}

func GetSimScreenSize(udid string) (string, string, error) {
	output, err := XcrunExecGeneric("simctl", "io", udid, "enumerate")
	if err != nil {
		return "", "", err
	}

	scanner := bufio.NewScanner(bytes.NewBuffer(output))
	var width, height, scale string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Pixel Size: ") {
			splitLine := strings.Split(line, ": ")
			screenSizeData := splitLine[1]
			cleanedScreenSizeData := strings.TrimSuffix(strings.TrimPrefix(screenSizeData, "{"), "}")
			splitScreenSizeData := strings.Split(cleanedScreenSizeData, ", ")
			width = splitScreenSizeData[0]
			height = splitScreenSizeData[1]
		}

		if strings.Contains(line, "Preferred UI Scale: ") {
			splitLine := strings.Split(line, ": ")
			scale = splitLine[1]
			break
		}
	}

	widthInt, _ := strconv.Atoi(width)
	heightInt, _ := strconv.Atoi(height)
	scaleInt, _ := strconv.Atoi(scale)

	scaledWidth := widthInt / scaleInt
	scaledHeight := heightInt / scaleInt

	return fmt.Sprintf("%v", scaledWidth), fmt.Sprintf("%v", scaledHeight), nil
}

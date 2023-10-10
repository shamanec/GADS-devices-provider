package main

import (
	"flag"
	"fmt"

	"github.com/shamanec/GADS-devices-provider/device"
	_ "github.com/shamanec/GADS-devices-provider/docs"
	"github.com/shamanec/GADS-devices-provider/router"
	"github.com/shamanec/GADS-devices-provider/util"
)

func main() {
	util.SetLogging()

	port_flag := flag.String("port", "10001", "The port to run the server on")
	flag.Parse()
	util.ProviderLogger.LogInfo("provider_setup", fmt.Sprintf("Starting provider on port `%v`", *port_flag))

	// Parse config.json, get the connected devices and updated the DB with the initial data
	err := device.SetupConfig()
	if err != nil {
		util.ProviderLogger.LogError("provider_setup", fmt.Sprintf("Initial config setup failed - %s", err))
	}

	util.GetConfigJsonData()

	// Start a goroutine that will update devices on provider start and when there are events in /dev(device connected/disconnected)
	go device.UpdateDevices()

	// Handle the endpoints
	r := router.HandleRequests()

	r.Run(":10001")
}

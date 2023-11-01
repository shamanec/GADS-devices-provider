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

	port_flag := flag.String("port", "10001", "The port to run the server on")
	log_level := flag.String("log_level", "info", "The log level of the provider app - debug, info or error")
	flag.Parse()

	util.SetupConfig()
	util.InitMongoClient()
	defer util.CloseMongoConn()

	util.SetupLogging(*log_level)
	util.UpsertProviderMongo()

	util.ProviderLogger.LogInfo("provider_setup", fmt.Sprintf("Starting provider on port `%v`", *port_flag))

	// Start a goroutine that will update devices on provider start
	go device.UpdateDevices()

	// Handle the endpoints
	r := router.HandleRequests()

	r.Run(fmt.Sprintf(":%v", util.Config.EnvConfig.HostPort))
}

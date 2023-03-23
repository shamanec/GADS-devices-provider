package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/shamanec/GADS-devices-provider/config"
	"github.com/shamanec/GADS-devices-provider/device"
	_ "github.com/shamanec/GADS-devices-provider/docs"
	"github.com/shamanec/GADS-devices-provider/router"

	log "github.com/sirupsen/logrus"
)

var projectLogFile *os.File

func setLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	projectLogFile, err := os.OpenFile("./logs/provider.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic("Could not set log output" + err.Error())
	}
	log.SetOutput(projectLogFile)
}

func main() {
	config.SetupConfig()

	setLogging()

	go device.UpdateDevices()
	handler := router.HandleRequests()

	fmt.Printf("Starting provider on port:%v\n", config.ProviderPort)
	log.Fatal(http.ListenAndServe(":"+config.ProviderPort, handler))
}

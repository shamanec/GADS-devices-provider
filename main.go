package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/shamanec/GADS-devices-provider/docker"
	_ "github.com/shamanec/GADS-devices-provider/docs"
	"github.com/shamanec/GADS-devices-provider/provider"
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
	provider.SetupConfig()

	setLogging()

	//go docker.DevicesWatcher()
	go docker.StartDevicesListener()
	handler := router.HandleRequests()

	fmt.Printf("Starting provider on port:%v\n", provider.ProviderPort)
	log.Fatal(http.ListenAndServe(":"+provider.ProviderPort, handler))
}

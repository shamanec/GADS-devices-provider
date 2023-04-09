package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

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
		panic("Could not set log output: " + err.Error())
	}
	log.SetOutput(projectLogFile)
}

func main() {
	setLogging()

	port_flag := flag.String("port", "10001", "The port to run the server on")
	flag.Parse()

	fmt.Printf("Starting provider on port:%v\n", *port_flag)

	err := device.SetupConfig()
	if err != nil {
		fmt.Println("Initial config setup failed: " + err.Error())
	}

	go device.UpdateDevices()
	handler := router.HandleRequests()

	log.Fatal(http.ListenAndServe(":"+*port_flag, handler))
}

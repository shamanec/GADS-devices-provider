package main

import (
	"fmt"
	"net/http"
	"os"

	_ "github.com/shamanec/GADS-devices-provider/docs"
	"github.com/shamanec/GADS-devices-provider/provider"
	"github.com/shamanec/GADS-devices-provider/router"

	log "github.com/sirupsen/logrus"
)

var project_log_file *os.File

func setLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	project_log_file, err := os.OpenFile("./logs/provider.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic("Could not set log output" + err.Error())
	}
	log.SetOutput(project_log_file)
}

func main() {
	provider.SetupConfig()

	setLogging()

	handler := router.HandleRequests()

	fmt.Printf("Starting provider on port:%v\n", provider.ProviderPort)
	log.Fatal(http.ListenAndServe(":"+provider.ProviderPort, handler))
}

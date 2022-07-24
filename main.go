package main

import (
	"net/http"
	"os"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var project_log_file *os.File
var ProviderPort = "10001"

func setLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	project_log_file, err := os.OpenFile("./logs/provider.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic(err)
	}
	log.SetOutput(project_log_file)
}

func handleRequests() {
	// Create a new instance of the mux router
	myRouter := mux.NewRouter().StrictSlash(true)

	myRouter.HandleFunc("/available-devices", GetAvailableDevicesInfo).Methods("GET")
	myRouter.HandleFunc("/device-containers/remove", RemoveDeviceContainer).Methods("POST")
	myRouter.HandleFunc("/device-containers/create", CreateDeviceContainer).Methods("POST")
	myRouter.HandleFunc("/containers/{container_id}/restart", RestartContainer).Methods("POST")
	myRouter.HandleFunc("/containers/{container_id}/remove", RemoveContainer).Methods("POST")
	myRouter.HandleFunc("/containers/{container_id}/logs", GetContainerLogs).Methods("GET")
	myRouter.HandleFunc("/configuration/create-udev-rules", CreateUdevRules).Methods("POST")
	myRouter.HandleFunc("/provider-logs", GetLogs).Methods("GET")

	log.Fatal(http.ListenAndServe(":"+ProviderPort, myRouter))
}

func main() {
	setLogging()
	handleRequests()
}

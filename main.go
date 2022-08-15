package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	_ "github.com/shamanec/GADS-devices-provider/docs"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	httpSwagger "github.com/swaggo/http-swagger"
)

var project_log_file *os.File
var ProviderPort string
var HomeDir string
var ProjectDir string

func setLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	project_log_file, err := os.OpenFile("./logs/provider.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic("Could not set log output" + err.Error())
	}
	log.SetOutput(project_log_file)
}

func originHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(w, r)
	})
}

func handleRequests() {
	// Create a new instance of the mux router
	router := mux.NewRouter().StrictSlash(true)

	router.PathPrefix("/swagger").Handler(httpSwagger.WrapHandler)
	router.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"), //The url pointing to API definition
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("#swagger-ui"),
	))

	router.HandleFunc("/available-devices", GetAvailableDevicesInfo).Methods("GET")
	router.HandleFunc("/device-containers/remove", RemoveDeviceContainer).Methods("POST")
	router.HandleFunc("/device-containers/create", CreateDeviceContainer).Methods("POST")
	router.HandleFunc("/containers/{container_id}/restart", RestartContainer).Methods("POST")
	router.HandleFunc("/containers/{container_id}/remove", RemoveContainer).Methods("POST")
	router.HandleFunc("/containers/{container_id}/logs", GetContainerLogs).Methods("GET")
	router.HandleFunc("/configuration/create-udev-rules", CreateUdevRules).Methods("POST")
	router.HandleFunc("/provider-logs", GetLogs)
	router.HandleFunc("/device-containers", GetDeviceContainers).Methods("GET")

	log.Fatal(http.ListenAndServe(":"+ProviderPort, originHandler(router)))
}

func main() {
	HomeDir, _ = os.UserHomeDir()
	ProjectDir, _ = os.Getwd()

	port_flag := flag.String("port", "10001", "The port to run the server on")
	flag.Parse()

	ProviderPort = *port_flag

	fmt.Printf("Starting provider on port:%v\n", ProviderPort)

	setLogging()
	handleRequests()
}

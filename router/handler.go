package router

import (
	"net/http"

	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

// Make all browser requests to any provider host accessible
func originHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(w, r)
	})
}

func HandleRequests() http.Handler {
	// Create a new instance of the mux router
	router := mux.NewRouter().StrictSlash(true)

	router.PathPrefix("/swagger").Handler(httpSwagger.WrapHandler)
	router.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"), //The url pointing to API definition
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("#swagger-ui"),
	))

	router.HandleFunc("/device/{udid}/health", DeviceHealth).Methods("GET")
	router.HandleFunc("/device/list", GetProviderDevices).Methods("GET")
	router.HandleFunc("/containers/{container_id}/logs", GetContainerLogs).Methods("GET")
	router.HandleFunc("/device/create-udev-rules", CreateUdevRules).Methods("POST")
	router.HandleFunc("/logs", GetLogs)

	return originHandler(router)
}

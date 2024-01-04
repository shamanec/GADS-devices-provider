package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/shamanec/GADS-devices-provider/config"
	"github.com/shamanec/GADS-devices-provider/db"
	"github.com/shamanec/GADS-devices-provider/devices"
	_ "github.com/shamanec/GADS-devices-provider/docs"
	"github.com/shamanec/GADS-devices-provider/logger"
	"github.com/shamanec/GADS-devices-provider/router"
)

func main() {

	// Flags for the provider startup
	log_level := flag.String("log-level", "info", "The log level of the provider app - debug, info or error")
	nickname := flag.String("nickname", "", "The nickname of the provider")
	mongo_db := flag.String("mongo-db", "localhost:27017", "The address of the MongoDB instance")
	provider_folder := flag.String("provider-folder", ".", "The folder where logs and apps are stored")
	flag.Parse()

	// Nickname is mandatory, this is what we use to get the configuration from the DB
	// if *nickname == "" {
	// 	log.Fatal("Please provide --nickname=* flag")
	// }

	// Print out some info on startup, maybe a flag was missed
	fmt.Printf("Current log level: %s, use the --log-level flag to change it\n", *log_level)
	fmt.Printf("Will use `%s` as address for MongoDB instance, use the --mongo-db flag to change it\n", *mongo_db)
	fmt.Printf("Will use `%s` as provider folder, use the --provider-folder flag to change it\n", *provider_folder)

	// Create a connection to Mongo
	db.InitMongoClient(fmt.Sprintf("%v", *mongo_db))
	// Set up the provider configuration
	config.SetupConfig(fmt.Sprintf("%v", *nickname), fmt.Sprintf("%v", *provider_folder))
	config.Config.EnvConfig.OS = runtime.GOOS
	// Defer closing the Mongo connection on provider stopped
	defer db.CloseMongoConn()

	// If on Linux or Windows and iOS devices provision enabled check for WebDriverAgent.ipa
	if config.Config.EnvConfig.OS != "macos" && config.Config.EnvConfig.ProvideIOS {
		// Check for WDA ipa, then WDA app availability
		_, err := os.Stat(fmt.Sprintf("%s/apps/WebDriverAgent.ipa", *provider_folder))
		if err != nil {
			_, err = os.Stat(fmt.Sprintf("%s/apps/WebDriverAgent.app", *provider_folder))
			if os.IsNotExist(err) {
				log.Fatalf("You should put signed WebDriverAgent.ipa file in the `apps` folder in `%s`", *provider_folder)
			}
		}
	}

	// If Android devices provision enabled check for gads-stream.apk
	if config.Config.EnvConfig.ProvideAndroid {
		_, err := os.Stat(fmt.Sprintf("%s/apps/gads-stream.apk", *provider_folder))
		if os.IsNotExist(err) {
			log.Fatalf("You should put gads-stream.apk file in the `apps` folder in `%s`", *provider_folder)
		}
	}

	// Create logs folder if it doesn't exist
	_, err := os.Stat(fmt.Sprintf("%s/logs", *provider_folder))
	if os.IsNotExist(err) {
		err = os.Mkdir(fmt.Sprintf("%s/logs", *provider_folder), os.ModePerm)
		if err != nil {
			log.Fatal("Could not create logs folder - " + err.Error())
		}
	} else if err != nil {
		log.Fatal("Could not create logs folder - " + err.Error())
	}

	// Setup logging for the provider itself
	logger.SetupLogging(*log_level)
	logger.ProviderLogger.LogInfo("provider_setup", fmt.Sprintf("Starting provider on port `%v`", config.Config.EnvConfig.Port))

	// Start a goroutine that will update devices on provider start
	go devices.UpdateDevices()

	// Handle the endpoints
	r := router.HandleRequests()

	// Start the provider
	r.Run(fmt.Sprintf(":%v", config.Config.EnvConfig.Port))
}

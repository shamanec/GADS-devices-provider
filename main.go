package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/shamanec/GADS-devices-provider/device"
	_ "github.com/shamanec/GADS-devices-provider/docs"
	"github.com/shamanec/GADS-devices-provider/router"
	"github.com/shamanec/GADS-devices-provider/util"
)

func main() {

	log_level := flag.String("log-level", "info", "The log level of the provider app - debug, info or error")
	nickname := flag.String("nickname", "", "The nickname of the provider")
	mongo_db := flag.String("mongo-db", "localhost:27017", "The address of the MongoDB instance")
	provider_folder := flag.String("provider-folder", ".", "The folder where logs and apps are stored")
	flag.Parse()

	if *nickname == "" {
		log.Fatal("Please provide --nickname=* flag")
	}

	fmt.Printf("Current log level: %s, use the --log-level flag to change it", *log_level)
	fmt.Printf("Will use `%s` as address for MongoDB instance, use the --mongo-db flag to change it\n", *mongo_db)
	fmt.Printf("Will use `%s` as provider folder, use the --provider-folder flag to change it\n", *provider_folder)

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

	util.InitMongoClient(fmt.Sprintf("%v", *mongo_db))
	util.SetupConfig(fmt.Sprintf("%v", *nickname), fmt.Sprintf("%v", *provider_folder))
	defer util.CloseMongoConn()

	util.SetupLogging(*log_level)

	util.ProviderLogger.LogInfo("provider_setup", fmt.Sprintf("Starting provider on port `%v`", util.Config.EnvConfig.Port))

	// Start a goroutine that will update devices on provider start
	go device.UpdateDevices()

	// Handle the endpoints
	r := router.HandleRequests()

	r.Run(fmt.Sprintf(":%v", util.Config.EnvConfig.Port))
}

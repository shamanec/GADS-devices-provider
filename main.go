package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/shamanec/GADS-devices-provider/config"
	"github.com/shamanec/GADS-devices-provider/db"
	"github.com/shamanec/GADS-devices-provider/devices"
	_ "github.com/shamanec/GADS-devices-provider/docs"
	"github.com/shamanec/GADS-devices-provider/logger"
	"github.com/shamanec/GADS-devices-provider/models"
	"github.com/shamanec/GADS-devices-provider/router"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// Parse command line flags
	log_level, nickname, mongo_db, provider_folder := parseFlags()

	// Create a connection to Mongo
	db.InitMongoClient(mongo_db)
	defer db.MongoCtxCancel()
	// Set up the provider configuration
	config.SetupConfig(nickname, provider_folder)
	config.Config.EnvConfig.OS = runtime.GOOS
	// Defer closing the Mongo connection on provider stopped
	defer db.CloseMongoConn()

	// Check if logs folder exists in given provider folder and attempt to create it if it doesn't exist
	createFolderIfNotExist(provider_folder, "logs")
	// Check if conf folder exists in given provider folder and attempt to create it if it doesn't exist
	createFolderIfNotExist(provider_folder, "conf")
	// Check if apps folder exists in given provider folder and attempt to create it if it doesn't exist
	createFolderIfNotExist(provider_folder, "apps")

	// Finalize grid configuration if Selenium Grid usage enabled
	if config.Config.EnvConfig.UseSeleniumGrid {
		configureSeleniumSettings(provider_folder)
	}

	// Setup logging for the provider itself
	logger.SetupLogging(log_level)
	logger.ProviderLogger.LogInfo("provider_setup", fmt.Sprintf("Starting provider on port `%v`", config.Config.EnvConfig.Port))

	// If on Linux or Windows and iOS devices provision enabled check for WebDriverAgent.ipa/app
	configureWebDriverBinary(provider_folder)

	// Start a goroutine that will update devices on provider start
	go devices.DevicesListener()

	// Start the provider
	startHTTPServer()
}

func startHTTPServer() {
	// Handle the endpoints
	r := router.HandleRequests()
	// Start periodically updating the provider data in the DB
	go updateProviderInDB()
	// Start the provider
	r.Run(fmt.Sprintf(":%v", config.Config.EnvConfig.Port))
}

// Create a required provider folder if it doesn't exist
func createFolderIfNotExist(baseFolder, subFolder string) {
	folderPath := fmt.Sprintf("%s/%s", baseFolder, subFolder)
	_, err := os.Stat(folderPath)
	if os.IsNotExist(err) {
		fmt.Printf("`%s` folder does not exist in `%s` provider folder, attempting to create it\n", subFolder, baseFolder)
		err = os.Mkdir(folderPath, os.ModePerm)
		if err != nil {
			log.Fatalf("Could not create `%s` folder in `%s` provider folder - %s", subFolder, baseFolder, err)
		}
	} else if err != nil {
		log.Fatalf("Could not stat `%s` folder in `%s` provider folder - %s", subFolder, baseFolder, err)
	}
}

func parseFlags() (string, string, string, string) {
	log_level := flag.String("log-level", "info", "The log level of the provider app - debug, info, or error")
	nickname := flag.String("nickname", "", "The nickname of the provider")
	mongo_db := flag.String("mongo-db", "localhost:27017", "The address of the MongoDB instance")
	provider_folder := flag.String("provider-folder", ".", "The folder where logs and apps are stored")
	flag.Parse()

	//Nickname is mandatory, this is what we use to get the configuration from the DB
	if *nickname == "" {
		log.Fatal("Please provide --nickname flag")
	}

	// Print out some info on startup, maybe a flag was missed
	fmt.Printf("Current log level: %s, use the --log-level flag to change it\n", *log_level)
	fmt.Printf("Will use `%s` as address for MongoDB instance, use the --mongo-db flag to change it\n", *mongo_db)
	fmt.Printf("Will use `%s` as provider folder, use the --provider-folder flag to change it\n", *provider_folder)

	// Remove trailing slash if provided, all code assumes its not there
	*provider_folder, _ = strings.CutSuffix(*provider_folder, "/")
	return *log_level, *nickname, *mongo_db, *provider_folder
}

// Check for and set up selenium jar file for creating Appium grid nodes in config
func configureSeleniumSettings(providerFolder string) {
	seleniumJarFile := ""
	filepath.Walk(fmt.Sprintf("%s/conf", config.Config.EnvConfig.ProviderFolder), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.Contains(info.Name(), "selenium") && filepath.Ext(path) == ".jar" {
			seleniumJarFile = info.Name()
			return nil
		}
		return nil
	})
	if seleniumJarFile == "" {
		log.Fatalf("You have enabled Selenium Grid connection but no selenium jar file was found in the `conf` folder in `%s`", config.Config.EnvConfig.ProviderFolder)
	}
	config.Config.EnvConfig.SeleniumJarFile = seleniumJarFile
}

// Check for and set up WebDriverAgent.ipa/app binary in config
func configureWebDriverBinary(providerFolder string) {
	if config.Config.EnvConfig.OS != "darwin" && config.Config.EnvConfig.ProvideIOS {
		// Check for WDA ipa, then WDA app availability
		ipaPath := fmt.Sprintf("%s/conf/WebDriverAgent.ipa", providerFolder)
		_, err := os.Stat(ipaPath)
		if err != nil {
			appPath := fmt.Sprintf("%s/conf/WebDriverAgent.app", providerFolder)
			_, err = os.Stat(appPath)
			if os.IsNotExist(err) {
				log.Fatalf("You should put signed WebDriverAgent.ipa/app file in the `conf` folder in `%s`", providerFolder)
			}
			config.Config.EnvConfig.WebDriverBinary = "WebDriverAgent.app"
		} else {
			config.Config.EnvConfig.WebDriverBinary = "WebDriverAgent.ipa"
		}
	}
}

// Periodically send current provider data updates to MongoDB
func updateProviderInDB() {
	for {
		coll := db.MongoClient().Database("gads").Collection("providers")
		filter := bson.D{{Key: "nickname", Value: config.Config.EnvConfig.Nickname}}

		var providedDevices []models.Device
		for _, mapDevice := range devices.DeviceMap {
			providedDevices = append(providedDevices, *mapDevice)
		}
		sort.Sort(models.ByUDID(providedDevices))

		update := bson.M{
			"$set": bson.M{
				"last_updated":     time.Now().UnixMilli(),
				"provided_devices": providedDevices,
			},
		}
		opts := options.Update().SetUpsert(true)
		ctx, _ := context.WithCancel(db.MongoCtx())
		_, err := coll.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			logger.ProviderLogger.LogError("update_provider", fmt.Sprintf("Failed to upsert provider in DB - %s", err))
		}
		time.Sleep(1 * time.Second)
	}
}

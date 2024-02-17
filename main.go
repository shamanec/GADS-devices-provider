package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/shamanec/GADS-devices-provider/util"
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
	logLevel, nickname, mongoDb, providerFolder := parseFlags()

	fmt.Println("Preparing...")

	// Create a connection to Mongo
	db.InitMongoClient(mongoDb)
	defer db.MongoCtxCancel()
	// Set up the provider configuration
	config.SetupConfig(nickname, providerFolder)
	config.Config.EnvConfig.OS = runtime.GOOS
	// Defer closing the Mongo connection on provider stopped
	defer db.CloseMongoConn()

	// Check if logs folder exists in given provider folder and attempt to create it if it doesn't exist
	createFolderIfNotExist(providerFolder, "logs")
	// Check if conf folder exists in given provider folder and attempt to create it if it doesn't exist
	createFolderIfNotExist(providerFolder, "conf")
	// Check if apps folder exists in given provider folder and attempt to create it if it doesn't exist
	createFolderIfNotExist(providerFolder, "apps")

	// Setup logging for the provider itself
	logger.SetupLogging(logLevel)
	logger.ProviderLogger.LogInfo("provider_setup", fmt.Sprintf("Starting provider on port `%v`", config.Config.EnvConfig.Port))

	// If running on macOS
	if config.Config.EnvConfig.OS == "darwin" && config.Config.EnvConfig.ProvideIOS {
		// Add a trailing slash to WDA repo folder if its missing
		if !strings.HasSuffix(config.Config.EnvConfig.WdaRepoPath, "/") {
			fmt.Println("Adding slash")
			config.Config.EnvConfig.WdaRepoPath = fmt.Sprintf("%s/", config.Config.EnvConfig.WdaRepoPath)
		}
		// Check if xcodebuild is available - Xcode and command line tools should be installed
		if !util.XcodebuildAvailable() {
			log.Fatal("xcodebuild is not available, you need to set up the host as explained in the readme")
		}

		if !util.GoIOSAvailable() {
			log.Fatal("provider", "go-ios is not available, you need to set up the host as explained in the readme")
		}

		// Check if provided WebDriverAgent repo path exists
		_, err := os.Stat(config.Config.EnvConfig.WdaRepoPath)
		if err != nil {
			log.Fatalf("`%s` does not exist, you need to provide valid path to the WebDriverAgent repo in the provider configuration", config.Config.EnvConfig.WdaRepoPath)
		}

		// Build the WebDriverAgent using xcodebuild from the provided repo path
		err = util.BuildWebDriverAgent()
		if err != nil {
			log.Fatalf("updateDevices: Could not build WebDriverAgent for testing - %s", err)
		}
	}

	// If we want to provide Android devices check if adb is available on PATH
	if config.Config.EnvConfig.ProvideAndroid {
		if !util.AdbAvailable() {
			logger.ProviderLogger.LogError("provider", "adb is not available, you need to set up the host as explained in the readme")
			fmt.Println("adb is not available, you need to set up the host as explained in the readme")
			os.Exit(1)
		}
	}

	// Try to remove potentially hanging ports forwarded by adb
	util.RemoveAdbForwardedPorts()

	// Finalize grid configuration if Selenium Grid usage enabled
	if config.Config.EnvConfig.UseSeleniumGrid {
		configureSeleniumSettings()
	}

	// If on Linux or Windows and iOS devices provision enabled check for WebDriverAgent.ipa/app
	configureWebDriverBinary(providerFolder)

	// Start a goroutine that will update devices on provider start
	go devices.Listener()

	// Start the provider
	err := startHTTPServer()
	if err != nil {
		log.Fatal("HTTP server stopped")
	}
}

func startHTTPServer() error {
	// Handle the endpoints
	r := router.HandleRequests()
	// Start periodically updating the provider data in the DB
	go updateProviderInDB()
	// Start the provider
	err := r.Run(fmt.Sprintf(":%v", config.Config.EnvConfig.Port))
	if err != nil {
		return err
	}
	return fmt.Errorf("HTTP server stopped due to an unknown reason")
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
	logLevel := flag.String("log-level", "info", "The log level of the provider app - debug, info, or error")
	nickname := flag.String("nickname", "", "The nickname of the provider")
	mongoDb := flag.String("mongo-db", "localhost:27017", "The address of the MongoDB instance")
	providerFolder := flag.String("provider-folder", ".", "The folder where logs and apps are stored")
	flag.Parse()

	//Nickname is mandatory, this is what we use to get the configuration from the DB
	if *nickname == "" {
		log.Fatal("Please provide --nickname flag")
	}

	// Print out some info on startup, maybe a flag was missed
	fmt.Printf("Current log level: %s, use the --log-level flag to change it\n", *logLevel)
	fmt.Printf("Will use `%s` as address for MongoDB instance, use the --mongo-db flag to change it\n", *mongoDb)
	fmt.Printf("Will use `%s` as provider folder, use the --provider-folder flag to change it\n", *providerFolder)

	// Remove trailing slash if provided, all code assumes it's not there
	*providerFolder, _ = strings.CutSuffix(*providerFolder, "/")
	return *logLevel, *nickname, *mongoDb, *providerFolder
}

// Check for and set up selenium jar file for creating Appium grid nodes in config
func configureSeleniumSettings() {
	seleniumJarFile := ""
	err := filepath.Walk(fmt.Sprintf("%s/conf", config.Config.EnvConfig.ProviderFolder), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.Contains(info.Name(), "selenium") && filepath.Ext(path) == ".jar" {
			seleniumJarFile = info.Name()
			return nil
		}
		return nil
	})
	if err != nil {
		return
	}
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
	ctx, cancel := context.WithCancel(db.MongoCtx())
	defer cancel()

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
		_, err := coll.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			logger.ProviderLogger.LogError("update_provider", fmt.Sprintf("Failed to upsert provider in DB - %s", err))
		}
		time.Sleep(1 * time.Second)
	}
}

package device

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/shamanec/GADS-devices-provider/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var session *r.Session

// Create a new connection to the DB
func newDBConn() {
	var err error = nil
	session, err = r.Connect(r.ConnectOpts{
		Address:  Config.EnvConfig.RethinkDB,
		Database: "gads",
	})

	if err != nil {
		panic("Could not make initial connection to RethinkDB on " + Config.EnvConfig.RethinkDB + ", make sure it is set up and running: " + err.Error())
	}

	go checkDBConnection()
}

// Check if the DB connection is alive and attempt to reconnect if not
func checkDBConnection() {
	for {
		if !session.IsConnected() {
			err := session.Reconnect()
			if err != nil {
				panic("DB is not connected and could not reestablish connection: " + err.Error())
			}
		}
		time.Sleep(2 * time.Second)
	}
}

func insertDevicesMongo() {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()

	for _, device := range Config.Devices {
		filter := bson.M{"_id": device.UDID}
		update := bson.M{
			"$set": device,
		}
		opts := options.Update().SetUpsert(true)

		_, err := util.MongoClient().Database("gads").Collection("devices").UpdateOne(util.MongoCtx(), filter, update, opts)

		if err != nil {
			util.ProviderLogger.LogError("provider", "Failed inserting device data in Mongo - "+err.Error())
		}
	}
}

func updateDevicesMongo() {
	for {
		insertDevicesMongo()
		time.Sleep(1 * time.Second)
	}
}

// Loop through the registered devices and update the health status in the DB for each device each second
func devicesHealthCheck() {
	for {
		for _, device := range Config.Devices {
			if device.Connected {
				go device.updateHealthStatusDB()
			}
		}
		time.Sleep(1 * time.Second)
	}
}

// Check Appium and WDA(for iOS) status and update the device health in DB
func (device *Device) updateHealthStatusDB() {
	allGood := false
	appiumGood := false
	wdaGood := true

	appiumGood, _ = device.appiumHealthy()

	if appiumGood && device.OS == "ios" {
		wdaGood, _ = device.wdaHealthy()
	}

	allGood = appiumGood && wdaGood

	if allGood {
		device.LastHealthyTimestamp = time.Now().UnixMilli()
		device.Healthy = true

	} else {
		device.Healthy = false
	}
}

func createMongoLogCollectionsForAllDevices() {
	db := util.MongoClient().Database("logs")
	collections, err := db.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		panic(fmt.Sprintf("Could not get the list of collection names in the `logs` database in Mongo - %s\n", err))
	}

	// Loop through the devices from the config
	// And create a collection for each device that doesn't already have one
	for _, device := range Config.Devices {
		if slices.Contains(collections, device.UDID) {
			continue
		}
		// Create capped collection options with limit of documents or 20 mb size limit
		// Seems reasonable for now, I have no idea what is a proper amount
		collectionOptions := options.CreateCollection()
		collectionOptions.SetCapped(true)
		collectionOptions.SetMaxDocuments(30000)
		collectionOptions.SetSizeInBytes(20 * 1024 * 1024)

		// Create the actual collection
		err = db.CreateCollection(util.MongoCtx(), device.UDID, collectionOptions)
		if err != nil {
			panic(fmt.Sprintf("Could not create collection for device `%s` - %s\n", device.UDID, err))
		}

		// Define an index for queries based on timestamp in ascending order
		indexModel := mongo.IndexModel{
			Keys: bson.M{"timestamp": 1},
		}

		// Add the index on the respective device collection
		_, err = db.Collection(device.UDID).Indexes().CreateOne(util.MongoCtx(), indexModel)
		if err != nil {
			panic(fmt.Sprintf("Could not add index on a capped collection for device `%s` - %s\n", device.UDID, err))
		}
	}
}

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
)

func insertDevicesMongo() {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()

	for _, device := range localDevices {
		filter := bson.M{"_id": device.Device.UDID}
		update := bson.M{
			"$set": device.Device,
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
		for _, device := range localDevices {
			if device.Device.Connected {
				go device.updateHealthStatusDB()
			}
		}
		time.Sleep(1 * time.Second)
	}
}

// Check Appium and WDA(for iOS) status and update the device health in DB
func (device *LocalDevice) updateHealthStatusDB() {
	allGood := false
	appiumGood := false
	wdaGood := true

	appiumGood, _ = device.appiumHealthy()

	if appiumGood && device.Device.OS == "ios" {
		wdaGood, _ = device.wdaHealthy()
	}

	allGood = appiumGood && wdaGood

	if allGood {
		device.Device.LastHealthyTimestamp = time.Now().UnixMilli()
		device.Device.Healthy = true

	} else {
		device.Device.Healthy = false
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
	for _, device := range localDevices {
		if slices.Contains(collections, device.Device.UDID) {
			continue
		}
		// Create capped collection options with limit of documents or 20 mb size limit
		// Seems reasonable for now, I have no idea what is a proper amount
		collectionOptions := options.CreateCollection()
		collectionOptions.SetCapped(true)
		collectionOptions.SetMaxDocuments(30000)
		collectionOptions.SetSizeInBytes(20 * 1024 * 1024)

		// Create the actual collection
		err = db.CreateCollection(util.MongoCtx(), device.Device.UDID, collectionOptions)
		if err != nil {
			panic(fmt.Sprintf("Could not create collection for device `%s` - %s\n", device.Device.UDID, err))
		}

		// Define an index for queries based on timestamp in ascending order
		indexModel := mongo.IndexModel{
			Keys: bson.M{"timestamp": 1},
		}

		// Add the index on the respective device collection
		_, err = db.Collection(device.Device.UDID).Indexes().CreateOne(util.MongoCtx(), indexModel)
		if err != nil {
			panic(fmt.Sprintf("Could not add index on a capped collection for device `%s` - %s\n", device.Device.UDID, err))
		}
	}
}

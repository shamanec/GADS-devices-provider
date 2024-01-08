package devices

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/shamanec/GADS-devices-provider/db"
	"github.com/shamanec/GADS-devices-provider/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Update all devices data in Mongo each second
func updateDevicesMongo() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		upsertDevicesMongo()
	}
}

// Upsert all devices data in Mongo
func upsertDevicesMongo() {
	for _, device := range DeviceMap {
		filter := bson.M{"_id": device.UDID}
		if device.Connected {
			device.LastUpdatedTimestamp = time.Now().UnixMilli()
		}

		update := bson.M{
			"$set": device,
		}
		opts := options.Update().SetUpsert(true)

		_, err := db.MongoClient().Database("gads").Collection("devices").UpdateOne(db.MongoCtx(), filter, update, opts)

		if err != nil {
			logger.ProviderLogger.LogError("provider", "Failed inserting device data in Mongo - "+err.Error())
		}
	}
}

func createMongoLogCollectionsForAllDevices() {
	ctx, cancel := context.WithTimeout(db.MongoCtx(), 10*time.Second)
	defer cancel()

	db := db.MongoClient().Database("logs")
	collections, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		panic(fmt.Sprintf("Could not get the list of collection names in the `logs` database in Mongo - %s\n", err))
	}

	// Loop through the devices from the config
	// And create a collection for each device that doesn't already have one
	for _, device := range DeviceMap {
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
		err = db.CreateCollection(ctx, device.UDID, collectionOptions)
		if err != nil {
			panic(fmt.Sprintf("Could not create collection for device `%s` - %s\n", device.UDID, err))
		}

		// Define an index for queries based on timestamp in ascending order
		indexModel := mongo.IndexModel{
			Keys: bson.M{"timestamp": 1},
		}

		// Add the index on the respective device collection
		_, err = db.Collection(device.UDID).Indexes().CreateOne(ctx, indexModel)
		if err != nil {
			panic(fmt.Sprintf("Could not add index on a capped collection for device `%s` - %s\n", device.UDID, err))
		}
	}
}

func createCappedCollection(dbName, collectionName string, maxDocuments, mb int64) {
	database := db.MongoClient().Database(dbName)
	collections, err := database.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		panic(fmt.Sprintf("Could not get the list of collection names in the `%s` database in Mongo - %s\n", dbName, err))
	}

	if slices.Contains(collections, collectionName) {
		return
	}

	// Create capped collection options with limit of documents or 20 mb size limit
	// Seems reasonable for now, I have no idea what is a proper amount
	collectionOptions := options.CreateCollection()
	collectionOptions.SetCapped(true)
	collectionOptions.SetMaxDocuments(maxDocuments)
	collectionOptions.SetSizeInBytes(mb * 1024 * 1024)

	// Create the actual collection
	err = database.CreateCollection(db.MongoCtx(), collectionName, collectionOptions)
	if err != nil {
		panic(fmt.Sprintf("Could not create collection `%s` - %s\n", collectionName, err))
	}
}

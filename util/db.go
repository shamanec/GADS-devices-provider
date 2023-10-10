package util

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoClient *mongo.Client
var mongoClientCtx context.Context

func NewMongoClient() {
	var err error
	connectionString := "mongodb://localhost:27017"

	// Set up a context for the connection.
	mongoClientCtx = context.TODO()

	// Create a MongoDB client with options.
	clientOptions := options.Client().ApplyURI(connectionString)
	mongoClient, err = mongo.Connect(mongoClientCtx, clientOptions)
	if err != nil {
		panic(fmt.Sprintf("Could not connect to Mongo server at `%s` - %s", connectionString, err))
	}

	go checkDBConnection()
}

func MongoClient() *mongo.Client {
	return mongoClient
}

func MongoCtx() context.Context {
	return mongoClientCtx
}

func checkDBConnection() {
	for {
		err := mongoClient.Ping(mongoClientCtx, nil)
		if err != nil {
			fmt.Println("Lost connection to MongoDB server, attempting to create a new client - " + err.Error())
			NewMongoClient()
			break
		}
		time.Sleep(1 * time.Second)
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

		_, err := MongoClient().Database("gads").Collection("devices").UpdateOne(MongoCtx(), filter, update, opts)

		if err != nil {
			ProviderLogger.LogError("provider", "Failed inserting device data in Mongo - "+err.Error())
		}
	}
}

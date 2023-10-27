package util

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoClient *mongo.Client
var mongoClientCtx context.Context

func InitMongoClient() {
	var err error
	connectionString := "mongodb://" + Config.EnvConfig.MongoDB + "/?keepAlive=true"

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

func CloseMongoConn() {
	mongoClient.Disconnect(mongoClientCtx)
}

func MongoCtx() context.Context {
	return mongoClientCtx
}

func checkDBConnection() {
	errorCounter := 0
	for {
		if errorCounter < 10 {
			time.Sleep(1 * time.Second)
			err := mongoClient.Ping(mongoClientCtx, nil)
			if err != nil {
				fmt.Println("FAILED PINGING MONGO")
				errorCounter++
				continue
			}
		} else {
			panic("Lost connection to MongoDB server for more than 10 seconds!")
		}
	}
}

func UpsertProviderMongo() {
	data := bson.M{
		"_id":                        Config.EnvConfig.ProviderNickname,
		"host_address":               Config.EnvConfig.HostAddress,
		"selenium_hub_host":          Config.AppiumConfig.SeleniumHubHost,
		"selenium_hub_port":          Config.AppiumConfig.SeleniumHubPort,
		"selenium_hub_protocol_type": Config.AppiumConfig.SeleniumHubProtocolType,
		"connect_selenium_grid":      Config.EnvConfig.ConnectSeleniumGrid,
		"devices_in_config":          len(Config.Devices),
	}
	filter := bson.M{"_id": Config.EnvConfig.ProviderNickname}
	update := bson.M{
		"$set": data,
	}

	opts := options.Update().SetUpsert(true)
	_, err := MongoClient().Database("gads").Collection("providers").UpdateOne(MongoCtx(), filter, update, opts)
	if err != nil {
		ProviderLogger.LogError("provider", "Failed registering provider data in Mongo - "+err.Error())
	}
}

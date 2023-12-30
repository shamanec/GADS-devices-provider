package util

import (
	"context"
	"fmt"
	"time"

	"github.com/shamanec/GADS-devices-provider/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoClient *mongo.Client
var mongoClientCtx context.Context

func InitMongoClient(mongo_db string) {
	var err error
	connectionString := "mongodb://" + mongo_db + "/?keepAlive=true"

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

func GetProviderFromDB(nickname string) (models.ProviderDB, error) {
	var provider models.ProviderDB
	coll := mongoClient.Database("gads").Collection("providers_new")
	filter := bson.D{{Key: "nickname", Value: nickname}}

	err := coll.FindOne(context.TODO(), filter).Decode(&provider)
	if err != nil {
		return models.ProviderDB{}, err
	}
	return provider, nil
}

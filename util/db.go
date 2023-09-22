package util

import (
	"context"
	"fmt"
	"time"

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
		panic(err)
	}

	go checkDBConnection()
}

func checkDBConnection() {
	for {
		err := mongoClient.Ping(mongoClientCtx, nil)
		if err != nil {
			fmt.Println("Lost connection to MongoDB server, attempting to create a new client - " + err.Error())
			NewMongoClient()
		}
		time.Sleep(1 * time.Second)
	}
}

func insertDB(db, collection string) {

}

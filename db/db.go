package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"slices"

	"github.com/shamanec/GADS-devices-provider/constants"
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
		log.Fatalf("Could not connect to Mongo server at `%s` - %s", connectionString, err)
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
			log.Fatal("Lost connection to MongoDB server for more than 10 seconds!")
		}
	}
}

func GetProviderFromDB(nickname string) (models.ProviderDB, error) {
	var provider models.ProviderDB
	coll := mongoClient.Database("gads").Collection("providers")
	filter := bson.D{{Key: "nickname", Value: nickname}}

	err := coll.FindOne(context.TODO(), filter).Decode(&provider)
	if err != nil {
		return models.ProviderDB{}, err
	}
	return provider, nil
}

func GetConfiguredDevices(providerName string) ([]*models.Device, error) {
	var devicesList []*models.Device
	ctx, cancel := context.WithTimeout(mongoClientCtx, 10*time.Second)
	defer cancel()

	collection := mongoClient.Database("gads").Collection("devices")
	filter := bson.D{{Key: "provider", Value: providerName}}
	cursor, err := collection.Find(ctx, filter, options.Find())
	if err != nil {
		return devicesList, fmt.Errorf("Could not get db cursor when trying to get latest configured devices info from db - %s", err)
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &devicesList); err != nil {
		return devicesList, fmt.Errorf("Could not get devices latest configured devices info from db cursor - %s", err)
	}

	if err := cursor.Err(); err != nil {
		return devicesList, fmt.Errorf("Encountered db cursor error - %s", err)
	}

	return devicesList, nil
}

func GetConfiguredDevice(udid string) (*models.Device, error) {
	var deviceInfo models.Device
	ctx, cancel := context.WithTimeout(mongoClientCtx, 10*time.Second)
	defer cancel()

	collection := mongoClient.Database("gads").Collection("devices")
	filter := bson.D{{Key: "udid", Value: udid}}

	err := collection.FindOne(ctx, filter).Decode(&deviceInfo)
	if err != nil {
		return &models.Device{}, err
	}
	return &deviceInfo, nil
}

func UpsertDeviceDB(device models.Device) error {
	update := bson.M{
		"$set": device,
	}
	coll := mongoClient.Database("gads").Collection("devices")
	filter := bson.D{{Key: "udid", Value: device.UDID}}
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(mongoClientCtx, filter, update, opts)
	if err != nil {
		return err
	}
	return nil
}

func CreateCappedCollection(dbName, collectionName string, maxDocuments, mb int64) error {

	database := MongoClient().Database(dbName)
	collections, err := database.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		return err
	}

	if slices.Contains(collections, collectionName) {
		return err
	}

	// Create capped collection options with limit of documents or 20 mb size limit
	// Seems reasonable for now, I have no idea what is a proper amount
	collectionOptions := options.CreateCollection()
	collectionOptions.SetCapped(true)
	collectionOptions.SetMaxDocuments(maxDocuments)
	collectionOptions.SetSizeInBytes(mb * 1024 * 1024)

	// Create the actual collection
	err = database.CreateCollection(MongoCtx(), collectionName, collectionOptions)
	if err != nil {
		return err
	}

	return nil
}

func CollectionExists(dbName, collectionName string) (bool, error) {
	database := MongoClient().Database(dbName)
	collections, err := database.ListCollectionNames(context.Background(), bson.M{})
	if err != nil {
		return false, err
	}

	if slices.Contains(collections, collectionName) {
		return true, nil
	}

	return false, nil
}

func AddCollectionIndex(dbName, collectionName, key string, order constants.IndexSort) error {
	// Define an index model for the respective key and index order
	indexModel := mongo.IndexModel{
		Keys: bson.M{key: order},
	}

	ctx, cancel := context.WithCancel(MongoCtx())
	defer cancel()

	db := MongoClient().Database(dbName)
	_, err := db.Collection(collectionName).Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return err
	}

	return nil
}

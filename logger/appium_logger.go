package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/shamanec/GADS-devices-provider/db"
	"github.com/shamanec/GADS-devices-provider/models"
	"go.mongodb.org/mongo-driver/mongo"
)

type AppiumLogger struct {
	localFile       *os.File
	mongoCollection *mongo.Collection
}

func (logger *AppiumLogger) Log(logData models.AppiumLog) {
	err := appiumLogToFile(logger, logData)
	if err != nil {
		fmt.Printf("Failed writing Appium log to file - %s\n", err)
	}
	err = appiumLogToMongo(logger, logData)
	if err != nil {
		fmt.Printf("Failed writing Appium log to Mongo - %s\n", err)
	}
}

func appiumLogToFile(logger *AppiumLogger, logData models.AppiumLog) error {
	jsonData, err := json.Marshal(logData)
	if err != nil {
		return err
	}

	if _, err := logger.localFile.WriteString(string(jsonData) + "\n"); err != nil {
		return err
	}

	return nil
}

func appiumLogToMongo(logger *AppiumLogger, logData models.AppiumLog) error {
	_, err := logger.mongoCollection.InsertOne(context.TODO(), logData)
	if err != nil {
		return err
	}

	return nil
}

func (logger *AppiumLogger) Close() {
	if err := logger.localFile.Close(); err != nil {
		log.Println("Error closing the log file:", err)
	}
}

func CreateAppiumLogger(logFilePath, udid string) (*AppiumLogger, error) {
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	collection := db.MongoClient().Database("appium_logs").Collection(udid)

	return &AppiumLogger{
		localFile:       file,
		mongoCollection: collection,
	}, nil
}

type AppiumDBHook struct {
	Client     *mongo.Client
	Ctx        context.Context
	DB         string
	Collection string
}

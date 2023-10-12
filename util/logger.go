package util

import (
	"context"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type CustomLogger struct {
	*log.Logger
}

var ProviderLogger *CustomLogger

func SetupLogging() {
	var err error
	ProviderLogger, err = CreateCustomLogger("./logs/provider.log", Config.EnvConfig.DevicesHost)
	if err != nil {
		panic(err)
	}
}

func (l CustomLogger) LogDebug(event_name string, message string) {
	l.WithFields(log.Fields{
		"event": event_name,
	}).Debug(message)
}

func (l CustomLogger) LogInfo(event_name string, message string) {
	l.WithFields(log.Fields{
		"event": event_name,
	}).Info(message)
}

func (l CustomLogger) LogError(event_name string, message string) {
	l.WithFields(log.Fields{
		"event": event_name,
	}).Error(message)
}

func (l CustomLogger) LogWarn(event_name string, message string) {
	l.WithFields(log.Fields{
		"event": event_name,
	}).Warn(message)
}

func (l CustomLogger) LogFatal(event_name string, message string) {
	l.WithFields(log.Fields{
		"event": event_name,
	}).Fatal(message)
}

func (l CustomLogger) LogPanic(event_name string, message string) {
	l.WithFields(log.Fields{
		"event": event_name,
	}).Panic(message)
}

func CreateCustomLogger(logFilePath, collection string) (*CustomLogger, error) {
	// Create a new logger instance
	logger := log.New()

	// Configure the logger
	logger.SetFormatter(&log.JSONFormatter{})
	logger.SetLevel(log.DebugLevel)

	// Open the log file
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		return &CustomLogger{}, fmt.Errorf("Could not set log output - %v", err)
	}

	// Set the output to the log file
	logger.SetOutput(logFile)

	logger.AddHook(&MongoDBHook{
		Client:     mongoClient,
		DB:         "logs",
		Collection: collection,
		Ctx:        mongoClientCtx,
	})

	return &CustomLogger{Logger: logger}, nil
}

type MongoDBHook struct {
	Client     *mongo.Client
	Ctx        context.Context
	DB         string
	Collection string
}

type logEntry struct {
	Level     string
	Message   string
	Timestamp int64
	Host      string
	EventName string
}

func (hook *MongoDBHook) Fire(entry *log.Entry) error {
	fields := entry.Data

	logEntry := logEntry{
		Level:     entry.Level.String(),
		Message:   entry.Message,
		Timestamp: time.Now().UnixMilli(),
		Host:      Config.EnvConfig.ProviderNickname,
		EventName: fields["event"].(string),
	}

	document, err := bson.Marshal(logEntry)
	if err != nil {
		fmt.Println("Failed marshalling log entry inserting provider log through hook - " + err.Error())
	}

	_, err = hook.Client.Database(hook.DB).Collection(hook.Collection).InsertOne(hook.Ctx, document)
	if err != nil {
		fmt.Println("Failed inserting provider log through hook - " + err.Error())
	}

	return err
}

// Levels returns the log levels at which the hook should fire
func (hook *MongoDBHook) Levels() []log.Level {
	return log.AllLevels
}

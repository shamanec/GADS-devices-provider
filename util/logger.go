package util

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func SetLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	projectLogFile, err := os.OpenFile("./logs/provider.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		panic("Could not set log output: " + err.Error())
	}
	log.SetLevel(log.DebugLevel)
	log.SetOutput(projectLogFile)
}

func LogDebug(event_name string, message string) {
	log.WithFields(log.Fields{
		"event": event_name,
	}).Debug(message)
}

func LogInfo(event_name string, message string) {
	log.WithFields(log.Fields{
		"event": event_name,
	}).Info(message)
}

func LogError(event_name string, message string) {
	log.WithFields(log.Fields{
		"event": event_name,
	}).Error(message)
}

func LogWarn(event_name string, message string) {
	log.WithFields(log.Fields{
		"event": event_name,
	}).Warn(message)
}

func LogFatal(event_name string, message string) {
	log.WithFields(log.Fields{
		"event": event_name,
	}).Fatal(message)
}

func LogPanic(event_name string, message string) {
	log.WithFields(log.Fields{
		"event": event_name,
	}).Panic(message)
}

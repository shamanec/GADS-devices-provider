package device

import (
	"sync"
	"time"

	"github.com/shamanec/GADS-devices-provider/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var session *r.Session

// Create a new connection to the DB
func newDBConn() {
	var err error = nil
	session, err = r.Connect(r.ConnectOpts{
		Address:  Config.EnvConfig.RethinkDB,
		Database: "gads",
	})

	if err != nil {
		panic("Could not make initial connection to RethinkDB on " + Config.EnvConfig.RethinkDB + ", make sure it is set up and running: " + err.Error())
	}

	go checkDBConnection()
}

// Check if the DB connection is alive and attempt to reconnect if not
func checkDBConnection() {
	for {
		if !session.IsConnected() {
			err := session.Reconnect()
			if err != nil {
				panic("DB is not connected and could not reestablish connection: " + err.Error())
			}
		}
		time.Sleep(2 * time.Second)
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
		for _, device := range Config.Devices {
			if device.Connected {
				go device.updateHealthStatusDB()
			}
		}
		time.Sleep(1 * time.Second)
	}
}

// Check Appium and WDA(for iOS) status and update the device health in DB
func (device *Device) updateHealthStatusDB() {
	allGood := false
	appiumGood := false
	wdaGood := true

	appiumGood, _ = device.appiumHealthy()

	if appiumGood && device.OS == "ios" {
		wdaGood, _ = device.wdaHealthy()
	}

	allGood = appiumGood && wdaGood

	if allGood {
		device.LastHealthyTimestamp = time.Now().UnixMilli()
		device.Healthy = true

	} else {
		device.Healthy = false
	}
}

package device

import (
	"time"

	log "github.com/sirupsen/logrus"
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

// Insert/update the registered devices from config.json to the DB
// when starting the provider
func insertDevicesDB() error {
	for _, device := range Config.Devices {
		// Check if data for the device by UDID already exists in the table
		cursor, err := r.Table("devices").Get(device.UDID).Run(session)
		if err != nil {
			return err
		}

		// If there is no data for the device with this UDID
		// Insert the data into the table
		if cursor.IsNil() {
			err = r.Table("devices").Insert(device).Exec(session)
			if err != nil {
				log.WithFields(log.Fields{
					"event": "insert_db",
				}).Error("Inserting device in DB failed: " + err.Error())
			}
			continue
		}
		cursor.Close()

		// If data for the device with this UDID exists in the DB
		// Update it with the latest info
		device.updateDB()
	}

	return nil
}

// Update the respective device document in the DB
func (device *Device) updateDB() {
	err := r.Table("devices").Update(device).Exec(session)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "update_db",
		}).Error("Updating device in DB failed: " + err.Error())
	}
}

// Loop through the registered devices and update the health status in the DB for each device each second
func devicesHealthCheck() {
	for {
		for _, device := range Config.Devices {
			if device.Connected == true {
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
		device.updateDB()

	} else {
		device.Healthy = false
		device.updateDB()
	}
}

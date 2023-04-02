package device

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var session *r.Session

func NewDBConn() {
	var err error = nil
	session, err = r.Connect(r.ConnectOpts{
		Address:  Config.EnvConfig.RethinkDB,
		Database: "gads",
	})

	if err != nil {
		panic("Could not make initial connection to db on " + Config.EnvConfig.RethinkDB + ", err: " + err.Error())
	}

	go checkDBConnection()
}

func checkDBConnection() {
	for {
		if !session.IsConnected() {
			err := session.Reconnect()
			if err != nil {
				panic("DB is not connected and could not reestablish connection, err: " + err.Error())
			}
		}
		time.Sleep(2 * time.Second)
	}
}

// Insert/update the registered devices from config.json to the DB
// when starting the provider
func InsertDevicesDB() error {
	for _, device := range Config.Devices {
		// Check if data for the device by UDID already exists in the table
		fmt.Println(session)
		cursor, err := r.Table("devices").Get(device.UDID).Run(session)
		if err != nil {
			fmt.Println("HERE")
			return err
		}
		defer cursor.Close()

		// If there is no data for the device with this UDID
		// Insert the data into the table
		if cursor.IsNil() {
			err = r.Table("devices").Insert(device).Exec(session)
			if err != nil {
				log.WithFields(log.Fields{
					"event": "insert_db",
				}).Error("Insert db fail: " + err.Error())
			}
			continue
		}

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
			"event": "insert_db",
		}).Error("Update db fail: " + err.Error())
	}
}

// Get the current State of the device from the DB
func (device *Device) getStateDB() string {
	cursor, err := r.Table("devices").Get(device.UDID).Field("State").Run(session)
	if err != nil {
		fmt.Println("Could not get device state in DB, err: " + err.Error())
	}
	defer cursor.Close()

	var dbState string
	err = cursor.One(&dbState)
	if err != nil {
		fmt.Println("Could not get device state in DB, err: " + err.Error())
	}

	return dbState
}

// Loop through the registered devices and update the health status in the DB for each device each second
func devicesHealthCheck() {
	for {
		for _, device := range Config.Devices {
			go device.updateHealthStatusDB()
		}
		time.Sleep(1 * time.Second)
	}
}

// Check Appium and WDA(for iOS) status and update the device health in DB
func (device *Device) updateHealthStatusDB() {
	allGood := false
	appiumGood := false
	wdaGood := true

	if device.Connected {
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
}

package device

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var session *r.Session

func NewDBConn(address string) {
	var err error = nil
	session, err = r.Connect(r.ConnectOpts{
		Address:  address,
		Database: "gads",
	})

	if err != nil {
		panic("Could not make initial connection to db on " + address + ", err: " + err.Error())
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
		err = r.Table("devices").Update(device).Exec(session)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "insert_db",
			}).Error("Update db fail: " + err.Error())
		}
	}

	//go devicesHealthCheck()

	return nil
}

func (device *Device) updateDB() {
	err := r.Table("devices").Update(device).Exec(session)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "insert_db",
		}).Error("Update db fail: " + err.Error())
	}
}

func (device *Device) updateStateDB(state string) {
	dbState := device.getStateDB()

	if dbState != state {
		device.State = state
		err := r.Table("devices").Update(device).Exec(session)
		if err != nil {
			log.WithFields(log.Fields{
				"event": "insert_db",
			}).Error("Update db fail: " + err.Error())
		}
		return
	}
}

func (device *Device) updateConnectedDB(connected bool) {
	device.Connected = connected

	err := r.Table("devices").Update(device).Exec(session)

	if err != nil {
		log.WithFields(log.Fields{
			"event": "insert_db",
		}).Error("Update db fail: " + err.Error())
	}
}

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

func (device *Device) getHealthStatusDB() bool {
	cursor, err := r.Table("devices").Get(device.UDID).Field("Healthy").Run(session)
	if err != nil {
		fmt.Println("Could not get device health status in DB, err: " + err.Error())
	}
	defer cursor.Close()

	var healthy bool
	err = cursor.One(&healthy)
	if err != nil {
		fmt.Println("Could not get device health status in DB, err: " + err.Error())
	}

	return healthy
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
	var err error = nil

	if device.Connected {
		appiumGood, _ = device.appiumHealthy()

		if appiumGood && device.OS == "ios" {
			wdaGood, _ = device.wdaHealthy()
		}

		allGood = appiumGood && wdaGood

		if allGood {
			device.LastHealthyTimestamp = time.Now().UnixMilli()
			device.Healthy = true
			err = r.Table("devices").Update(device).Exec(session)

			if err != nil {
				log.WithFields(log.Fields{
					"event": "insert_db",
				}).Error("Update db fail: " + err.Error())
			}

		} else {
			device.Healthy = false
			err = r.Table("devices").Update(device).Exec(session)
			if err != nil {
				log.WithFields(log.Fields{
					"event": "insert_db",
				}).Error("Update db fail: " + err.Error())
			}
		}
	}
}

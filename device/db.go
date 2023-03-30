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
		panic("Could not connect to db on " + address + ", err: " + err.Error())
	}
}

func InsertDevicesDB() error {
	for _, device := range Config.Devices {
		// Check if data for the device by UDID already exists in the table
		cursor, err := r.Table("devices").Get(device.UDID).Run(session)
		if err != nil {
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

	return nil
}

func (device *Device) updateDB() {
	device.LastUpdateTimestamp = time.Now().UnixMilli()

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
		device.LastUpdateTimestamp = time.Now().UnixMilli()
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
	device.LastUpdateTimestamp = time.Now().UnixMilli()

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

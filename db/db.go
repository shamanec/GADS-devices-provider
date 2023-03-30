package db

import (
	"github.com/shamanec/GADS-devices-provider/device"
	log "github.com/sirupsen/logrus"
	r "gopkg.in/rethinkdb/rethinkdb-go.v6"
)

var session *r.Session

func New(address string) {
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
	for _, device := range device.Config.Devices {
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

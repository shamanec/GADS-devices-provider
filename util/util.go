package util

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
)

// Convert an interface{}(struct) into an indented JSON string
func ConvertToJSONString(data interface{}) (string, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "convert_interface_to_json",
		}).Error("Could not marshal interface to json: " + err.Error())
		return "", err
	}
	return string(b), nil
}

// Unmarshal provided JSON string into a struct
func UnmarshalJSONString(jsonString string, v interface{}) error {
	bs := []byte(jsonString)

	err := json.Unmarshal(bs, v)
	if err != nil {
		return err
	}

	return nil
}

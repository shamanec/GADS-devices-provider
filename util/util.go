package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/shamanec/GADS-devices-provider/models"
	log "github.com/sirupsen/logrus"
)

var ProjectDir string
var Config models.ConfigJsonData
var mu sync.Mutex
var usedPorts = make(map[int]bool)

// Convert an interface{}(struct) into an indented JSON string
func ConvertToJSONString(data interface{}) (string, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
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

func SetupConfig() {
	var err error
	ProjectDir, err = os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Could not get project dir with os.Getwd() - %s", err))
	}

	err = getConfigJsonData()
	if err != nil {
		panic(fmt.Sprintf("Could not get config data from config.json - %s", err))
	}
}

func GetFreePort() (port int, err error) {
	mu.Lock()
	defer mu.Unlock()

	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			port = l.Addr().(*net.TCPAddr).Port
			if _, ok := usedPorts[port]; ok {
				return GetFreePort()
			}
			usedPorts[port] = true
			return port, nil
		}
	}
	return
}

// Read the config.json file and initialize the configuration struct
func getConfigJsonData() error {
	bs, err := getConfigJsonBytes()
	if err != nil {
		return err
	}

	err = json.Unmarshal(bs, &Config)
	if err != nil {
		return err
	}

	return nil
}

// Read the config.json file into a byte slice
func getConfigJsonBytes() ([]byte, error) {
	jsonFile, err := os.Open("./config/config.json")
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not open config file: " + err.Error())
		return nil, err
	}
	defer jsonFile.Close()

	bs, err := io.ReadAll(jsonFile)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_config_data",
		}).Error("Could not read config file to byte slice: " + err.Error())
		return nil, err
	}

	return bs, err
}

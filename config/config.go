package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/shamanec/GADS-devices-provider/db"
	"github.com/shamanec/GADS-devices-provider/models"
	log "github.com/sirupsen/logrus"
)

var Config models.ConfigJsonData

func SetupConfig(nickname, folder string) {
	err := getConfigJsonData()
	if err != nil {
		panic(fmt.Sprintf("Could not get config data from config.json - %s", err))
	}

	provider, _ := db.GetProviderFromDB(nickname)
	if (provider == models.ProviderDB{}) {
		panic("Provider with this nickname is not registered in the DB")
	}
	// Config.EnvConfig.ProviderFolder = folder
	provider.ProviderFolder = folder
	Config.EnvConfig = provider
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

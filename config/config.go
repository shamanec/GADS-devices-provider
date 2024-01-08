package config

import (
	"github.com/shamanec/GADS-devices-provider/db"
	"github.com/shamanec/GADS-devices-provider/models"
)

var Config models.ConfigJsonData

func SetupConfig(nickname, folder string) {
	provider, err := db.GetProviderFromDB(nickname)
	if err != nil {
		panic("Could not get provider data from DB")
	}
	if (provider == models.ProviderDB{}) {
		panic("Provider with this nickname is not registered in the DB")
	}
	provider.ProviderFolder = folder
	Config.EnvConfig = provider
}

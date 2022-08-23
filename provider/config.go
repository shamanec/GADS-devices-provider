package provider

import (
	"flag"
	"os"

	"github.com/shamanec/GADS-devices-provider/util"
)

var ProviderPort string
var HomeDir string
var ProjectDir string
var err error
var ConfigData util.ConfigJsonData

func SetupConfig() {
	HomeDir, err = os.UserHomeDir()
	if err != nil {
		panic("Could not get home dir: " + err.Error())
	}

	ProjectDir, err = os.Getwd()
	if err != nil {
		panic("Could not get project dir: " + err.Error())
	}

	port_flag := flag.String("port", "10001", "The port to run the server on")
	flag.Parse()

	ConfigData, err = util.GetConfigJsonData()
	if err != nil {
		panic("Could not get config data from config.json: " + err.Error())
	}

	ProviderPort = *port_flag
}

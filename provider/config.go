package provider

import (
	"flag"
	"os"
)

var ProviderPort string
var HomeDir string
var ProjectDir string
var err error

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

	ProviderPort = *port_flag
}

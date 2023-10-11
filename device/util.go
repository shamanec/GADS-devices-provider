package device

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

// Get all the connected devices to the host by reading the symlinks in /dev
func getConnectedDevices() ([]string, error) {
	// Get all files/symlinks/folders in /dev
	var connectedDevices []string = []string{}
	devFiles, err := filepath.Glob("/dev/*")
	if err != nil {
		fmt.Println("Error listing files in /dev:", err)
		return nil, err
	}

	for _, devFile := range devFiles {
		// Split the devFile to get only the file name
		_, fileName := filepath.Split(devFile)
		// If the filename is a device symlink
		// Add it to the connected devices list
		if strings.Contains(fileName, "device") {
			connectedDevices = append(connectedDevices, fileName)
		}
	}

	return connectedDevices, nil
}

var cli *client.Client

// Create a docker client singleton to be used by the provider
// This avoids exhausting docker socket connections and also makes code cleaner
// Might be changed in the future if this becomes a problem
func initDockerClient() error {
	var err error
	cli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_host_containers",
		}).Error(". Error: " + err.Error())
		return err
	}

	return nil
}

// Get list of all containers on host
func getHostContainers() ([]types.Container, error) {
	if cli == nil {
		err := initDockerClient()
		if err != nil {
			return []types.Container{}, err
		}
	}

	// Get the list of containers
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		log.WithFields(log.Fields{
			"event": "get_host_containers",
		}).Error(". Error: " + err.Error())
		return nil, errors.New("Could not get container list: " + err.Error())
	}
	return containers, nil
}

func getFreePort() (port int, err error) {
	mu.Lock()
	defer mu.Unlock()

	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			port = l.Addr().(*net.TCPAddr).Port
			if _, ok := usedPorts[port]; ok {
				return getFreePort()
			}
			usedPorts[port] = true
			return port, nil
		}
	}
	return
}

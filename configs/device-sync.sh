#!/bin/bash
# Enable nvm, because Appium will not be available to bash (could handle later in Go)
export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"

# Start the container server and output the logs
container-server > "/opt/logs/container-server.log" 2>&1
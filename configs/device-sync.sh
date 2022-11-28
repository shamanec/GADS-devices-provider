#!/bin/bash
export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
container-server > "/opt/logs/container-server.log" 2>&1
#!/bin/bash

# Hit the Appium status URL to see if it is available and start it if not
check-appium-status() {
  if ! curl -Is "http://localhost:4723/status" | head -1 | grep -q '200 OK'; then
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] Appium is not running, starting.."
    start-appium
  fi
}

# Start appium server for the device
# If the device is on Selenium Grid use created nodeconfig.json, if not - skip applying it in the command
start-appium() {
  if [ ${ON_GRID} == "true" ]; then
    appium -p 4723 \
      --log-timestamp \
      --allow-cors \
      --session-override \
      --allow-insecure chromedriver_autodownload \
      --default-capabilities \
      '{"appium:udid": "'$DEVICE_UDID'", "appium:automationName":"UiAutomator2", "platformName": "Android", "appium:deviceName": "'${DEVICE_NAME}'"}' \
      --nodeconfig /opt/nodeconfig.json >>/opt/logs/appium-logs.log 2>&1 &
  else
    appium -p 4723 \
      --log-timestamp \
      --allow-cors \
      --session-override \
      --allow-insecure chromedriver_autodownload \
      --default-capabilities \
      '{"appium:udid": "'$DEVICE_UDID'", "appium:automationName":"UiAutomator2", "platformName": "Android", "appium:deviceName": "'${DEVICE_NAME}'"}' >>/opt/logs/appium-logs.log 2>&1 &
  fi
}

# Validate device is available to adb, kill the container if not
check-device-available() {
  available="false"
  for i in {1..3}; do
    if ! adb devices | grep -q "$DEVICE_UDID"; then 
      sleep 3
    else
      available="true"
      break
    fi
  done

  if [ $available == "false" ]; then
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] Device did not become available to adb in 10 seconds"
    exit -1
  fi
}

# Forward minicap socket to local tcp port 1313
forward-minicap() {
  adb forward tcp:1313 localabstract:minicap
  sleep 2
}

# MAIN SCRIPT
export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"

if [ ${ON_GRID} == "true" ]; then
  /opt/nodeconfiggen.sh > /opt/nodeconfig.json
fi

sleep 2
check-device-available

# Don't attempt to run minicap if there will be no remote control
if [ ${REMOTE_CONTROL} == "true" ]; then
  touch /opt/logs/minicap.log
  STREAM_WIDTH=""
  STREAM_HEIGHT=""
  # If you want higher fps and have provided MINICAP_HALF_RESOLUTION true, minicap will run at half the original device resolution
  if [ "${MINICAP_HALF_RESOLUTION}" == "true" ]; then
    STREAM_WIDTH=$((${SCREEN_WIDTH} / 2))
    STREAM_HEIGHT=$((${SCREEN_HEIGHT} / 2))
  else
    # Or at the original device resolution
    STREAM_WIDTH=${SCREEN_WIDTH}
    STREAM_HEIGHT=${SCREEN_HEIGHT}
  fi
  # If no specific FPS is specified run it at whatever minicap provides
  # Else try to run at the specified FPS
  if [ "${MINICAP_FPS}" == "" ]; then
    cd /root/minicap/ && ./run.sh -P ${SCREEN_SIZE}@${STREAM_WIDTH}x${STREAM_HEIGHT}/0 >>/opt/logs/minicap.log 2>&1 &
  else
    cd /root/minicap/ && ./run.sh -r ${MINICAP_FPS} -S -P ${SCREEN_SIZE}@${STREAM_WIDTH}x${STREAM_HEIGHT}/0 >>/opt/logs/minicap.log 2>&1 &
  fi
  forward-minicap
fi

container-server 2>&1 &

touch /opt/logs/appium-logs.log

while true; do
  check-appium-status
  check-device-available
  sleep 10
done
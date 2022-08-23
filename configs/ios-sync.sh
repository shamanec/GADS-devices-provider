#!/bin/bash

# Start WebDriverAgent on the device by provided bundleID
start-wda-go-ios() {
  echo "[$(date +'%d/%m/%Y %H:%M:%S')] Starting WebDriverAgent application on default port 8100 and mjpeg stream default port 9100.."
  ios runwda --bundleid=$WDA_BUNDLEID --testrunnerbundleid=$WDA_BUNDLEID --xctestconfig=WebDriverAgentRunner.xctest --udid $DEVICE_UDID > "/opt/logs/wda-logs.log" 2>&1 &
  sleep 2
}


# Install WebDriverAgent on the device from the provided IPA file
install-wda() {
  echo "[$(date +'%d/%m/%Y %H:%M:%S')] Installing WebDriverAgent application on device.."
  ios install --path=/opt/WebDriverAgent.ipa --udid=$DEVICE_UDID
}

# Start the WebDriverAgent service on the device
start-wda() {
  install-wda
  start-wda-go-ios

  # Check if WDA is up and running for 20 seconds
  wda_up="false"
  for i in {1..10}; do
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] Checking if WebDriverAgent is started.."
    if ! curl -Is "http://localhost:8100/status" | head -1 | grep -q '200 OK'; then
      sleep 2
    else
      # If WebDriverAgent is running break out of the loop
      wda_up="true" 
      break
    fi
  done

  # If WebDriverAgent was not successfully started kill the container by exiting the script
  if [ $wda_up == "false" ]; then
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] Could not start WebDriverAgent for 20 seconds"
    exit -1
  fi
}

# Hit WebDriverAgent status URL and if service not available start it again
check-wda-status() {
  if ! curl -Is "http://localhost:8100/status" | head -1 | grep -q '200 OK'; then
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] WebDriverAgent is not running, attempting to start/restart WebDriverAgent.."
    start-wda
    update-wda-stream-settings
  else
    sleep 10
  fi
}

update-wda-stream-settings() {
  echo "[$(date +'%d/%m/%Y %H:%M:%S')] Updating WebDriverAgent stream settings.."
  # Create a dummy session and get the ID
  sessionID=$(curl --silent --location --request POST "http://localhost:8100/session" --header 'Content-Type: application/json' --data-raw '{"capabilities": {"waitForQuiescence": false}}' | jq -r '.sessionId')
  # Update the stream settings of the session
  curl -s --location --request POST "http://localhost:8100/session/$sessionID/appium/settings" --header 'Content-Type: application/json' --data-raw '{"settings": {"mjpegServerFramerate": 30, "mjpegServerScreenshotQuality": 50, "mjpegScalingFactor": 100}}' -o /dev/null
}

# Hit the Appium status URL to see if it is available and start it if not
check-appium-status() {
  if ! curl -Is "http://localhost:4723/status" | head -1 | grep -q '200 OK'; then
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] Appium server is not running, starting.."
    start-appium
  fi
}

# Start appium server for the device
# If the device is on Selenium Grid use created nodeconfig.json, if not - skip applying it in the command
start-appium() {
  if [ ${ON_GRID} == "true" ]; then
    appium -p 4723 --udid "$DEVICE_UDID" \
      --log-timestamp \
      --allow-cors \
      --session-override \
      --default-capabilities \
      '{"appium:mjpegServerPort": "9100", "appium:clearSystemFiles": "false", "appium:webDriverAgentUrl":"http://localhost:8100", "appium:preventWDAAttachments": "true", "appium:simpleIsVisibleCheck": "false", "appium:wdaLocalPort": "8100", "appium:platformVersion": "'${DEVICE_OS_VERSION}'", "appium:automationName":"XCUITest", "platformName": "iOS", "appium:deviceName": "'${DEVICE_NAME}'", "appium:wdaLaunchTimeout": "120000", "appium:wdaConnectionTimeout": "240000"}' \
      --nodeconfig /opt/nodeconfig.json >>"/opt/logs/appium-logs.log" 2>&1 &
  else
    appium -p 4723 --udid "$DEVICE_UDID" \
      --log-timestamp \
      --allow-cors \
      --session-override \
      --default-capabilities \
      '{"appium:mjpegServerPort": "9100", "appium:clearSystemFiles": "false", "appium:webDriverAgentUrl":"http://localhost:8100",  "appium:preventWDAAttachments": "true", "appium:simpleIsVisibleCheck": "false", "appium:wdaLocalPort": "8100", "appium:platformVersion": "'${DEVICE_OS_VERSION}'", "appium:automationName":"XCUITest", "platformName": "iOS", "appium:deviceName": "'${DEVICE_NAME}'", "appium:wdaLaunchTimeout": "120000", "appium:wdaConnectionTimeout": "240000"}' >>"/opt/logs/appium-logs.log" 2>&1 &
  fi
}

# Mount the respective Apple Developer Disk Image for the current device OS version
# Skip mounting images if they are already mounted
mount-disk-images() {
  if ios image list --udid=$DEVICE_UDID 2>&1 | grep "none"; then
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] Could not find Developer disk images on the device, mounting.."
    ios image auto --basedir=/opt/DeveloperDiskImages --udid=$DEVICE_UDID
  else
    echo "[$(date +'%d/%m/%Y %H:%M:%S')] Developer disk images are already mounted on the device, nothing to do"
  fi
}

# Pair device using the supervision identity
pair-device() {
  echo "[$(date +'%d/%m/%Y %H:%M:%S')] Pairing supervised device.."
  ios pair --p12file="/opt/supervision.p12" --password="${SUPERVISION_PASSWORD}" --udid="${DEVICE_UDID}"
}

# Forward the WebDriverAgent port and the WebDriverAgent mjpeg stream port to the container
forward-wda() {
  echo "[$(date +'%d/%m/%Y %H:%M:%S')] Forwarding WebDriverAgent port and stream port to container.."
  ios forward 8100 8100 --udid=$DEVICE_UDID 2>&1 &
  ios forward 9100 9100 --udid=$DEVICE_UDID 2>&1 &
}

# MAIN SCRIPT

# Activate nvm
# TODO: Revise if needed
export NVM_DIR="$HOME/.nvm" && [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"

# Only generate nodeconfig.json if the device will be registered on Selenium Grid
if [ ${ON_GRID} == "true" ]; then
  ./opt/nodeconfiggen.sh > /opt/nodeconfig.json
fi

if [ ${CONTAINERIZED_USBMUXD} == "true" ]; then
  # Start usbmuxd inside the container
  usbmuxd -U usbmux -f 2>&1 &
  echo "[$(date +'%d/%m/%Y %H:%M:%S')] Waiting 5 seconds after starting usbmuxd before attempting to pair device.."
  sleep 5

  # Pairing using supervision identity should be required only when usbmuxd is containerized
  pair-device

else
  sleep 2
fi

mount-disk-images
sleep 2

forward-wda
sleep 1

container-server 2>&1 &

while true; do
  check-wda-status
  check-appium-status
done

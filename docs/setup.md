# Provider setup  
Currently the project assumes that GADS UI, RethinkDB and device providers are on the same network. They can all be on the same machine as well.  
The provider supports Linux and MacOS. On Linux Android devices are fully supported and iOS <17 devices are fully supported. On MacOS also iOS >= 17 devices are supported.  

## Dependencies  
### Linux
#### Docker
1. Install Docker.  

#### Go
2. Install Go 1.21 or higher  

#### Android debug bridge
1. Install `adb` (Android debug bridge). It should be available in PATH so it can be directly accessed via Terminal  

#### Usbmuxd
1. Install usbmuxd - `sudo apt install usbmuxd` on the host if you want to see the device UDIDs with `go-ios` or similar tools

### MacOS
#### Xcode
1. Install latest stable Xcode release - for iOS 17 install latest beta release
2. Install command line tools with `xcode-select --install`

#### Android debug bridge
1. Install `adb` (Android debug bridge). It should be available in PATH so it can be directly accessed via Terminal

#### Set up go-ios
1. Download the latest release of [go-ios](https://github.com/danielpaulus/go-ios) and unzip it
2. Add it to `/usr/local/bin` with `sudo cp ios /usr/local/bin`

#### WebDriverAgent
1. Download the latest release of [WebDriverAgent](https://github.com/appium/WebDriverAgent/releases)
2. Unzip the source code in any folder.
3. Open WebDriverAgent.xcodeproj in Xcode
4. Select signing profiles for WebDriverAgentLib and WebDriverAgentRunner.
5. Build the WebDriverAgent and run on a device at least once to validate it builds and runs as expected.

#### GADS Android stream
1. Download the latest release of [GADS-Android-stream](https://github.com/shamanec/GADS-Android-stream/releases)
2. Unzip the `*.apk` file and put it in `apps` folder (in the GADS-devices-provider repo)

### RethinkDB - Linux, MacOS
The project uses RethinkDB for syncing devices availability between providers and GADS UI. You need to have RethinkDB running and set up as explained in the [GADS](https://github.com/shamanec/GADS) readme before running the provider.  
1. Open `config.json`  
2. Update the `rethink_db` value in `env-config` with the IP address of the machine running the RethinkDB instance and the port on which it is accepting connections. The default port if you followed the setup would be `32771`. Example: `192.168.1.2:32771`  

## Configuration setup
### Environment config
#### Common config - Linux, MacOS
~1. Set `connect_selenium_grid` in `env-config` to `false` or `true` - currently Selenium Grid connection not working~
2. Set `devices_host` in `env-config` to the IP of the provider host, e.g. `192.168.1.16` or whatever it is on your network  
3. Set `rethink_db` in `env-config` to the IP and port of the RethinkDB instance, eg.g `192.168.1.6:32771` if you followed the steps above.  

#### MacOS specific config
1. Set `supervision_password` in `env-config` to the password for your supervised devices certificate - supervision setup can be found below.  
2. Set `wda_repo_path` in `env-config` to the folder where WebDriverAgent was downloaded from Github, e.g. `/Users/shamanec/Downloads/WebDriverAgent-5.8.3/` 
  * When the provider is started it will use this path to build WebDriverAgent with `xcodebuild build-for-testing` once and then will run WebDriverAgent on each device with `xcodebuild test-without-building`. When `go-ios` starts supporting iOS >= 17 then the approach will be changed with prebuilt WebDriverAgent to spend less resources than with `xcodebuild`

#### Linux specific config
1. Set `wda_bundle_id` in `env-config` to the bundle ID used to prebuild WebDriverAgent.

### Appium config - currently hosts only Selenium Grid related data and that does NOT work
1. Set `selenium_hub_host` to the IP address of the Selenium Grid instance  
2. Set `selenium_hub_port` to the port of the Selenium Grid instance  
3. Set `selenium_hub_protocol_type` to `http` or `https` depending on the Selenium Grid instance  

### Devices config
Each device should have a JSON object in `devices-config` like:
```
{
      "os": "ios",
      "name": "iPhone_11",
      "os_version": "17.0",
      "udid": "00008030000418C136FB8022",
      "screen_size": "375x667",
      "model": "iPhone 11"
}
```
For each device set: 
  * `os` - should be `android` or `ios`
  * `screen_size`- e.g. `375x667` where first value is width, second is height
    * For Android - go to `https://whatismyandroidversion.com` and fill in the displayed `Screen size`, not `Viewport size`  
    * For iOS - you can get it on https://whatismyviewport.com (ScreenSize: at the bottom)   
  * `os_version` - `11` or `13.5.1` for example  
  * `name` - avoid using special characters and spaces except '_'. Example: `Huawei_P20_Pro`, `iPhone_11`  
  * `udid` - UDID of the Android or iOS device
    * For Android can get it with `adb devices`
    * For iOS can get it with Xcode through `Devices & Simulator` or using `go-ios` or a similar tool (tidevice, gidevice, pymobiledevice3)
  * `model` - device model to be displayed in [GADS](https://github.com/shamanec/GADS) device selection.  

## Setup - Linux  
### Build iOS Docker image
1. Cd into the project folder  
2. Execute `docker build -f Dockerfile-iOS -t ios-appium .`  

### Build Android Docker image
1. Cd into the project folder  
2. Execute `docker build -f Dockerfile-Android -t android-appium .`  

### Setup udev rules
**NB** Before this step you need to register your devices in `config.json` according to [Devices setup](#devices-setup)  
1. Execute `curl -X POST http://localhost:{ProviderPort}/device/create-udev-rules`  
2. Copy the newly created `90-device.rules` file in the project folder to `/etc/udev/rules.d/` - `sudo cp 90-device.rules /etc/udev/rules/`  
3. Execute `sudo udevadm control --reload-rules` or restart the machine    

**NB** You need to perform this step each time you add a new device to `config.json` so that the symlink for that respective device is properly created in `/dev`  

## Additional setup
### Android setup
1. You should have `adb` already installed on the host
2. Enable `USB Debugging` for each Android device through the developer tools.  
3. Connect the devices to the host and authorize the USB debugging - this will create pairing keys. Also on Linux these pairing keys will be mounted to the containes so you don't have to authorize each time a container is created (since its essentially a new host)

### Kill adb-server - LINUX ONLY
1. You need to make sure that adb-server is not running on the Linux host before you start devices containers.  
2. Run `adb kill-server`.  

## Running the provider
1. Execute `go build .` and `./GADS-devices-provider`  
2. You can also use `./GADS-devices-provider -port={PORT}` to run the provider on a selected port, the default port without the flag is 10001 - might not work, use at your own risk :D

### iOS setup
### Dependencies
1. Install usbmuxd - `sudo apt install usbmuxd`  

### Known limitations
1. It is not possible to execute **driver.executeScript("mobile: startPerfRecord")** with Appium to record application performance since Xcode tools are not available.  
2. Anything else that might need Instruments and/or any other Xcode/OSX exclusive tools  

### Prepare WebDriverAgent file

You need an Apple Developer account to build and sign `WebDriverAgent`

1. Download and install [iOS App Signer](https://dantheman827.github.io/ios-app-signer/)  
2. Open `WebDriverAgent.xcodeproj` in Xcode.  
3. Ensure a team is selected before building the application. To do this go to: *Targets* and select each target one at a time. There should be a field for assigning teams certificates to the target.  
4. Remove your `WebDriverAgent` folder from `DerivedData` and run `Clean build folder` (just in case)  
5. Next build the application by selecting the `WebDriverAgentRunner` target and build for `Generic iOS Device`. Run `Product => Build for testing`. This will create a `Products/Debug-iphoneos` in the specified project directory.  
 `Example`: **/Users/<username>/Library/Developer/Xcode/DerivedData/WebDriverAgent-dzxbpamuepiwamhdbyvyfkbecyer/Build/Products/Debug-iphoneos**  
6. Open `iOS App Signer`  
7. Select `WebDriverAgentRunner-Runner.app`.  
8. Generate the WDA *.ipa file.  

**or zip it manually into an IPA yourself, I had some issues last time I did it :(**

### Provide the WebDriverAgent ipa  
1. Paste your WDA ipa in the `./apps` folder with name `WebDriverAgent.ipa` (exact name is important for the scripts)  

### Supervise the iOS devices - NON-MANDATORY BUT PREFERABLE
1. Install Apple Configurator 2 on your Mac.  
2. Attach your first device.  
3. Set it up for supervision using a new(or existing) supervision identity. You can do that for free without having a paid MDM account.  
4. Connect each consecutive device and supervise it using the same supervision identity.  
5. Export your supervision identity file and choose a password.  
6. Save your new supervision identity file in the project `./configs` folder as `supervision.p12`.  
7. Open `config.json` and set your `supervision_password` in `env-config`  

**Note** You can also Trust manually when container is created if no supervision file is provided, but this is not optimal  

### Register your devices for the project
1. Open the `config.json` file.  
2. For each iOS device add a new object inside the `devices-config` array in the json.  
3. For each device provide (all values as strings):  
  * `os` - should be "ios"  
  * `udid` - UDID of the iOS device, can get it with `go-ios` for example  
  * `os_version` - "15.2" for example  
  * `name` - avoid using special characters and spaces except '_'. Example: "Huawei_P20_Pro"  
  * `screen_size` - this is needed to easily work with the stream and remote control. Example: "375x667". You can get it on https://whatismyviewport.com (ScreenSize: at the bottom)   
  * `model` - device model to be displayed in [GADS](https://github.com/shamanec/GADS) device selection.  

### Containerized usbmuxd - LINUX ONLY, DO NOT SKIP
The usual approach would be to mount `/var/run/usbmuxd` to each container. This in practice shares the socket for all iOS devices connected to the host with all the containers. This way a single `usbmuxd` host failure will reflect on all containers. We have a way for `usbmuxd` running inside each container without running on the host at all.  

**Note1** `usbmuxd` HAS to be installed on the host even if we don't really use it. I could not make it work without it.  
**Note2** `usbmuxd` has to be completely disabled on the host so it doesn't automatically start/stop when you connect/disconnect devices.  

1. Open terminal and execute `sudo systemctl mask usbmuxd`. This will stop the `usbmuxd` service from automatically starting and in turn will not lock devices from `usbmuxd` running inside the containers - this is the fast approach. You could also spend the time to completely remove this service from the system (without uninstalling `usbmuxd`)  
2. Validate the service is not running with `sudo systemctl status usbmuxd`  

**NB** It is preferable to have supervised the devices in advance and provided supervision file and password to make setup even more autonomous.  
**NB** Please note that this way the devices will not be available to the host, but you shouldn't really need that unless you are setting up new devices and need to find out the UDIDs, in this case just revert the usbmuxd change with `sudo systemctl unmask usbmuxd`, do what you need to do and mask it again, restart all containers or your system and you should be good to go.  

With this approach we mount the symlink of each device created by the udev rules to each separate container. This in turn makes only a specific device available to its respective container which gives us better isolation from host and more stability.

### Access iOS devices from a Mac for remote development - LINUX ONLY, just for info  
1. Execute `sudo socat TCP-LISTEN:10015,reuseaddr,fork UNIX-CONNECT:/var/run/usbmuxd` on the Linux host with the devices.  
2. Execute `sudo socat UNIX-LISTEN:/var/run/usbmuxd,fork,reuseaddr,mode=777 TCP:192.168.1.8:10015` on a Mac machine on the same network as the Linux devices host.  
3. Restart Xcode and you should see the devices available.  
**NB** Don't forget to replace listen port and TCP IP address with yours.   
**NB** This is in the context when using host `usbmuxd` socket. It is not yet tested with containerized usbmuxd although in theory it should not have issues.  

This can be used for remote development of iOS apps or execution of native XCUITests. It is not thoroughly tested, just tried it out.  

### Example config.json
```
{
  "appium-config": {
    "selenium_hub_host": "192.168.1.8",
    "selenium_hub_port": "4444",
    "selenium_hub_protocol_type": "http"
  },
  "env-config": {
    "devices_host": "192.168.1.5",
    "connect_selenium_grid": "false",
    "supervision_password": "patladjan1",
    "wda_bundle_id": "com.shamanec.WebDriverAgentRunner.xctrunner"
  },
  "devices-config": [
    {
      "os": "ios",
      "name": "iPhone_11",
      "os_version": "13.5.1",
      "udid": "00008030000418C136FB8022",
      "screen_size": "375x667",
      "model": "iPhone 11"
    },
    {
      "os": "android",
      "screen_size": "1080x2241",
      "udid": "WCR7N18B14002300",
      "name": "Huawei_P20_Pro",
      "os_version": "10",
      "model": "Huawei P20 Pro"
    }
  ]
}
```
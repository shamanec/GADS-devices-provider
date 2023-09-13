# Provider
Currently the project assumes that GADS UI, RethinkDB and device providers are on the same network. They can all be on the same machine as well.  
The provider supports Linux, macOS and Windows  
* Linux - iOS < 17, Android
* macOS - iOS, Android
* Windows - Android

# Setup
## Common
### Start RethinkDB instance
The project uses RethinkDB for syncing devices info between providers and GADS UI.  
The RethinkDB instance does not have to be on the same host as the provider or GADS UI. You just need to provide the correct instance IP address and port for the connection.  

**Prerequisites** You need to have Docker(Docker Desktop on macOS, Windows) installed.  
**NB** You don't have to use docker for RethinkDB, you can spin it up any way you prefer.

1. Execute `docker run -d --restart always --name gads-rethink -p 32770:8080 -p 32771:28015 rethinkdb:2.4.2`. This will pull the official RethinkDB 2.4.2 image from Docker Hub and start a container binding ports `32770` for the RethinkDB dashboard and `32771` for db connection.  
2. Open the RethinkDB dashboard on `http://localhost:32770/`  
3. Go to `Tables` and create a new database named `gads`  
4. Add a new table to `gads` database named `devices` with primary key `UDID` (you need to click `Show optional settings` for the primary key)

## Linux
### Docker
1. Install Docker.
2. [Prepare](https://docs.docker.com/engine/install/linux-postinstall/) Docker usage without root privileges

### Golang
* Install Go 1.21 or higher

### Android debug bridge
* Install `adb` (Android debug bridge). It should be available in PATH so it can be directly accessed via Terminal

### Trust devices to use adb - Android only
* On each device activate `Developer options`, open them and enable `Enable USB debugging`
* Connect each device to the host without the provider running - a popup will appear on the device to pair - allow it.

### Usbmuxd
* Install usbmuxd - `sudo apt install usbmuxd` on the host if you want to see the device UDIDs with `go-ios` or similar tools

### WebDriverAgent - iOS only
**NB** You need a Mac machine to do this!
1. [Create](#prepare-webdriveragent-file---linux) a `WebDriverAgent.ipa` file
2. Copy the newly created `ipa` file in the `./apps` folder with name `WebDriverAgent.ipa` (exact name is important for the scripts)

### Supervise devices - iOS only
**NB** You need a Mac machine to do this!
1. Supervise your iOS devices as explained [here](#supervise-devices--ios-only)
2. Copy your supervision certificate and add your supervision password as explained [here](#supervise-devices---ios-only)

### Containerized usbmuxd - iOS only
The usual approach would be to mount `/var/run/usbmuxd` to each container. This in practice shares the socket for all iOS devices connected to the host with all the containers. This way a single `usbmuxd` host failure will reflect on all containers. We have a way for `usbmuxd` running inside each container without running on the host at all.  

**Note1** `usbmuxd` HAS to be installed on the host even if we don't really use it. I could not make it work without it.  
**Note2** `usbmuxd` has to be completely disabled on the host so it doesn't automatically start/stop when you connect/disconnect devices.  

Disable usbmuxd:
1. Open terminal and execute `sudo systemctl mask usbmuxd`. This will stop the `usbmuxd` service from automatically starting and in turn will not lock devices from `usbmuxd` running inside the containers - this is the fast approach. You could also spend the time to completely remove this service from the system (without uninstalling `usbmuxd`)  
2. Validate the service is not running with `sudo systemctl status usbmuxd`  

**NB** It is preferable to have supervised the devices in advance and provided supervision file and password to make setup even more autonomous.  
**NB** Please note that this way the devices will not be available to the host, but you shouldn't really need that unless you are setting up new devices and need to find out the UDIDs, in this case just revert the usbmuxd change with `sudo systemctl unmask usbmuxd`, do what you need to do and mask it again, restart all containers or your system and you should be good to go.  

With this approach we mount the symlink of each device created by the udev rules to each separate container. This in turn makes only a specific device available to its respective container which gives us better isolation from host and more stability.

### Build iOS Docker image
1. Cd into the project folder  
2. Execute `docker build -f Dockerfile-iOS -t ios-appium .`  

### Build Android Docker image
1. Cd into the project folder  
2. Execute `docker build -f Dockerfile-Android -t android-appium .`  

### Setup udev rules
**NB** Provider has to already be [running](#running-the-provider) for this step.  
**NB** Before this step you need to register your devices in `config.json` according to [Devices setup](#devices-setup)  
1. Execute `curl -X POST http://localhost:{ProviderPort}/device/create-udev-rules`  
2. Copy the newly created `90-device.rules` file in the project folder to `/etc/udev/rules.d/` - `sudo cp 90-device.rules /etc/udev/rules/`  
3. Execute `sudo udevadm control --reload-rules` or restart the machine    

**NB** You need to perform this step each time you add a new device to `config.json` so that the symlink for that respective device is properly created in `/dev`  

### Access iOS devices from a Mac for remote development - LINUX ONLY, just for info  
1. Execute `sudo socat TCP-LISTEN:10015,reuseaddr,fork UNIX-CONNECT:/var/run/usbmuxd` on the Linux host with the devices.  
2. Execute `sudo socat UNIX-LISTEN:/var/run/usbmuxd,fork,reuseaddr,mode=777 TCP:192.168.1.8:10015` on a Mac machine on the same network as the Linux devices host.  
3. Restart Xcode and you should see the devices available.  
**NB** Don't forget to replace listen port and TCP IP address with yours.   
**NB** This is in the context when using host `usbmuxd` socket. It is not yet tested with containerized usbmuxd although in theory it should not have issues.  

This can be used for remote development of iOS apps or execution of native XCUITests. It is not thoroughly tested, just tried it out.  

### Known limitations - iOS
1. It is not possible to execute **driver.executeScript("mobile: startPerfRecord")** with Appium to record application performance since Xcode tools are not available.
2. Anything else that might need Instruments and/or any other Xcode/OSX exclusive tools


## macOS
### Xcode - iOS devices
* Install latest stable Xcode release - for iOS 17 install latest beta release
* Install command line tools with `xcode-select --install`

### WebDriverAgent - iOS devices
1. Download the latest release of [WebDriverAgent](https://github.com/appium/WebDriverAgent/releases)
2. Unzip the source code in any folder.
3. Open WebDriverAgent.xcodeproj in Xcode
4. Select signing profiles for WebDriverAgentLib and WebDriverAgentRunner.
5. Build the WebDriverAgent and run on a device at least once to validate it builds and runs as expected.

### Set up go-ios - iOS devices
* Download the latest release of [go-ios](https://github.com/danielpaulus/go-ios) and unzip it
* Add it to `/usr/local/bin` with `sudo cp ios /usr/local/bin`

### Android debug bridge - Android devices
* Install `adb` (Android debug bridge). It should be available in PATH so it can be directly accessed via Terminal

### GADS Android stream - Android devices
1. Download the latest release of [GADS-Android-stream](https://github.com/shamanec/GADS-Android-stream/releases)
2. Unzip the `*.apk` file and put it in `apps` folder (in the GADS-devices-provider repo)

### Appium
* Install Node > 16
* Install Appium with `npm install -g appium`
* Install Appium drivers
    * iOS devices - `appium install driver xcuitestdriver`
    * Android deviecs - `appium install driver uiautomator2`
* Add any additional Appium dependencies like `ANDROID_HOME`(Android SDK) environment variable, etc.

## Windows
### Appium
* Install Node > 16
* Install Appium with `npm install -g appium`
* Install Appium drivers
    * iOS devices - `appium install driver xcuitestdriver`
    * Android deviecs - `appium install driver uiautomator2`
* Add any additional Appium dependencies like `ANDROID_HOME`(Android SDK) environment variable, etc.

### Android debug bridge
* Install `adb` (Android debug bridge). It should be available in PATH so it can be directly accessed via CLI

# Configuration JSON setup
The provider uses `config.json` located in `./config` folder for configuration. It contains config data for Appium, general environment and provisioned devices.

## Common
### Env config
* Set `supervision_password` in `env-config` to the password for your supervised devices certificate - supervision setup can be found below.
    * **NB** Only if you have supervised iOS devices, else you can skip this
* Set `wda_bundle_id` in `env-config`. This is the bundleID used for the prebuilt WebDriverAgent, e.g. `com.shamanec.WebDriverAgentRunner.xctrunner`
    * **NB** Only if you have iOS devices running on Linux, Windows. On macOS the WebDriverAgent is started with `xcodebuild` so the bundleID is irrelevant
* Set `devices_host` in `env-config` to the IP address of the provider machine, e.g. `192.168.1.6`
* Set `rethink_db` in `env-config` to the IP address and port of the RethinkDB instance on your network, e.g. `192.168.1.632771` if you followed the setup for RethinkDB
* ~Set `selenium_grid` in `env-config` to `true/false` if you want Selenium Grid connection established.~ - NOT working atm, leave to `false`

### Appium config - currently hosts only Selenium Grid related data and that does NOT work
* Set `selenium_hub_host` to the IP address of the Selenium Grid instance  
* Set `selenium_hub_port` to the port of the Selenium Grid instance  
* Set `selenium_hub_protocol_type` to `http` or `https` depending on the Selenium Grid instance

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

## Linux
There are no Linux specific configuration options at the moment

## macOS
* Set `wda_repo_path` in `env-config` to the folder where WebDriverAgent was downloaded from Github, e.g. `/Users/shamanec/Downloads/WebDriverAgent-5.8.3/` 
  * When the provider is started it will use this path to build WebDriverAgent with `xcodebuild build-for-testing` once and then will run WebDriverAgent on each device with `xcodebuild test-without-building`. When `go-ios` starts supporting iOS >= 17 then the approach will be changed with prebuilt WebDriverAgent to spend less resources than with `xcodebuild` and speed up provisioning as a whole

## Windows
There are no Windows specific configuration options at the moment

# Additional setup notes
## Prepare WebDriverAgent file - Linux

You need a Mac machine to at least build and sign WebDriverAgent, currently we cannot avoid this.  
You need a paid Apple Developer account to build and sign `WebDriverAgent`. With latest Apple changes it might be possible to do it with free accounts but maybe you'll have to sign the `ipa` file each week

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

## Supervise the iOS devices - Linux, macOS
This is a non-mandatory but a preferable step - it will reduce the needed device provisioning manual interactions

1. Install Apple Configurator 2 on your Mac.
2. Attach your first device.
3. Set it up for supervision using a new(or existing) supervision identity. You can do that for free without having a paid MDM account.
4. Connect each consecutive device and supervise it using the same supervision identity.
5. Export your supervision identity file and choose a password.
6. Save your new supervision identity file in the project `./configs` folder as `supervision.p12`.
7. Open `config.json` and set your `supervision_password` in `env-config`

**Note** You can also Trust manually when device provisioning is running but this is not optimal.

# Running the provider
1. Execute `go build .` and `./GADS-devices-provider`  
2. You can also use `./GADS-devices-provider -port={PORT}` to run the provider on a selected port, the default port without the flag is 10001 - might not work, use at your own risk :D
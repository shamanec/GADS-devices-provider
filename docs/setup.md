# Provider
Currently the project assumes that [GADS UI](https://github.com/shamanec/GADS), MongoDB and device providers are on the same network. They can all be on the same machine as well.  
The provider supports Linux, macOS and Windows  
* Linux - iOS < 17, Android
* macOS - iOS, Android
* Windows - Android

# Setup
## Common
### Start MongoDB instance
The project uses MongoDB for syncing devices info between providers and GADS UI.  
The MongoDB instance does not have to be on the same host as the provider or GADS UI. You just need to provide the correct instance IP address and port for the connection.  

**Prerequisites** You need to have Docker(Docker Desktop on macOS, Windows) installed.  
**NB** You don't have to use docker for MongoDB, you can spin it up any way you prefer.

1. Execute `docker run -d --restart=always --name mongodb -p 27017:27017 mongo:6.0`. This will pull the official MongoDB 6.0 image from Docker Hub and start a container binding port `27017` for db connection.  

## Linux
### Golang
* Install Go 1.21 or higher

### Android debug bridge
* Install `adb` (Android debug bridge). It should be available in PATH so it can be directly accessed via Terminal

### Trust devices to use adb - Android only
* On each device activate `Developer options`, open them and enable `Enable USB debugging`
* Connect each device to the host - a popup will appear on the device to pair - allow it.

### Usbmuxd
* Install usbmuxd - `sudo apt install usbmuxd`

### Appium
* Install Node > 16
* Install Appium with `npm install -g appium`
* Install Appium drivers
    * iOS devices - `appium install driver xcuitestdriver`
    * Android deviecs - `appium install driver uiautomator2`
* Add any additional Appium dependencies like `ANDROID_HOME`(Android SDK) environment variable, etc.

### WebDriverAgent - iOS only
**NB** You need a Mac machine to do this!
1. [Create](#prepare-webdriveragent-file---linux) a `WebDriverAgent.ipa` file
2. Copy the newly created `ipa` file in the `./apps` folder with name `WebDriverAgent.ipa` (exact name is important for the scripts)

### Supervise devices - iOS only
**NB** You need a Mac machine to do this!  
**NB**   
1. Supervise your iOS devices as explained [here](#supervise-devices--ios-only)  
2. Copy your supervision certificate and add your supervision password as explained [here](#supervise-devices---ios-only)  

**NB** You can skip supervising the devices and you should trust manually on first pair attempt by the provider but it is preferable to have supervised the devices in advance and provided supervision file and password to make setup more autonomous.  

### Access iOS devices from a Mac for remote development - LINUX ONLY, just for info  
1. Execute `sudo socat TCP-LISTEN:10015,reuseaddr,fork UNIX-CONNECT:/var/run/usbmuxd` on the Linux host with the devices.  
2. Execute `sudo socat UNIX-LISTEN:/var/run/usbmuxd,fork,reuseaddr,mode=777 TCP:192.168.1.8:10015` on a Mac machine on the same network as the Linux devices host.  
3. Restart Xcode and you should see the devices available.  
**NB** Don't forget to replace listen port and TCP IP address with yours.  

This can be used for remote development of iOS apps or execution of native XCUITests. It is not thoroughly tested, just tried it out.  

### Known limitations - iOS
1. It is not possible to execute **driver.executeScript("mobile: startPerfRecord")** with Appium to record application performance since Xcode tools are not available.
2. Anything else that might need Instruments and/or any other Xcode/OSX exclusive tools

## macOS
### Xcode - iOS only
* Install latest stable Xcode release - for iOS 17 install latest beta release
* Install command line tools with `xcode-select --install`

### WebDriverAgent - iOS only
1. Download the latest release of [WebDriverAgent](https://github.com/appium/WebDriverAgent/releases)
2. Unzip the source code in any folder.
3. Open WebDriverAgent.xcodeproj in Xcode
4. Select signing profiles for WebDriverAgentLib and WebDriverAgentRunner.
5. Build the WebDriverAgent and run on a device at least once to validate it builds and runs as expected.

### Set up go-ios - iOS only
* Download the latest release of [go-ios](https://github.com/danielpaulus/go-ios) and unzip it
* Add it to `/usr/local/bin` with `sudo cp ios /usr/local/bin`

### Android debug bridge - Android only
* Install `adb` (Android debug bridge). It should be available in PATH so it can be directly accessed via Terminal

### GADS Android stream - Android only
1. Starting the provider will automatically download the latests GADS-stream release and put the `apk` file in the `./apps` folder  

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

### Android debug bridge - Android only
* Install `adb` (Android debug bridge). It should be available in PATH so it can be directly accessed via terminal

### iTunes - iOS only
* Install `iTunes` to be able to provision iOS < 17 devices - NOT READY YET, CAN BE SKIPPED

# Configuration JSON setup
The provider uses `config.json` located in `./config` folder for configuration. It contains config data for Appium, general environment and provisioned devices.

## Common
### Env config
* Set `supervision_password` in `env-config` to the password for your supervised iOS devices certificate - supervision setup can be found below.
    * **NB** Only if you have supervised iOS devices, else you can skip this
* Set `wda_bundle_id` in `env-config`. This is the bundleID used for the prebuilt WebDriverAgent, e.g. `com.shamanec.WebDriverAgentRunner.xctrunner`
    * **NB** Only if you have iOS devices running on Linux, Windows. On macOS the WebDriverAgent is started with `xcodebuild` so the bundleID is irrelevant
* Set `host_address` in `env-config` to the IP address of the provider machine, e.g. `192.168.1.6`
* Set `mongo_db` in `env-config` to the IP address and port of the MongoDB instance on your network, e.g. `192.168.1.8:27017` if you followed the setup for MongoDB
* Set `use_selenium_grid` in `env-config` to `true/false` if you want Selenium Grid connection established.  
* Set `provide_android` in `env-config` to `true` if you want to setup and provide Android devices  
* Set `provide_ios` in `env-config` to `true` if you want to setup and provide iOS devices

#### Selenium Grid
Devices can be automatically connected to Selenium Grid 4 instance. You need to create the Selenium Grid instance yourself and then setup the provider to connect to it.  
To setup the provider download the latest Selenium server jar [release](https://github.com/SeleniumHQ/selenium/releases). Copy the downloaded jar and put it in the provider `./apps` folder.  
Open the `config.json` file and in `env-config`:  
* Set `selenium_jar` to the name of the jar you just pasted in `./apps`. Example: `selenium-server-4.14.0.jar`
* Set `use_selenium_grid` in `env-config` to `true` and restart the provider. TOML files for each Appium server will be automatically created and used to connect nodes to the grid instance.  
* Set `selenium_hub_host` to the IP address of the Selenium Grid instance  
* Set `selenium_hub_port` to the port of the Selenium Grid instance  
* Set `selenium_hub_protocol_type` to `http` or `https` depending on the Selenium Grid instance

### Devices config
Each device should have a JSON object in `devices-config` like:
```
{
      "os": "ios",
      "name": "iPhone_11",
      "udid": "00008030000418C136FB8022"
}
```
For each device set: 
  * `os` - should be `android` or `ios`  
  * `name` - use to differentiate devices if needed if multiple devices with the same brand and model available  
  * `udid` - UDID of the Android or iOS device  
    * For Android can get it with `adb devices`  
    * For iOS can get it with Xcode through `Devices & Simulators` or using `go-ios` or a similar tool (tidevice, gidevice, pymobiledevice3)  

## Linux
There are no Linux specific configuration options at the moment

## macOS
* Set `wda_repo_path` in `env-config` to the folder where WebDriverAgent was downloaded from Github, e.g. `/Users/shamanec/Downloads/WebDriverAgent-5.8.3/` 
  * When the provider is started it will use this path to build WebDriverAgent with `xcodebuild build-for-testing` once and then will run WebDriverAgent on each device with `xcodebuild test-without-building`. When `go-ios` starts supporting iOS >= 17 then the approach might be changed with prebuilt WebDriverAgent to spend less resources than with `xcodebuild` and speed up provisioning as a whole

## Windows
There are no Windows specific configuration options at the moment

# Additional setup notes
## Prepare WebDriverAgent file - Linux, Windows

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
8. Generate the WebDriverAgent *.ipa file.  

**or zip it manually into an IPA yourself, I had some issues last time I did it :(**

## Supervise the iOS devices - Linux, macOS, ~Windows~
This is a non-mandatory but a preferable step - it will reduce the needed device provisioning manual interactions

1. Install Apple Configurator 2 on your Mac.
2. Attach your first device.
3. Set it up for supervision using a new(or existing) supervision identity. You can do that for free without having a paid MDM account.
4. Connect each consecutive device and supervise it using the same supervision identity.
5. Export your supervision identity file and choose a password.
6. Save your new supervision identity file in the project `./config` folder as `supervision.p12`.
7. Open `config.json` and set your `supervision_password` in `env-config`

**Note** You can also Trust manually when device provisioning is running but this is not optimal.

# Running the provider
1. Execute `go build .` and `./GADS-devices-provider`  
~2. You can also use `./GADS-devices-provider -port={PORT}` to run the provider on a selected port, the default port without the flag is 10001 - might not work, use at your own risk :D~

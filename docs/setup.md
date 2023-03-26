# Provider setup  

## Dependencies  
The provider itself has minimum dependencies:  
1. Install Docker.  
2. Install Go 1.17 or higher (that is what I'm using, lower might also work)    

## Update the environment in ./configs/config.json  
~1. Set Selenium Grid connection - `true` or `false`. `true` attempts to connect each Appium server to the Selenium Grid instance defined in the same file~ At the moment Selenium Grid connection does not work!  

## Run the provider server   
1. Execute `go build .` and `./GADS-devices-provider` or `go run  main.go` 
2. You can also use `./GADS-devices-provider -port={PORT}` to run the provider on a selected port, the default port without the flag is 10001.     

You can access Swagger documentation on `http://localhost:{PORT}/swagger/index.html`  

## Setup  
### Build iOS Docker image
1. Cd into the provider folder  
2. Execute `docker build -f Dockerfile-iOS -t ios-appium .`  

### Build Android Docker image
1. Cd into the project folder  
2. Execute `docker build -f Dockerfile-Android -t android-appium .`  

### Setup udev rules
**NB** Before this step you need to register your devices in `config.json` according to [Devices setup](#devices-setup)  
1. Execute `curl -X POST http://localhost:{ProviderPort}/device/create-udev-rules`  
2. Copy the newly created `90-device.rules` file to `/etc/udev/rules.d/` - `sudo cp 90-device.rules /etc/udev/rules/`  
3. Execute `sudo udevadm control --reload-rules` or restart the machine    

### Update the Appium config  
1. Open `config.json` 
3. Update your Selenium Grid values and the bundle ID of the used WebDriverAgent in `appium-config`  

### Spin up containers  
If you have followed all the steps, set up and registered the devices and configured the provider just connect all your devices. Container should be automatically created for each of them.  

# Devices setup  
## Android setup
### Dependencies  

1. Install `adb`  with `sudo apt install adb`  
2. Enable `USB Debugging` for each Android device through the developer tools.  
3. Connect the devices to the host and authorize the USB debugging - this will create pairing keys that are used by the containers so you don't have to authorize the debugging each time a container is created.  

### Register devices in config.json
1. Open the `config.json` file.  
2. For each Android device add a new object inside the `devices-list` array in the json.  
3. For each device provide (all values as strings):  
  * `os` - should be `android`   
  * `screen_size` - Go to `https://whatismyandroidversion.com` and fill in the displayed `Screen size`, not `Viewport size`  
  * `os_version` - "11" for example  
  * `name` - avoid using special characters and spaces except '_'. Example: "Huawei_P20_Pro"  
  * `udid` - UDID of the Android device, can get it with `adb devices`   
  * `model` - device model to be displayed in [GADS](https://github.com/shamanec/GADS) device selection.  
  * `minicap_fps` - non-mandatory field, if not provided `minicap` will run with uncapped FPS  
  * `minicap_half_resolution` - non-mandatory field, add with `true` if you want to achieve maximum FPS, will lower stream quality though  
  * `use_minicap` - non-mandatory field, add with `false` if you want to use `GADS-Android-stream` in case `minicap` does not work for the device

### Kill adb-server
1. You need to make sure that adb-server is not running on the host before you start devices containers.  
2. Run `adb kill-server`.  

### Minicap setup

**NB** You can skip this step if you are not going to use remote control with [GADS](https://github.com/shamanec/GADS). In `config.json` change `remote_control` to `false` in `env-config`  
**NB** You can skip this step if you want to use `GADS-Android-stream`, refer to the json explanation above  

1. Unzip the `minicap.zip` file directly into the main project folder. You should see a single `minicap` folder.   
*Note that this might or might not work for you*  

or  

1. Setup Android SDK.  
2. Download and setup Android NDK.  
3. Clone [minicap](https://github.com/shamanec/minicap.git) in the main project folder in a `minicap` folder(default).  
4. Open the `minicap` folder.  
5. Execute `git submodule init` and `git submodule update`.  
6. Execute `ndk-build`.  
7. Execute `experimental/gradlew -p experimental assembleDebug`  
8. Execute `ndk-build NDK_DEBUG=1 1>&2`  

## iOS setup
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

### Supervise the iOS devices  
1. Install Apple Configurator 2 on your Mac.  
2. Attach your first device.  
3. Set it up for supervision using a new(or existing) supervision identity. You can do that for free without having a paid MDM account.  
4. Connect each consecutive device and supervise it using the same supervision identity.  
5. Export your supervision identity file and choose a password.  
6. Save your new supervision identity file in the project `./configs` (or other) folder as `supervision.p12`.  
7. Open `config.json` and set your `supervision_password` in `env-config`  

~**Note** You can also Trust manually when container is created but this is not optimal~ This is not applicable at the moment because `GADS-docker-server` is set up to work with supervised devices  

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

### Containerized usbmuxd
The usual approach would be to mount `/var/run/usbmuxd` to each container. This in practice shares the socket for all iOS devices connected to the host with all the containers. This way a single `usbmuxd` host failure will reflect on all containers. We have a way for `usbmuxd` running inside each container without running on the host at all.  

**Note1** `usbmuxd` HAS to be installed on the host even if we don't really use it. I could not make it work without it.  
**Note2** `usbmuxd` has to be completely disabled on the host so it doesn't automatically start/stop when you connect/disconnect devices.  

1. Open terminal and execute `sudo systemctl mask usbmuxd`. This will stop the `usbmuxd` service from automatically starting and in turn will not lock devices from `usbmuxd` running inside the containers - this is the fast approach. You could also spend the time to completely remove this service from the system (without uninstalling `usbmuxd`)  
2. Validate the service is not running with `sudo systemctl status usbmuxd`  

**NB** It is preferable to have supervised the devices in advance and provided supervision file and password to make setup even more autonomous.  
**NB** Please note that this way the devices will not be available to the host, but you shouldn't really need that unless you are setting up new devices and need to find out the UDIDs, in this case just revert the usbmuxd change with `sudo systemctl unmask usbmuxd`, do what you need to do and mask it again, restart all containers or your system and you should be good to go.  

With this approach we mount the symlink of each device created by the udev rules to each separate container. This in turn makes only a specific device available to its respective container which gives us better isolation from host and more stability.

### Access iOS devices from a Mac for remote development - just for info  
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
    "devices_host": "192.168.1.5",
    "selenium_hub_host": "192.168.1.8",
    "selenium_hub_port": "4444",
    "selenium_hub_protocol_type": "http",
    "wda_bundle_id": "com.shamanec.WebDriverAgentRunner.xctrunner"
  },
  "env-config": {
    "connect_selenium_grid": "false",
    "supervision_password": "patladjan1",
    "remote_control": "true"
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
      "model": "Huawei P20 Pro",
      "minicap_fps" : "30",
      "minicap_half_resolution": "true",
      "use_minicap": "false"
    }
  ]
}
```
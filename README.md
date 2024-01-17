## Introduction

* GADS-device-provider is a server that sets up devices for [Appium](https://github.com/appium/appium) tests execution and remote control with [GADS](https://github.com/shamanec/GADS).
* Supports both Android and iOS devices
* Supports Linux, macOS and Windows - notes below

**NB** I've been doing this having only small number of devices available. It looks like everything is pretty much working but I do not know how it would behave on a bigger scale.  

## Features
* Straighforward setup
* Automatic provisioning when devices are connected
  * Dependencies automatically installed on devices
  * Appium server set up and started for each device
  * Optionally Selenium Grid 4 node can be registered for each device Appium server - NOT WORKING
* [GADS-UI](https://github.com/shamanec/GADS) remote control support
  * iOS video stream using [WebDriverAgent](https://github.com/appium/WebDriverAgent)
  * Android video stream using [GADS-Android-stream](https://github.com/shamanec/GADS-Android-stream)
  * Limited interaction wrapped around Appium - tap, swipe, touch&hold, type text, lock and unlock device
* Appium test execution - each device has its Appium server proxied on a provider endpoint for easier access
* Linux
  * Supports both Android and iOS < 17
  * Has some limitations to Appium execution with iOS devices due to actual Xcode tools being unavailable on Linux
* macOS
  * Supports both Android and iOS
* Windows 10
  * Supports Android and iOS < 17

Developed and tested on `Ubuntu 18.04 LTS`, `macOS Ventura 13.5.1`, `Windows 10`

## Setup  
Read the setup very carefully before starting, I've tried to give as much information as I can - in case something is wrong or missed raise an issue, contact me or contribute :P  
[Provider setup](./docs/setup.md)  

## Thanks
| |About|
|---|---|
|[go-ios](https://github.com/danielpaulus/go-ios)|Many thanks for creating this tool to communicate with iOS devices on Linux, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode|
|[Appium](https://github.com/appium/appium)|Since the project revolves around Appium test execution and it is also used for the remote control with GADS, none of this would be actually possible without it, kudos!|
|[iOS App Signer](https://github.com/DanTheMan827/ios-app-signer)|This is an app for OS X that can (re)sign apps and bundle them into ipa files that are ready to be installed on an iOS device.|  

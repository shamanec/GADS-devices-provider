## Introduction

* GADS-device-provider is a server that sets up devices for remote control with [GADS](https://github.com/shamanec/GADS) and Appium tests execution.
* Supports both Android and iOS devices
* Supports Linux, macOS and Windows with some potential limitations based on OS

**NB** I've been doing this having only small number of devices available. It looks like everything is pretty much working but I do not know how it would behave on a bigger scale.  

## Features
* Lots of steps but straighforward setup
* Remote control support
  * iOS video stream with WebDriverAgent
  * Android video stream with GADS-Android-stream
  * Limited interaction - tap, swipe, type text, lock and unlock device
  * Simple in-browser Appium inspector - see elements and their attributes
  * Taking high quality screenshots - useful since the stream quality is reduced to increase fps
* Appium test execution - each device has its own Appium server running which is exposed on a provider endpoint for easier access
* Optional Selenium Grid connection
* Linux
  * Supports both Android and iOS < 17
  * Has some limitations to Appium execution with iOS devices due to actual Xcode tools being unavailable on Linux
* macOS
  * Supports both Android and iOS
  * Automatically configures each device when it is connected/disconnected
* Windows 10
  * Supports Android
  * Automatically configures each device when it is connected/disconnected

Developed and tested on `Ubuntu 18.04 LTS`, `macOS Ventura 13.5.1`, `Windows 10`

## Setup  
Read the setup very carefully before starting, I've tried to give as much information as I can - in case something is wrong or missed raise an issue, contact me or contribute :P  
[Provider setup](./docs/setup.md)  

## Thanks
| |About|
|---|---|
|[go-ios](https://github.com/danielpaulus/go-ios)|Many thanks for creating this tool to communicate with iOS devices on Linux, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode. Without it none of this would be possible|
|[iOS App Signer](https://github.com/DanTheMan827/ios-app-signer)|This is an app for OS X that can (re)sign apps and bundle them into ipa files that are ready to be installed on an iOS device.|  
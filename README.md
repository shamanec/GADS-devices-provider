## Introduction

* GADS-device-provider is a server that sets up devices for remote control with [GADS](https://github.com/shamanec/GADS) and Appium tests execution.
* Supports both Android and iOS devices
* Supports Linux and macOS

**NB** I've been doing this having only small number of devices available. It looks like everything is pretty much working but I do not know how it would behave on a bigger scale.  

## Features
* Lots of steps but straighforward setup
* Remote control support
  * iOS video stream with WebDriverAgent
  * Android video stream with GADS-Android-stream
  * Limited interaction - tap, swipe, type text, lock and unlock device
* Appium test execution - each device has its own Appium server running which is exposed on a provider endpoint for easier access
* Linux
  * Supports both Android and iOS
  * Automatically spins up/down Docker containers when device is connected/disconnected. Each device is mounted in isolation to its respective container.
  * Can run iOS Appium tests on cheap hardware on bigger scale with only one host machine due to the containerization. Currently does not support iOS >= 17 until a suitable solution for Linux is found.
  * Has some limitations to Appium execution with iOS devices due to actual Xcode tools being unavailable on Linux
* macOS
  * Supports both Android and iOS
  * Automatically configures each device when it is connected/disconnected

Developed and tested on `Ubuntu 18.04 LTS` and `macOS Ventura 13.5.1`

## Setup  
[Provider setup](./docs/setup.md)  

## Thanks
| |About|
|---|---|
|[go-ios](https://github.com/danielpaulus/go-ios)|Many thanks for creating this tool to communicate with iOS devices on Linux, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode. Without it none of this would be possible|
|[iOS App Signer](https://github.com/DanTheMan827/ios-app-signer)|This is an app for OS X that can (re)sign apps and bundle them into ipa files that are ready to be installed on an iOS device.|  
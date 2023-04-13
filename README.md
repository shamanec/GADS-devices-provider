## Introduction

* GADS-devices-provider is a server that spins up/down Docker containers for iOS/Android Appium testing. Can be used autonomously or in conjunction with [GADS](https://github.com/shamanec/GADS) and have your own device farm with basic providers orchestration and devices remote control.   
**NB** I've been doing this having only small number of devices available. It looks like everything is pretty much working but I do not know how it would behave on a bigger scale.  

## Features
* Straighforward setup   
* Endpoints to control the provider, get information, manage containers etc  
* iOS Appium servers in Docker containers  
  - Most of the available functionality of the iOS devices is essentially available because of the amazing [go-ios](https://github.com/danielpaulus/go-ios) project without which none of this would be possible  
  - Automatically spin up when registered device is connected/disconnected  
  - ~Selenium Grid 3 connection~ Currently not working  
  - Run iOS Appium tests on cheap hardware on much bigger scale with only one host machine and in isolation  
  - There are some limitations, you can check [Devices setup](./docs/setup.md)  
* Android Appium servers in Docker containers  
  - Automatically spin up when registered device is connected/disconnected  
  - ~Selenium Grid 3 connection~ Currently not working  

Developed and tested on Ubuntu 18.04 LTS  

## Setup  
[Provider setup](./docs/setup.md)  

## Thanks

| |About|
|---|---|
|[go-ios](https://github.com/danielpaulus/go-ios)|Many thanks for creating this tool to communicate with iOS devices on Linux, perfect for installing/reinstalling and running WebDriverAgentRunner without Xcode. Without it none of this would be possible|
|[iOS App Signer](https://github.com/DanTheMan827/ios-app-signer)|This is an app for OS X that can (re)sign apps and bundle them into ipa files that are ready to be installed on an iOS device.|  
## Dependencies  
The provider itself has minimum dependencies:  
1. Install Docker.  
2. Install Go 1.17 or higher (that is what I'm using, lower might also work)    

## Update the environment in ./configs/config.json  
1. Set Selenium Grid connection - `true` or `false`. `true` attempts to connect each Appium server to the Selenium Grid instance defined in the same file  

## Update provider port
1. Inside `main.go` update the `ProviderPort` value to the port you wish to use.  

## Run the provider server   
1. Execute `go build .` and `./GADS-devices-provider`    

You can access Swagger documentation on `http://localhost:{ProviderPort}/swagger/index.html`  

## Setup  
### Build iOS Docker image
1. Cd into the provider folder  
2. Execute `docker build -f Dockerfile-iOS -t ios-appium .`  

### Build Android Docker image
1. Cd into the project folder.  
2. Execute `docker build -f Dockerfile-Android -t android-appium .`  

### Setup udev rules
**NB** Before this step you need to register your devices in `config.json` according to [Devices setup](./docs/devices-setup.md)  
1. Execute `curl -X POST http://localhost:{ProviderPort}/configuration/create-udev-rules`  
2. Copy the newly created `90-device.rules` file to `/etc/udev/rules.d/`.  
3. Execute `sudo udevadm control --reload-rules`  

### Update the Appium config  
1. Open `config.json` 
3. Update your Selenium Grid values and the bundle ID of the used WebDriverAgent in `appium-config`.  

### Update host udev rules service
1. Open `/lib/systemd/system/systemd-udevd.service` (`sudo systemctl status udev.service` to find out if its a different file)  
2. Add `IPAddressAllow=127.0.0.1` at the bottom  
3. Restart the machine.  
4. This is to allow curl calls from the udev rules to the provider server   

### Spin up containers  
If you have followed all the steps, set up and registered the devices and configured the provider just connect all your devices. Container should be automatically created for each of them.  
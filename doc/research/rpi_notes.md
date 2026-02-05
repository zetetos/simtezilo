# Raspberry Pi Compute Module 4

## Installation

From a macOS machine, you can also run usbboot, just follow the same steps:

1. Clone the usbboot repository
2. Install libusb (`brew install libusb`)
3. Install pkg-config (`brew install pkg-config`)
4. (Optional) Export the `PKG_CONFIG_PATH` so that it includes the directory enclosing `libusb-1.0.pc`
5. Build using make
6. Run the binary

```
git clone --recurse-submodules --shallow-submodules --depth=1 https://github.com/raspberrypi/usbboot
cd usbboot
brew install libusb
brew install pkg-config
make
sudo ./rpiboot
```

If the build is unable to find the header file libusb.h then most likely the `PKG_CONFIG_PATH` is not set properly. This should be set via `export PKG_CONFIG_PATH="$(brew --prefix libusb)/lib/pkgconfig"`.

If the build fails on an ARM-based Mac with a linker error such as `ld: warning: ignoring file '/usr/local/Cellar/libusb/1.0.27/lib/libusb-1.0.0.dylib': found architecture 'x86_64', required architecture 'arm64'` then you may need to build and install `libusb-1.0` yourself:

```
curl -OL https://github.com/libusb/libusb/releases/download/v1.0.27/libusb-1.0.27.tar.bz2
tar -xf libusb-1.0.27.tar.bz2
cd libusb-1.0.27
./configure
make
make check
sudo make INSTALL_PREFIX=/usr/local install
cd ..
```

Running make again should now succeed.


# Raspberry Pi OS based on Debian Trixie (or Bookworm)

* sysfs is fully deprecated and has bee replaced by gpiod

## Operating system preparation

### Avoid SD card write wear

Install [Log2Ram](https://github.com/azlux/log2ram)

### Install supporting packages

Install Opus libraries for pit radio voice comms to Discord
```
apt-get update && apt-get install libopus0
```

### Setup Raspberry Pi

1. Disable on-board audio by adding `dtparam=audio=off` to `/boot/config.txt`.
2. Edit `/etc/fstab` and add the following line
   ```
   tmpfs /var/log tmpfs defaults,size=20M 0 0
   ```
3. Update journald config in `/etc/systemd/journald.conf`
   ```
   [Journal]
   SystemMaxUse=50M
   ForwardToSyslog=no
   ```

### Setup Simtezilo service

1. Enable the Simtezilo service

   `sudo systemctl enable /opt/simtezilo/init/simtezilo.service`


## GPIO notes

Set buttons to inputs with pull-up

`sudo pinctrl set 5,6,16,24 ip pu`

Monitor input buttons

`sudo gpiomon -c /dev/gpiochip0 5 6 16 24`

## Wifi notes
### AP mode

Get <serial> from /etc/cpuinfo
```
nmcli con add type wifi ifname wlan0 con-name SetupMode autoconnect yes Simtezilo-7f46fe8e Simtezilo-<serial>
nmcli con modify SetupMode ipv4.address 10.33.0.1/24
nmcli con modify SetupMode 802-11-wireless.mode ap 802-11-wireless.band bg ipv4.method shared
nmcli con modify SetupMode wifi-sec.key-mgmt wpa-psk
nmcli con modify SetupMode wifi-sec.psk "5imtezil0"
nmcli con modify SetupMode wifi-sec.group ccmp
nmcli con modify SetupMode wifi-sec.pairwise ccmp
nmcli con modify SetupMode 802-11-wireless.security.pmf 1
```


### Infrastructure mode

User provides <ssid> and <password>
```
nmcli connection add type wifi ifname wlan0 con-name RunMode ssid <ssid>
nmcli connection modify RunMode ipv4.method auto
nmcli connection modify RunMode wifi-sec.key-mgmt wpa-psk
nmcli connection modify RunMode wifi-sec.psk <password>
nmcli connection up RunMode
```

### Other commands

```
# List available networks
sudo nmcli -f SSID -t d wifi list ifname wlan0 --rescan auto

# Get AP mode SSID
nmcli -f AP dev show wlan0 | awk '/\.SSID:/ {print $NF}'
```

## Raspberry Pi HAT setup

### Pimoroni Pirate Audio

[Product page](https://learn.pimoroni.com/article/getting-started-with-pirate-audio)

Both the Pirate Audio Line Out and Pirate Audio Headphone Amp can be used. The Headphone Amp version will typically provide the 
best results when set to low gain mode as the output voltage more closely matches the various input voltage of most haptic
amplifiers (input levels of 50-200 millivolts). The Line Out version is better suited to amplifiers that accept around input
levels of ~2.1 volts.


1. Enable SPI in order to use the LCD
2. Turn the backlight off by driving GPIO 13 low when not in use
3. Enable the hifiberry-dac driver
4. Enable the audio amp by driving GPIO 25 high
5. Set button GPIO pins to input, driven high

Add the following to `/boot/config.txt`.

```
dtparam=audio=off
dtparam=spi=on
gpio=13=op,dl

dtoverlay=hifiberry-dac
gpio=25=op,dh

gpio=5=ip,dh
gpio=6=ip,dh
gpio=16=ip,dh
gpio=24=ip,dh
```

### Waveshare 1.3inch LCD HAT

[Product page](https://www.waveshare.com/wiki/1.3inch_LCD_HAT)

1. Enable SPI in order to use the LCD
2. Turn the backlight off by driving GPIO 24 low when not in use


```
dtparam=spi=on
gpio=24=op,dl
```

### Spotpear RPi-1.3inch-MINI-LCD

[Product page](https://spotpear.com/shop/Raspberry-Pi-LCD-Display-Screen-1.3-inch-LCD-Game-RP2040-PiZero-WS.html)

1. Download [audremap18.dtbo](https://cdn.static.spotpear.com/uploads/download/diver/gm154/audremap18.dtbo) to `/boot/overlays/`
2. Enable SPI in order to use the LCD
3. Enable PWM output on pin 18


```
dtparam=spi=on
dtoverlay=audremap18,pins_18_19
```

_If no audio devices are found when executing `aplay -l` then the standard remap overlay can be used by setting `dtoverlay=audremap,pins_18_19`. Note that the select button on GPIO 19 will no longer work as it will be configured for PWM output._


## Buttkicker USB

With `dtparam=audio=off` set, Alsa should see the Butkicker USB device as the only audio output.

Recommended gain setting is 0dB.


# Troubleshooting

## Fix USB audio device ordering

`/lib/modprobe.d/aliases.conf`
Comment out the following
#options snd-usb-audio index=-2

## Fix asound audio rerouting

`~/.asound.rc`
```
pcm.!default { 
   type hw 
#   card PRO 
   card 0 
} 
 
ctl.!default { 
   type hw 
#   card PRO 
   card 0 
}
```
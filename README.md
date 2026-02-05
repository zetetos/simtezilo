<p align="center">
<img src="doc/images/simtezilo-logo.svg" alt="Simtezilo logo" width=300>
<p align="center">

Simtezilo is an application for processesing live Gran Turismo 7 telemetry data and transforming it into haptic feedback for tactile transducers (aka bass shakers), voice-based race notifications via Discord, and visual dashboards for web browsers and Raspberry Pi displays.

## Background

Simtezilo arose from a fairly simple idea of getting some audio feedback about race state so that on-screen overlays could be disabled when racing in VR (Virtual Reality). During early experimentation the idea quickly evolved into a solution for producing full haptic feedback through tactile transducers, with support for running on a small Raspberry Pi to avoid the need for a bulky desktop or laptop machine that other existing solutions require.

The [gt-telemetry](http://github.com/zetetos/gt-telemetry/v2) library was the first component to be built, providing an easy way to read telemetry from the game using Golang. This library is still a core part of Simtezilo.

Today Simtezilo outputs haptic feedback through audio devices, voice notifications through Discord, and provides both a web-based dashboard and an SPI LCD interface for Raspberry Pi hardware.

Output interfaces:
- **Haptics** — Chassis bumps and impacts, engine geometry specific vibration characteristics, transmission gear changes
- **Pit Radio** — Discord based voice notifications for race laps, lap times, progress as well as fuel and tyre advice.
- **Web UI** — Browser-based configuration and telemetry
- **SBC Hardware** — LCD GUI and button support for on-device configuration and race data display

Supported platforms:
- Raspberry Pi (primary target)
- Linux
- MacOS
- Windows

## Installation and documentation

Visit the [Simtezilo website](https://simtezilo.com/) for all installation and configuration documentation.

## Development

See [development.md](doc/development.md)

## Quick-start

Ensure the install Go version matches or exceeds the value in [go.mod](go.mod).

1. Setup config to replay a demo recording
   ```sh
   sed 's/"source":.*/"source": "file:\/\/.\/data\/replays\/demo.gtz"/' support/simtezilo.conf > simtezilo.conf
   ```
2. Start Simtezilo
   ```sh
   make run
   ```

The haptic signal should now be output through the currently enabled audio output device.

## Maintainers

[@vwhitteron](https://github.com/vwhitteron)

## Contributing

Feel free to [open an issue](https://github.com/RichardLitt/standard-readme/issues/new) or submit PRs.


## License

[GPL-3.0](LICENSE)

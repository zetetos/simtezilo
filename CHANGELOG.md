# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.8.0] - 2026-02-05

### Added
- Initial public release
- Haptic output completely reworked
  - Chassis vibration and impacts
  - Engine vibration
  - Transmission gear changes
  - Vehicle specific profiles applied to chassis and engine vibration
  - Calibration mode to help setup output volume levels
  - Parametric equalization per channel
- Pit radio notifications
  - Output via Discord voice channel
  - Lap times as full times and/or deltas
  - Current lap after crossing the start line
  - Race progress updates at specified intervals
  - Fuel strategy and pit calls when refuel required
  - Tyre temperature (beta, work in progess)
- Circuit detection to speed up fuel range calculations
- Full internantional language support
- Web UI
  - App and device settings
  - Software update management
  - Race overview
  - Live telemetry graphs (beta, work in progress)
  - Log inspection
- Support for Raspberry Pi based devices
  - Graphical UI (SPI based LCD)
  - Navigation controls (GPIO buttons)
  - Setup wizard to simplify provisioning of new devices
  - Live data view (beta, work in progress)
  - Settings management using the LCD and buttons
- Support for Linux, MacOS and Windows (command line executable, managed using the web UI)
- Developer tools
  - Live telemetry capture to file
  - Captured telemetry file replay
  - Live charting of internal signal data

## c00f9eb - 2025-01-23

### Added
- Early alpha release
- Haptic output
  - Chassis vibration and impacts
  - Transmission gear changes
- Support for Raspberry Pi based devices
  - Minimal graphical UI (SPI based LCD)
  - Navigation controls (GPIO buttons)

[0.8.0]: https://github.com/zetetos/simtezilo/releases/tag/v0.8.0

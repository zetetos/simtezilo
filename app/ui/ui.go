// Package ui provides the user interface management for the application.
package ui

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/zetetos/simtezilo/app/exitcode"
	"github.com/zetetos/simtezilo/app/hardware"
	"github.com/zetetos/simtezilo/app/i18n"
	"github.com/zetetos/simtezilo/app/i18n/languagedb"
	"github.com/zetetos/simtezilo/app/kinematics"
	"github.com/zetetos/simtezilo/app/ui/gui"
)

// Config holds the configuration for initializing the UserInterface.
type Config struct {
	I18n             *i18n.I18n
	HIDEvents        chan HIDInputEvent
	Display          hardware.Display
	LiveData         *LiveData
	SettingsCallback func(languagedb.Key, string) string
	DevToolsEnabled  func() bool
	ExitCodeChan     chan exitcode.Code
	Log              zerolog.Logger
}

// LiveData holds the dynamic data that can currently be displayed on the UI.
type LiveData struct {
	Gear            int
	TelemetryActive bool
	Calibrating     bool
	forceRefresh    bool
}

// UserInterface manages the user interface components and state.
type UserInterface struct {
	i18n       *i18n.I18n
	display    hardware.Display
	Screen     *gui.Screen
	hidEvents  chan HIDInputEvent
	menuSystem *MenuSystem

	log zerolog.Logger

	settingsCallback func(setting languagedb.Key, action string) string
	done             chan exitcode.Code

	displayData      LiveData
	mode             ScreenMode
	startTime        time.Time
	lastMenuActivity time.Time
	lastActivity     time.Time
}

// NewUserInterface initializes and returns a new UserInterface instance.
func NewUserInterface(config *Config) *UserInterface {
	userInterface := &UserInterface{
		i18n:             config.I18n,
		display:          config.Display,
		hidEvents:        config.HIDEvents,
		menuSystem:       NewMenuSystem(),
		log:              config.Log.With().Str("package", "ui").Logger(),
		displayData:      LiveData{Gear: kinematics.NullGear},
		mode:             ScreenModeStartup,
		startTime:        time.Now(),
		lastActivity:     time.Now(),
		settingsCallback: config.SettingsCallback,
		done:             config.ExitCodeChan,
	}

	// Set devToolsEnabled callback if provided
	if config.DevToolsEnabled != nil {
		userInterface.menuSystem.SetDevToolsEnabledCallback(config.DevToolsEnabled)
	}

	var err error

	userInterface.Screen, err = gui.NewScreen(&gui.Config{
		DisplayDevice: config.Display,
		I18n:          config.I18n,
	})
	if err != nil {
		userInterface.log.Error().
			Err(err).
			Str("sub-component", "screen").
			Str("result", "failure").
			Msg("init")
	}

	return userInterface
}

// RegisterActivity updates the last activity timestamp to the current time.
func (u *UserInterface) RegisterActivity() {
	u.lastActivity = time.Now()
}

// DisplaySleep puts the display into sleep mode.
func (u *UserInterface) DisplaySleep() {
	if int(u.mode) == int(ScreenModeSleep) {
		return
	}

	u.display.Sleep()

	u.mode = ScreenModeSleep

	u.log.Debug().Str("state", "sleep").Msg("display update")
}

// DisplayOff turns the display off.
func (u *UserInterface) DisplayOff() {
	if int(u.mode) == int(ScreenModeOff) {
		return
	}

	u.display.Sleep()

	u.mode = ScreenModeOff

	u.log.Debug().Str("state", "off").Msg("display update")
}

// DisplayToggleOff toggles the display between off and wait modes.
func (u *UserInterface) DisplayToggleOff() bool {
	if int(u.mode) == int(ScreenModeOff) {
		u.mode = ScreenModeWait
		u.displayData.forceRefresh = true

		u.log.Debug().Str("state", "live").Msg("display update")

		return true
	}

	u.mode = ScreenModeOff

	u.log.Debug().Str("state", "off").Msg("display update")

	return false
}

// DrawReadyDisplay renders the ready screen on the display.
// TODO: move it elsewhere or get rid of it entirely.
func (u *UserInterface) DrawReadyDisplay() {
	// Don't override settings mode
	if int(u.mode) == int(ScreenModeSettings) {
		return
	}

	if int(u.mode) == int(ScreenModeWait) && !u.displayData.forceRefresh {
		return
	}

	_ = u.Screen.RenderSplashScreen(u.i18n.GetString(languagedb.UIReady))

	u.mode = ScreenModeWait

	u.log.Debug().Str("state", "wait").Msg("display update")
}

// DrawLiveDisplay renders the live data screen on the display.
func (u *UserInterface) DrawLiveDisplay(data LiveData) {
	if !u.displayData.forceRefresh {
		if data.Calibrating == u.displayData.Calibrating &&
			(data.Gear == u.displayData.Gear || data.Gear == kinematics.NullGear) {
			return
		}
	}

	displayValue := kinematics.GearName(data.Gear)
	if data.Calibrating {
		displayValue = u.i18n.GetString(languagedb.UICalibrating)
	}

	_ = u.Screen.RenderLiveScreen(displayValue)

	u.displayData = data
	u.mode = ScreenModeLive
	u.RegisterActivity()

	u.log.Debug().Str("state", "live").Msg("display update")
}

// ForceRedraw marks the display data as requiring a refresh on the next update.
// This is useful when display orientation changes or other display properties are updated.
func (u *UserInterface) ForceRedraw() {
	u.displayData.forceRefresh = true
}

// SettingAction performs a settings action and returns the resulting setting value.
func (u *UserInterface) SettingAction(setting languagedb.Key, action string) string {
	u.RegisterActivity()
	u.mode = ScreenModeSettings

	return u.settingsCallback(setting, action)
}

// UpdateDisplay updates the display based on the current mode and live data.
func (u *UserInterface) UpdateDisplay(data LiveData) {
	switch u.mode {
	case ScreenModeSettings:
		u.handleSettingsMode(data)
	case ScreenModeLive:
		u.handleLiveMode(data)
	case ScreenModeStartup:
		u.handleStartupMode(data)
	case ScreenModeWait:
		u.handleWaitMode(data)
	case ScreenModeSleep:
		u.handleSleepMode(data)
	case ScreenModeOff:
		u.handleOffMode()
	default:
		u.log.Warn().Str("state", "invalid").Int("mode", int(u.mode)).Msg("display update")
	}

	u.displayData = data
}

// GetSetupModeCountdown returns the current setup mode countdown value.
func (u *UserInterface) GetSetupModeCountdown() int {
	return u.menuSystem.GetSetupModeCountdown()
}

// ResetSetupModeCountdown resets the setup mode countdown to 5.
func (u *UserInterface) ResetSetupModeCountdown() int {
	return u.menuSystem.ResetSetupModeCountdown()
}

// DecrementSetupModeCountdown decrements the setup countdown by 1.
func (u *UserInterface) DecrementSetupModeCountdown() int {
	return u.menuSystem.DecrementSetupModeCountdown()
}

// IsSetupModeCountdownZero returns true if setup countdown has reached zero.
func (u *UserInterface) IsSetupModeCountdownZero() bool {
	return u.menuSystem.IsSetupModeCountdownZero()
}

// handleSettingsMode handles display updates in settings mode.
// Uses lastMenuActivity for timeout checks to prevent gear changes from
// interrupting menu navigation immediately after button presses.
func (u *UserInterface) handleSettingsMode(data LiveData) {
	// Check for 10-second inactivity timeout in menu
	if !u.lastMenuActivity.IsZero() && time.Since(u.lastMenuActivity) > 10*time.Second {
		u.log.Debug().Msg("Menu inactivity timeout - exiting settings")
		u.lastMenuActivity = time.Time{} // Reset timer

		// Show live display if telemetry is active, otherwise sleep
		if data.TelemetryActive {
			// Force refresh to ensure we transition even if gear hasn't changed
			u.displayData.forceRefresh = true
			u.DrawLiveDisplay(data)
		} else {
			u.DisplaySleep()
		}
	}
}

// handleLiveMode handles display updates in live mode.
func (u *UserInterface) handleLiveMode(data LiveData) {
	u.showActiveOrReadyDisplay(data)
}

// handleStartupMode handles display updates in startup mode.
func (u *UserInterface) handleStartupMode(data LiveData) {
	if u.displaySplashTimeoutReached() {
		u.showActiveOrReadyDisplay(data)

		if !data.TelemetryActive {
			u.RegisterActivity()
		}
	}
}

// handleWaitMode handles display updates in wait mode.
func (u *UserInterface) handleWaitMode(data LiveData) {
	if data.TelemetryActive {
		u.DrawLiveDisplay(data)
	} else if u.displayPowerOffTimeoutReached() {
		u.mode = ScreenModeSleep
		u.display.Sleep()
	}
}

// handleSleepMode handles display updates in sleep mode.
func (u *UserInterface) handleSleepMode(data LiveData) {
	if data.TelemetryActive {
		u.DrawLiveDisplay(data)
	}
}

// handleOffMode handles display updates in off mode.
func (u *UserInterface) handleOffMode() {
	u.display.Clear()
	u.display.Sleep()
}

// showActiveOrReadyDisplay shows live display if telemetry is active, otherwise ready display.
func (u *UserInterface) showActiveOrReadyDisplay(data LiveData) {
	if data.TelemetryActive {
		u.DrawLiveDisplay(data)
	} else {
		u.DrawReadyDisplay()
	}
}

// displayPowerOffTimeoutReached checks if the power-off timeout has been reached.
func (u *UserInterface) displayPowerOffTimeoutReached() bool {
	return time.Since(u.lastActivity) > 30*time.Second
}

// displaySplashTimeoutReached checks if the splash screen timeout has been reached.
func (u *UserInterface) displaySplashTimeoutReached() bool {
	return time.Since(u.lastActivity) > 2*time.Second
}

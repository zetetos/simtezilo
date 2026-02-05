package app

import (
	"fmt"
	"time"
)

const (
	positionDebounceTime = 5 * time.Second // Suppress position change notifications for this duration
)

// pitRadioState tracks Discord/pit radio communication state.
type pitRadioState struct {
	// Nofification tracking to prevent duplicate or noisy messages
	fuelNotifyPrewarnIssued          bool          // Flag indicating whether the fuel pre-warn notification has been sent
	fuelNotifyEmptyIssued            bool          // Flag indicating whether the fuel empty notification has been sent
	lastNotifiedLapFuelCritical      int16         // Last lap number when a critical fuel warning was sent
	lastNotifiedLapFuelWarning       int16         // Last lap number when a fuel warning was sent
	lastNotifiedLapFuelStrategy      int16         // Last lap number when a fuel strategy was sent
	lastNotifiedLapNumber            int16         // Last notified lap number
	lastNotifiedLapTime              time.Duration // Last notified lap time
	lastNotifiedRaceProgressInterval int8          // Last notified race progress percentage
	lastNotifiedGridPosition         int16         // Last notified grid position of the vehicle
	debounceGirdPositionNotify       time.Time     // Suppress grid position change notifications until this time

	// Grid position tracking
	currentGridPosition int16 // Current race position of the vehicle

	// Circuit tracking
	circuitName string // Current circuit name

	// Tyre temperature tracking
	tyreState tyreState
}

// resetPitRadioState resets the pit radio state to initial values.
func (a *App) resetPitRadioState() {
	a.pitRadioState = &pitRadioState{
		lastNotifiedLapNumber:      a.gtClient.Telemetry.CurrentLap(),
		lastNotifiedLapTime:        a.gtClient.Telemetry.LastLaptime(),
		lastNotifiedGridPosition:   a.gtClient.Telemetry.GridPosition(),
		debounceGirdPositionNotify: time.Now().Add(24 * time.Hour),
	}

	a.log.Debug().
		Str("state", fmt.Sprintf("%+v", a.state)).
		Msg("pit radio state reset")
}

// pitRadioIsActive checks if the pit radio is active and ready for sending messages.
func (a *App) pitRadioIsActive() bool {
	// Pit radio functionality is disabled in config
	if !a.config.PitRadioEnabled() {
		return false
	}

	// Pit radio is not initialized
	if a.pitRadio == nil {
		return false
	}

	// Initialize pit radio state when not set
	if a.pitRadio != nil && a.pitRadioState == nil {
		a.resetPitRadioState()

		return false
	}

	// Pit radio is active only during live sessions
	return a.state.current.isLive
}

// sendPitRadioMessage sends pit radio messages based on fuel range, lap progress and position changes.
func (a *App) sendPitRadioMessage() {
	if !a.pitRadioIsActive() {
		return
	}

	if !a.sequenceHasAdvanced() || !a.telemetryIsActive() {
		return
	}

	if a.state.current.lapNumber <= 0 {
		return
	}

	if a.raceHasFinished() {
		return
	}

	a.notifyFuelWarnings()
	a.notifyRaceProgress()
	a.notifyTyreTemperature()
	a.notifyGridPositionChange()
	a.notifyCircuitChange()
}

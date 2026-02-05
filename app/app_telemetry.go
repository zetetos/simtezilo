package app

import (
	gtmodels "github.com/zetetos/gt-telemetry/v2/pkg/models"
)

// checkForNewLap sends an event to the lapStartEvents channel when a new lap is detected.
func (a *App) checkForNewLap() {
	current := a.state.current.lapNumber
	last := a.state.last.lapNumber
	lapDelta := current - last

	//nolint:gocritic // if-else chain is clearer here
	if lapDelta == 1 {
		a.lapStartEvents <- a.state.current.sequenceNumber

		a.log.Debug().
			Int16("current", a.state.current.lapNumber).
			Int16("previous", a.state.last.lapNumber).
			Str("status", "normal").
			Msg("New lap started")
	} else if lapDelta > 1 {
		a.log.Debug().
			Int16("current", a.state.current.lapNumber).
			Int16("previous", a.state.last.lapNumber).
			Str("status", "skip forwards").
			Msg("New lap started")
	} else if lapDelta < 0 {
		a.log.Debug().
			Int16("current", a.state.current.lapNumber).
			Int16("previous", a.state.last.lapNumber).
			Str("status", "skip backwards or reset").
			Msg("New lap started")
	}
}

// checkRaceComplete checks if the race has been completed.
func (a *App) checkRaceComplete() {
	currentState := a.gtClient.Telemetry.RaceComplete()

	if a.state.raceComplete == currentState {
		return
	}

	a.state.raceComplete = currentState

	if a.state.raceComplete {
		a.log.Info().
			Int16("current_lap", a.state.current.lapNumber).
			Int16("race_laps", a.gtClient.Telemetry.RaceLaps()).
			Msg("Race complete")
	}
}

// checkInPostRaceMenu checks if the game is currently in the post-race menu based on telemetry data.
// This state is signified by the vehicle RPM and angular velocity is static. When the vehicle is on
// circuit these values will be constantly changing to a small degree, even when the vehicle is at a
// complete stop.
func (a *App) checkInPostRaceMenu() {
	// Reset state if not in appropriate game state
	if a.state.current.gameState != gtmodels.GameStateLive || a.gtClient.Telemetry.Flags().GamePaused {
		a.state.postRaceMenuFrameCount = 0
		if a.state.isInPostRaceMenu {
			a.log.Info().Msg("Exited race complete menu")
			a.state.isInPostRaceMenu = false
		}

		return
	}

	// Check if telemetry values are static
	currentRPM := float64(a.state.current.engineRPM)
	lastRPM := float64(a.state.last.engineRPM)
	rpmIsStatic := abs(currentRPM-lastRPM) == 0 && currentRPM > 0 // RPM must be > 0 to avoid false positives when engine has stalled
	rotationIsStatic := a.kinematics.Current.SixDOFRotation.AccelMag == 0
	telemetryIsStatic := rpmIsStatic && rotationIsStatic

	// Use frame counting to debounce state changes
	const stableFrameCount = 60

	if !telemetryIsStatic {
		// Only exit if when enough consecutive non-static frames have been seen
		if a.state.postRaceMenuFrameCount > 0 {
			a.state.postRaceMenuFrameCount--
		} else if a.state.isInPostRaceMenu {
			a.state.SetPostRaceMenuState(false)
		}

		return
	}

	// Enter post-race menu state after consecutive static frames have stabilized
	if a.state.postRaceMenuFrameCount >= stableFrameCount {
		a.state.SetPostRaceMenuState(true)
	} else {
		a.state.postRaceMenuFrameCount++
	}
}

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}

	return x
}

// sequenceHasAdvanced checks if the telemetry sequence number has advanced.
func (a *App) sequenceHasAdvanced() bool {
	if a.state.current.sequenceNumber == 0 || a.state.current.sequenceDelta == 0 {
		return false
	}

	return true
}

// telemetryIsActive checks if the telemetry is in an active state.
func (a *App) telemetryIsActive() bool {
	if a.gtClient.Telemetry.Flags().GamePaused {
		return false
	}

	if !a.vehicleIsOnTrack() {
		return false
	}

	if a.gtClient.Telemetry.Flags().Live {
		return true
	}

	// If in replay mode, telmetry is considered active if the time of day has advanced.
	if a.config.GetHapticsReplayEnabled() {
		return true
	}

	return false
}

// telemetryPacketsDropped checks if telemetry packets have been dropped.
// Returns the count of dropped packets.
func (a *App) telemetryPacketsDropped() uint32 {
	dropped := a.state.current.sequenceDelta - 1

	if dropped > 1 {
		a.log.Debug().Uint32("dropped", dropped).Msg("telemetry packets")
	}

	return dropped
}

// timeOfDayHasReset checks if the time of day has reset (gone backwards).
func (a *App) timeOfDayHasReset() bool {
	if a.state.current.sequenceNumber == 0 {
		return false
	}

	timeOfDayDelta := a.state.current.timeOfDay - a.state.last.timeOfDay

	return timeOfDayDelta.Milliseconds() < 0
}

// gameStateHasChanged checks if the game state has changed between updates.
// For main menu state, requires 3 consecutive frames to avoid false positives from transient flickering.
func (a *App) gameStateHasChanged() bool {
	current := a.state.current.gameState
	last := a.state.last.gameState

	return current != last
}

// liveFlagHasChanged checks if the live flag has changed between live and replay modes.
func (a *App) liveFlagHasChanged() bool {
	return a.state.current.isLive != a.state.last.isLive
}

// updateState copies the current state to the previous state and updates the current
// state with the latest telemetry data.
// State updates are skipped if the sequence ID has not changed.
// A boolean is returned indicating if the statewas updated (true) or not (false).
func (a *App) updateState() (didUpdate bool) {
	if a.gtClient.Telemetry.SequenceID() == a.state.current.sequenceNumber {
		return false
	}

	a.state.last = a.state.current

	// Game
	gameState := a.gtClient.Telemetry.GameState()
	if gameState == gtmodels.GameStateMainMenu {
		// Main menu state changes are delayed to avoid flapping (switch vehicle, start replay, etc)
		if a.state.mainMenuFrameCount >= 10 {
			a.state.current.gameState = gtmodels.GameStateMainMenu
		} else {
			a.state.mainMenuFrameCount++
		}
	} else {
		// Other game state changes are immediate
		a.state.mainMenuFrameCount = 0
		a.state.current.gameState = gameState
	}

	// Session
	a.state.current.sequenceNumber = a.gtClient.Telemetry.SequenceID()
	a.state.current.sequenceDelta = a.state.current.sequenceNumber - a.state.last.sequenceNumber
	a.state.current.timeOfDay = a.gtClient.Telemetry.TimeOfDay()
	a.state.current.isLive = a.gtClient.Telemetry.Flags().Live

	// Vehicle
	a.state.current.transmissionGear = a.gtClient.Telemetry.CurrentGear()
	a.state.current.engineRPM = a.gtClient.Telemetry.EngineRPM()

	// Race
	a.state.current.lapNumber = a.gtClient.Telemetry.CurrentLap()
	a.state.current.lastLapTime = a.gtClient.Telemetry.LastLaptime()

	return true
}

// vehicleHasChanged checks if the vehicle has changed based on telemetry data.
func (a *App) vehicleHasChanged() bool {
	return a.vehicle.ID != a.gtClient.Telemetry.VehicleID()
}

// vehicleIsOnTrack checks if the vehicle is on track based on telemetry data.
// When in the menu system the race laps will be set to uin16 max.
// When at a  track screen before a session has started, the race laps will be set to 0.
func (a *App) vehicleIsOnTrack() bool {
	return a.gtClient.Telemetry.RaceLaps() < 32000
}

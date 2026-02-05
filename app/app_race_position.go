package app

import (
	"fmt"
	"time"

	"github.com/zetetos/simtezilo/app/pitradio"
)

// notifyGridPositionChange sends position change notifications over the pit radio.
func (a *App) notifyGridPositionChange() {
	if !a.shouldNotifyPositionChange() {
		return
	}

	message := fmt.Sprintf("P%d", a.pitRadioState.currentGridPosition)

	if a.pitRadio != nil {
		err := a.pitRadio.Send(pitradio.Message{
			MessageType: pitradio.TextMessage,
			Text:        message,
			Lang:        a.i18n.LanguageCode(),
			Accent:      a.config.GetAppAccent(),
		})
		if err != nil {
			a.log.Error().
				Err(err).
				Str("message", message).
				Msg("Send position change message")

			return
		}

		a.log.Debug().
			Str("message", message).
			Int16("lap", a.state.current.lapNumber).
			Msg("Send position change message")
	}
}

// shouldNotifyPositionChange checks if conditions are met to send position change notifications.
func (a *App) shouldNotifyPositionChange() bool {
	if !a.pitRadioIsActive() {
		return false
	}

	if a.state.current.lapNumber <= 0 {
		return false
	}

	if a.gtClient.Telemetry.RaceEntrants() <= 1 {
		return false
	}

	if a.gtClient.Telemetry.RaceLaps() < 0 {
		return false
	}

	return a.positionHasChanged()
}

// positionHasChanged checks if the grid position has changed since the last update.
func (a *App) positionHasChanged() bool {
	position := a.gtClient.Telemetry.GridPosition()

	if position <= 0 {
		return false
	}

	if position == a.pitRadioState.lastNotifiedGridPosition {
		return false
	} else if a.pitRadioState.currentGridPosition != position {
		a.pitRadioState.currentGridPosition = position
		a.pitRadioState.debounceGirdPositionNotify = time.Now().Add(positionDebounceTime)

		a.log.Debug().
			Int16("new_position", position).
			Int16("old_position", a.pitRadioState.lastNotifiedGridPosition).
			Msg("Position change")
	}

	// Debounce position changes until time delay reached
	if time.Now().Before(a.pitRadioState.debounceGirdPositionNotify) {
		return false
	}

	// Reset debounce timer
	a.pitRadioState.debounceGirdPositionNotify = time.Now().Add(24 * time.Hour)
	a.pitRadioState.lastNotifiedGridPosition = a.pitRadioState.currentGridPosition

	return true
}

package app

import (
	"fmt"
	"time"

	"github.com/zetetos/gt-telemetry/v2/pkg/models"
	"github.com/zetetos/simtezilo/app/i18n/languagedb"
	"github.com/zetetos/simtezilo/app/pitradio"
)

// notifyLapTime sends lap time notifications over the pit radio.
func (a *App) notifyLapTime() {
	if !a.shouldSendLapTimeNotification() {
		return
	}

	var message string

	bestLapTime := a.gtClient.Telemetry.BestLaptime()
	maxNotifyDelta := time.Duration(a.config.GetPitRadioNotifyLapTimesMaxDeltaSeconds() * float64(time.Second))

	firstLapCompleted := a.state.current.lapNumber == 2 // notification will occur at the start of lap 2
	secondLapCompleted := a.state.current.lapNumber > 2 // notifcation will occur at the start of lap 3

	bestLapTimeSet := bestLapTime > 0
	newLapRecord := secondLapCompleted && bestLapTimeSet && a.state.current.lastLapTime <= bestLapTime

	delta := a.state.current.lastLapTime - bestLapTime
	reportableNegativeDelta := secondLapCompleted && bestLapTimeSet && delta > 0 && delta <= maxNotifyDelta

	switch {
	case firstLapCompleted:
		message = FormatDuration(a.state.current.lastLapTime)

		a.state.bestLapTime = a.state.current.lastLapTime
	case newLapRecord:
		format := a.i18n.GetString(languagedb.RadioLapRecordFmt)
		delta := FormatDeltaTime(a.state.bestLapTime - a.state.current.lastLapTime)
		duration := FormatDuration(a.state.current.lastLapTime)

		message = fmt.Sprintf(format, delta, duration)

		a.state.bestLapTime = bestLapTime
	case reportableNegativeDelta:
		format := a.i18n.GetString(languagedb.RadioSlowerLapFmt)
		delta := FormatDeltaTime(a.state.current.lastLapTime - bestLapTime)

		message = fmt.Sprintf(format, delta)
	default:
		// Non-reportable lap time
		return
	}

	// Send lap time message to Discord
	err := a.pitRadio.Send(pitradio.Message{
		MessageType: pitradio.TextMessage,
		Text:        message,
		Lang:        a.i18n.LanguageCode(),
		Accent:      a.config.GetAppAccent(),
		NoCache:     true,
	})
	if err != nil {
		a.log.Error().
			Err(err).
			Str("component", "pit radio").
			Str("message", message).
			Msg("Send lap time message")

		return
	}

	a.log.Debug().
		Str("message", message).
		Int16("lap", a.state.current.lapNumber).
		Dur("lapTime", a.state.current.lastLapTime).
		Msg("Send lap time message")
}

// shouldSendLapTimeNotification checks if conditions are met to send lap time notifications.
func (a *App) shouldSendLapTimeNotification() bool {
	if !a.pitRadioIsActive() {
		return false
	}

	if !a.config.GetPitRadioNotifyLapTimesEnabled() {
		return false
	}

	if a.pitRadioState.lastNotifiedLapTime == a.state.current.lastLapTime {
		return false
	}

	a.pitRadioState.lastNotifiedLapTime = a.state.current.lastLapTime

	return a.state.current.lastLapTime > 0
}

// notifyLapNumber sends lap number notifications over the pit radio.
func (a *App) notifyLapNumber() {
	if !a.shouldSendLapNumberNotification() {
		return
	}

	lapInfo := a.getLapNotificationInfo()

	if lapInfo.alreadyNotified {
		return
	}

	message := a.determineLapMessage(lapInfo)
	if message == "" {
		return
	}

	a.sendLapNotification(message, lapInfo.currentLap)
}

// shouldSendLapTimeNotification checks if conditions are met to send lap time notifications.
func (a *App) shouldSendLapNumberNotification() bool {
	if !a.pitRadioIsActive() {
		return false
	}

	if !a.config.GetPitRadioNotifyRaceLapsEnabled() {
		return false
	}

	raceType := a.gtClient.Telemetry.RaceType()
	raceLaps := a.gtClient.Telemetry.RaceLaps()
	currentLap := a.state.current.lapNumber

	// Do not notify for time trial sessiopns
	if raceType == models.RaceTypeTimeTrial {
		return false
	}

	// Do not notify for single lap sprint races
	if raceLaps == 1 {
		return false
	}

	// Do not notify on starting lap
	if currentLap == 0 {
		return false
	}

	// Do not notify beyond race laps
	if currentLap > raceLaps && raceLaps > 0 {
		return false
	}

	// Notify only at the configured lap interval
	if currentLap%int16(a.config.GetPitRadioNotifyRaceLapsIntervalLaps()) != 0 { //nolint:gosec // lap count will not overflow
		return false
	}

	return a.pitRadioState.lastNotifiedLapNumber < currentLap
}

// lapNotificationInfo holds information needed for lap notifications.
type lapNotificationInfo struct {
	currentLap      int16
	raceLaps        int16
	lapsRemaining   int16
	longRace        bool
	alreadyNotified bool
}

// getLapNotificationInfo gathers lap information for notifications.
func (a *App) getLapNotificationInfo() lapNotificationInfo {
	currentLap := a.state.current.lapNumber
	raceLaps := a.gtClient.Telemetry.RaceLaps()

	lapsRemaining := int16(-1)
	if raceLaps > 0 {
		lapsRemaining = raceLaps - currentLap + 1
	}

	// Notifications are more sparse for longer races
	longRace := raceLaps > int16(a.config.GetPitRadioNotifyRaceProgressMinLaps()) //nolint:gosec // race laps will not overflow

	return lapNotificationInfo{
		currentLap:      currentLap,
		raceLaps:        raceLaps,
		lapsRemaining:   lapsRemaining,
		longRace:        longRace,
		alreadyNotified: a.pitRadioState.lastNotifiedLapNumber == currentLap,
	}
}

// determineLapMessage determines what lap message to send based on race conditions.
func (a *App) determineLapMessage(info lapNotificationInfo) string {
	raceCompleted := info.raceLaps > 0 && info.lapsRemaining == 0
	finalLap := info.currentLap == info.raceLaps
	countdownLaps := int16(a.config.GetPitRadioNotifyRaceLapsCountdownLaps()) //nolint:gosec // lap count will not overflow
	lastFewLaps := info.lapsRemaining <= countdownLaps && info.longRace

	switch {
	case raceCompleted:
		return a.i18n.GetString(languagedb.RadioRaceFinish)
	case finalLap:
		return a.i18n.GetString(languagedb.RadioFinalLap)
	case lastFewLaps:
		format := a.i18n.GetString(languagedb.RadioLapsRemainingFmt)

		return fmt.Sprintf(format, info.lapsRemaining)
	case info.currentLap > 1:
		return fmt.Sprintf("Lap %d", info.currentLap)
	default:
		return ""
	}
}

// sendLapNotification sends the lap notification message via pit radio.
func (a *App) sendLapNotification(message string, currentLap int16) {
	a.pitRadioState.lastNotifiedLapNumber = currentLap

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
			Msg("Send lap number message")

		return
	}

	a.log.Debug().
		Str("message", message).
		Int16("lap", a.state.current.lapNumber).
		Msg("Send lap number message")
}

// notifyRaceProgress sends lap number notifications based on race progress over the pit radio.
func (a *App) notifyRaceProgress() {
	if !a.shouldNotifyRaceProgress() {
		return
	}

	progressIntervalPc := a.config.GetPitRadioNotifyRaceProgressIntervalPc()

	currentLap := a.state.current.lapNumber

	if currentLap == 0 {
		return
	}

	raceLaps := a.gtClient.Telemetry.RaceLaps()
	if raceLaps == 0 {
		// TODO: handle time based endurance races
		return
	}

	circuitLengthMetres := a.circuit.LengthMetres()
	if circuitLengthMetres <= 0 {
		return
	}

	// Calculate race progress based on distance in metres
	totalRaceDistanceMetres := float64(raceLaps) * circuitLengthMetres
	currentRaceDistanceMetres := float64(currentLap-1)*circuitLengthMetres + a.circuit.LapProgress()*circuitLengthMetres

	raceProgressPercent := int8(100 * currentRaceDistanceMetres / totalRaceDistanceMetres)

	// Calculate current progress interval based on progressInterval
	currentProgressInterval := (raceProgressPercent / int8(progressIntervalPc)) * int8(progressIntervalPc) //nolint:gosec // percent value will not overflow

	// Skip notifications at 0%
	if raceProgressPercent <= 0 {
		return
	}

	NotifyInterval := currentProgressInterval > a.pitRadioState.lastNotifiedRaceProgressInterval &&
		raceProgressPercent < 100

	if !NotifyInterval {
		return
	}

	format := a.i18n.GetString(languagedb.RadioRaceProgressFmt)
	message := fmt.Sprintf(format, raceProgressPercent)

	a.pitRadioState.lastNotifiedRaceProgressInterval = currentProgressInterval

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
				Msg("Send lap number message")

			return
		}

		a.log.Debug().
			Str("message", message).
			Int16("lap", a.state.current.lapNumber).
			Msg("Send lap number message")
	}
}

// shouldNotifyRaceProgress checks if conditions are met to send race progress notifications.
func (a *App) shouldNotifyRaceProgress() bool {
	if !a.pitRadioIsActive() {
		return false
	}

	if !a.config.GetPitRadioNotifyRaceProgressEnabled() {
		return false
	}

	if a.gtClient.Telemetry.RaceLaps() < int16(a.config.GetPitRadioNotifyRaceProgressMinLaps()) { //nolint:gosec // race laps will not overflow
		return false
	}

	return true
}

func (a *App) getPreviousLapTime(lap int16) (lapTime time.Duration) {
	if len(a.lapEvents) == 0 || lap < int16(len(a.lapEvents)) { //nolint:gosec // lap count will not overflow int16
		return 0
	}

	for i := len(a.lapEvents) - 1; i >= 0; i-- {
		if a.lapEvents[i].Lap == lap-1 {
			lapTime = a.lapEvents[i].LapTime

			break
		}
	}

	return lapTime
}

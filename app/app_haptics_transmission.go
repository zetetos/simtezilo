package app

import (
	"math"

	"github.com/zetetos/simtezilo/app/signal"
	"github.com/zetetos/simtezilo/app/synthesizer"
)

// playGearShiftHaptic outputs a haptic effect simulating forces during a gear change.
func (a *App) playGearShiftHaptic() {
	magnitude := a.determineGearShiftMagnitude()

	a.synth.PlayEffect("gearShift", magnitude, synthesizer.ChannelTransmission)

	a.log.Debug().
		Int("sequence_id", int(a.state.current.sequenceNumber)).
		Float64("magnitude", magnitude).
		Float64("gforce", a.kinematics.GetSurgeGforce()).
		Int("gear", a.kinematics.Current.TransmissionGear).
		Msg("gear change")
}

// determineGearShiftMagnitude calculates the magnitude of the gear shift haptic effect.
// A fixed magnitude simulates only the forces of the gear change mechanism itself.
// A dynamic magnitude simulates the forces of the gear change mechanism combined with
// the longitudinal g-force experienced during the gear change.
func (a *App) determineGearShiftMagnitude() float64 {
	synthMagnitude, _ := a.synth.GetChannelMagnitude(synthesizer.ChannelTransmission)

	if !a.config.GethapticsDynamicTransFeedbackEnabled() {
		return synthMagnitude
	}

	gForce := a.kinematics.GetSurgeGforce()
	gforceMax := a.config.GetHapticsTransmissionGforceMax()
	volumeCurve := a.config.GetHapticsTransmissionCurve() / 1000

	magnitudeMin := synthesizer.GainToPowerRatio(a.transmissionGainMin)

	magnitude := math.Pow((gForce/gforceMax), volumeCurve) * synthMagnitude
	magnitude, _ = signal.LimitWindow(magnitude, magnitudeMin, synthMagnitude)

	return magnitude
}

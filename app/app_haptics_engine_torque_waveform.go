package app

import (
	"math"

	"github.com/zetetos/simtezilo/app/signal"
	"github.com/zetetos/simtezilo/app/synthesizer"
	"github.com/zetetos/simtezilo/app/vehicle"
)

// GenerateTorqueCurveWaveform creates engine haptic waveforms based on engine-specific torque curves
// and torsional excitation characteristics as documented by EPI Engineering.
// This function incorporates both primary and secondary balance configurations to model
// realistic engine torque output variations.
// TODO: remove or keep and make private. Public to stop linter error.
func (a *App) GenerateTorqueCurveWaveform(rpm float64, engineRoughness float64, engineBuffer *[]float64) {
	params := a.calculateTorqueWaveformParams(rpm)
	a.generateTorqueWaveformSamples(params, engineRoughness, engineBuffer)
}

// torqueWaveformParams holds all calculated parameters for torque waveform generation.
type torqueWaveformParams struct {
	sampleRate          float64
	rpmPercent          float64
	rpmNormalized       float64
	primaryTorqueFreq   float64
	secondaryTorqueFreq float64
	tertiaryTorqueFreq  float64
	baseAmplitude       float64
	primaryAmplitude    float64
	secondaryAmplitude  float64
	tertiaryAmplitude   float64
	adjust              float64
	rpmPowerRatio       float64
}

// calculateTorqueWaveformParams calculates all parameters needed for torque waveform generation.
func (a *App) calculateTorqueWaveformParams(rpm float64) *torqueWaveformParams {
	sampleRate := float64(a.synth.GetSampleRate())
	rpmPercent := rpm / float64(a.vehicle.RevLimit)
	rpmNormalized, _ := signal.LimitWindow(rpmPercent, 0.0, 1.0)

	// Base firing frequency from engine characteristics
	baseFiringRate := rpm * a.vehicle.Engine.FiringFrequency * a.vehicle.Engine.Haptics.PulseScale
	frequencies := a.calculateTorqueFrequencies(baseFiringRate)
	amplitudes := a.calculateTorqueAmplitudes(rpmNormalized)

	return &torqueWaveformParams{
		sampleRate:          sampleRate,
		rpmPercent:          rpmPercent,
		rpmNormalized:       rpmNormalized,
		primaryTorqueFreq:   frequencies.primary,
		secondaryTorqueFreq: frequencies.secondary,
		tertiaryTorqueFreq:  frequencies.tertiary,
		baseAmplitude:       amplitudes.base,
		primaryAmplitude:    amplitudes.primary,
		secondaryAmplitude:  amplitudes.secondary,
		tertiaryAmplitude:   amplitudes.tertiary,
		adjust:              amplitudes.adjust,
		rpmPowerRatio:       amplitudes.rpmPowerRatio,
	}
}

// torqueFrequencies holds the calculated frequency components.
type torqueFrequencies struct {
	primary   float64
	secondary float64
	tertiary  float64
}

// calculateTorqueFrequencies calculates the torque frequency components based on engine layout.
func (a *App) calculateTorqueFrequencies(baseFiringRate float64) torqueFrequencies {
	var secondary, tertiary float64

	switch a.vehicle.Engine.Layout {
	case "I":
		// Inline engines: Secondary balance affects reciprocating mass imbalance
		secondary = baseFiringRate * 2.0
		tertiary = baseFiringRate * 4.0
	case "V":
		// V engines: Balance depends on bank angle and crank configuration
		secondary, tertiary = a.calculateVEngineFrequencies(baseFiringRate)
	case "H":
		// Horizontally opposed: Generally well-balanced
		secondary = baseFiringRate * 2.0
		tertiary = baseFiringRate * 4.0
	default:
		// Default case for other engine types
		secondary = baseFiringRate * 1.5
		tertiary = baseFiringRate * 3.0
	}

	// Limit frequencies to usable range (160Hz low-pass filter)
	maxUsableFreq := 160.0

	primary := baseFiringRate

	if primary > maxUsableFreq {
		primary = maxUsableFreq
	}

	if secondary > maxUsableFreq {
		secondary = maxUsableFreq * 0.8
	}

	if tertiary > maxUsableFreq {
		tertiary = maxUsableFreq * 0.6
	}

	return torqueFrequencies{primary, secondary, tertiary}
}

// calculateVEngineFrequencies calculates frequency components for V engines.
func (a *App) calculateVEngineFrequencies(baseFiringRate float64) (float64, float64) {
	switch a.vehicle.Engine.Chambers {
	case 8:
		// V8 with typical 90° bank angle
		return baseFiringRate * 1.5, baseFiringRate * 3.0
	case 6:
		// V6 configurations vary widely
		return baseFiringRate * 1.33, baseFiringRate * 2.67
	default:
		// Other V configurations
		return baseFiringRate * 1.25, baseFiringRate * 2.5
	}
}

// torqueAmplitudes holds the calculated amplitude components.
type torqueAmplitudes struct {
	base          float64
	primary       float64
	secondary     float64
	tertiary      float64
	adjust        float64
	rpmPowerRatio float64
}

// calculateTorqueAmplitudes calculates the amplitude components for torque waveform.
func (a *App) calculateTorqueAmplitudes(rpmNormalized float64) torqueAmplitudes {
	throttlePercent := float64(a.gtClient.Telemetry.ThrottleOutputPercent()) / 100
	throttlePercent, _ = signal.LimitWindow(throttlePercent, 0.0, 1.0)

	gainOffset, amplitudeScale := a.getVehicleTypeAdjustments()
	baseAmplitude := 0.6 + (throttlePercent * amplitudeScale)

	rpmPowerRatio := synthesizer.GainToPowerRatio(-1.0 * rpmNormalized)
	adjust := synthesizer.GainToPowerRatio(a.vehicle.Engine.Haptics.Gain + gainOffset)

	// Calculate component amplitudes
	primaryAmplitude := baseAmplitude * (2.0 - a.vehicle.Engine.Haptics.PrimaryBalance) * rpmPowerRatio
	secondaryAmplitude := baseAmplitude * 0.6 * (1.5 - a.vehicle.Engine.Haptics.SecondaryBalance) * rpmPowerRatio
	tertiaryAmplitude := baseAmplitude * 0.3 * (2.0 - a.vehicle.Engine.Haptics.PrimaryBalance) *
		(1.5 - a.vehicle.Engine.Haptics.SecondaryBalance) * rpmPowerRatio

	return torqueAmplitudes{
		base:          baseAmplitude,
		primary:       primaryAmplitude,
		secondary:     secondaryAmplitude,
		tertiary:      tertiaryAmplitude,
		adjust:        adjust,
		rpmPowerRatio: rpmPowerRatio,
	}
}

// getVehicleTypeAdjustments returns gain offset and amplitude scale based on vehicle type.
func (a *App) getVehicleTypeAdjustments() (float64, float64) {
	switch a.vehicle.VehicleType {
	case vehicle.TypeRace:
		return 0.0, 0.4
	case vehicle.TypeTuned:
		return -3.0, 0.25
	case vehicle.TypeStreet:
		fallthrough
	default:
		return -4.75, 0.02
	}
}

// generateTorqueWaveformSamples generates the actual waveform samples.
func (a *App) generateTorqueWaveformSamples(params *torqueWaveformParams, engineRoughness float64, engineBuffer *[]float64) {
	for index := range *engineBuffer {
		timePosition := float64(index) / params.sampleRate

		// Calculate phase components
		primaryComponent := params.primaryAmplitude * math.Sin(2.0*math.Pi*params.primaryTorqueFreq*timePosition)
		secondaryComponent := params.secondaryAmplitude * math.Sin(2.0*math.Pi*params.secondaryTorqueFreq*timePosition)
		tertiaryComponent := params.tertiaryAmplitude * math.Sin(2.0*math.Pi*params.tertiaryTorqueFreq*timePosition)

		// Combine components
		combinedTorque := primaryComponent + secondaryComponent + tertiaryComponent

		// Apply modulations
		combinedTorque = a.applyTorqueModulations(combinedTorque, engineRoughness, params, timePosition, index)

		// Apply final amplitude scaling and limits
		finalAmplitude := combinedTorque * params.adjust
		finalAmplitude, _ = signal.LimitWindow(finalAmplitude, -1.0, 1.0)

		(*engineBuffer)[index] = finalAmplitude
	}
}

// applyTorqueModulations applies various modulations to the combined torque signal.
func (a *App) applyTorqueModulations(combinedTorque, engineRoughness float64, params *torqueWaveformParams, timePosition float64, index int) float64 {
	// Add engine-specific roughness modulation
	roughnessPhase := (float64(a.state.current.sequenceNumber) + float64(index)) * 0.0003
	roughnessVariation := 1.0 + (engineRoughness * math.Sin(roughnessPhase) * 0.2)
	combinedTorque *= roughnessVariation

	// Apply RPM-dependent torque curve shaping
	rpmTorqueScaling := 1.0 + (params.rpmNormalized * 0.3)
	combinedTorque *= rpmTorqueScaling

	// Engine-specific torque curve modifications
	return a.applyEngineSpecificModulations(combinedTorque, params, timePosition)
}

// applyEngineSpecificModulations applies engine geometry-specific modulations.
func (a *App) applyEngineSpecificModulations(combinedTorque float64, params *torqueWaveformParams, timePosition float64) float64 {
	switch a.vehicle.Engine.Geometry {
	case "K":
		// Wankel rotary: Smoother torque curve with unique characteristics
		combinedTorque *= 0.8
		eccentricPhase := 2.0 * math.Pi * params.primaryTorqueFreq * timePosition * 0.33
		eccentricModulation := 1.0 + (0.1 * math.Sin(eccentricPhase))
		combinedTorque *= eccentricModulation
	case "S":
		// 2-stroke: More aggressive torque pulses with port effects
		combinedTorque *= 1.2
		portPhase := 2.0 * math.Pi * params.primaryTorqueFreq * timePosition * 2.0
		portModulation := 1.0 + (0.15 * math.Sin(portPhase))
		combinedTorque *= portModulation
	}

	return combinedTorque
}

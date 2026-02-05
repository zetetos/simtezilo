package app

import (
	"math"

	"github.com/zetetos/simtezilo/app/signal"
	"github.com/zetetos/simtezilo/app/synthesizer"
	"github.com/zetetos/simtezilo/app/vehicle"
)

// GenerateBalancedWaveform creates engine haptic waveforms based on primary and secondary balance characteristics
// with consideration for the 160Hz low-pass filter of the output device.
func (a *App) GenerateBalancedWaveform(rpm float64, engineRoughness float64, engineBuffer *[]float64) {
	sampleRate := float64(a.synth.GetSampleRate())
	rpmPercent := rpm / float64(a.vehicle.RevLimit)

	// Calculate frequencies with low-pass filter consideration
	primaryFreq, secondaryFreq := a.calculateBalancedFrequencies(rpm)

	// Calculate amplitude and gain settings
	amplitude := a.calculateBalancedAmplitude(rpm, engineRoughness)

	// Calculate balance contributions based on RPM
	secondaryContribution, primaryContribution := calculateBalanceContributions(rpmPercent)

	// Calculate imbalance and throttle factors
	primaryImbalance := 1.0 - a.vehicle.Engine.Haptics.PrimaryBalance
	secondaryImbalance := 1.0 - a.vehicle.Engine.Haptics.SecondaryBalance
	throttleScale := a.calculateThrottleScale()

	// Generate waveform samples
	a.generateBalancedSamples(engineBuffer, sampleRate, primaryFreq, secondaryFreq,
		primaryImbalance, secondaryImbalance, primaryContribution, secondaryContribution,
		throttleScale, amplitude, engineRoughness)
}

// calculateBalancedFrequencies calculates primary and secondary frequencies with filter consideration.
func (a *App) calculateBalancedFrequencies(rpm float64) (primaryFreq, secondaryFreq float64) {
	// Calculate base firing frequency and harmonics
	baseFiringRate := rpm * a.vehicle.Engine.FiringFrequency * a.vehicle.Engine.Haptics.PulseScale

	// Primary balance affects fundamental firing frequency
	primaryFreq = baseFiringRate

	// Secondary balance creates vibrations at intervals related to primary balance
	// Secondary imbalance creates vibrations at a slightly higher frequency (not double)
	// This represents reciprocating mass imbalance vibrations
	secondaryFreq = baseFiringRate * 1.3 // More subtle secondary frequency

	// Account for 160Hz low-pass filter - avoid generating significant content above this
	maxUsableFreq := 160.0
	if primaryFreq > maxUsableFreq {
		primaryFreq = maxUsableFreq
	}

	if secondaryFreq > maxUsableFreq {
		secondaryFreq = maxUsableFreq * 0.8 // Keep secondary below filter cutoff
	}

	return primaryFreq, secondaryFreq
}

// calculateBalancedAmplitude calculates the final amplitude for balanced waveform generation.
func (a *App) calculateBalancedAmplitude(rpm float64, engineRoughness float64) float64 {
	rpmPercent := rpm / float64(a.vehicle.RevLimit)
	throttlePercent := float64(a.gtClient.Telemetry.ThrottleOutputPercent()) / 100
	throttlePercent, _ = signal.LimitWindow(throttlePercent, 0.0, 1.0)

	gainOffset, amplitudeScale := a.getVehicleTypeSettings()

	// Generate amplitude aiming for signal max but respecting gain settings
	baseAmplitude := 0.9 + (throttlePercent * amplitudeScale)
	rpmNormalized, _ := signal.LimitWindow(rpmPercent, 0.0, 1.0)

	// Apply full gain control - this should be able to reduce volume significantly
	gainAdjust := synthesizer.GainToPowerRatio(a.vehicle.Engine.Haptics.Gain + gainOffset)

	// Calculate boost to reach signal max at 0dB gain, but scaled for proper gain response
	targetMaxAmplitude := 0.95 // Slightly below 1.0 to avoid clipping
	amplitudeBoost := targetMaxAmplitude / (baseAmplitude + (engineRoughness * rpmNormalized * 0.2))

	amplitude := (baseAmplitude + (engineRoughness * rpmNormalized * 0.2)) * gainAdjust * amplitudeBoost
	amplitude, _ = signal.LimitWindow(amplitude, 0, 1)

	return amplitude
}

// getVehicleTypeSettings returns gain offset and amplitude scale based on vehicle type.
func (a *App) getVehicleTypeSettings() (gainOffset, amplitudeScale float64) {
	switch a.vehicle.VehicleType {
	case vehicle.TypeRace:
		gainOffset = 0.0
		amplitudeScale = 0.3
	case vehicle.TypeTuned:
		gainOffset = -3.0
		amplitudeScale = 0.2
	case vehicle.TypeStreet:
		fallthrough
	default:
		gainOffset = -4.75
		amplitudeScale = 0.01
	}

	return gainOffset, amplitudeScale
}

// calculateBalanceContributions calculates secondary and primary balance contributions based on RPM.
func calculateBalanceContributions(rpmNormalized float64) (secondaryContribution, primaryContribution float64) {
	// Balance contribution factors based on RPM
	// Secondary balance dominates at low RPM, primary balance dominates at high RPM
	lowRpmThreshold := 0.3  // 30% of rev limit
	highRpmThreshold := 0.7 // 70% of rev limit

	switch {
	case rpmNormalized < lowRpmThreshold:
		// Low RPM: secondary balance dominates (much stronger contribution)
		secondaryContribution = 1.0
		primaryContribution = 0.8
	case rpmNormalized > highRpmThreshold:
		// High RPM: primary balance dominates (much stronger contribution)
		secondaryContribution = 0.6
		primaryContribution = 1.0
	default:
		// Mid RPM: transition between secondary and primary dominance
		transitionFactor := (rpmNormalized - lowRpmThreshold) / (highRpmThreshold - lowRpmThreshold)
		secondaryContribution = 1.0 - (transitionFactor * 0.4) // 1.0 to 0.6
		primaryContribution = 0.8 + (transitionFactor * 0.2)   // 0.8 to 1.0
	}

	return secondaryContribution, primaryContribution
}

// calculateThrottleScale calculates throttle-based vibration scaling.
func (a *App) calculateThrottleScale() float64 {
	throttlePercent := float64(a.gtClient.Telemetry.ThrottleOutputPercent()) / 100
	throttlePercent, _ = signal.LimitWindow(throttlePercent, 0.0, 1.0)

	// Higher throttle = more engine load = stronger vibrations
	// Lower throttle/idle = less engine load = weaker vibrations
	return 0.3 + (throttlePercent * 0.7) // 30% at idle, 100% at full throttle
}

// generateBalancedSamples generates the actual waveform samples for the balanced approach.
func (a *App) generateBalancedSamples(engineBuffer *[]float64, sampleRate, primaryFreq, secondaryFreq,
	primaryImbalance, secondaryImbalance, primaryContribution, secondaryContribution,
	throttleScale, amplitude, engineRoughness float64,
) {
	maxUsableFreq := 160.0

	for index := range *engineBuffer {
		timeOffset := float64(index) / sampleRate

		var waveformValue float64

		// Primary balance component - fundamental firing frequency
		if primaryFreq > 0 && primaryImbalance > 0.01 {
			primaryWave := a.generatePrimaryWave(primaryFreq, timeOffset, primaryImbalance, maxUsableFreq)
			waveformValue += primaryWave * (0.7 + primaryImbalance*0.3) * primaryContribution
		}

		// Secondary balance component - creates vibrations at interval of primary balance
		if secondaryFreq > 0 && secondaryImbalance > 0.01 {
			secondaryWave := a.generateSecondaryWave(secondaryFreq, timeOffset, secondaryImbalance)
			waveformValue += secondaryWave * (0.6 + secondaryImbalance*0.4) * secondaryContribution
		}

		// Add engine roughness similar to original implementation
		if engineRoughness > 0.01 {
			roughnessPhase := (float64(a.state.current.sequenceNumber) + float64(index)) * 0.001
			roughnessContribution := math.Sin(roughnessPhase) * engineRoughness * 0.1
			waveformValue += roughnessContribution
		}

		// Apply throttle-based scaling - more throttle = stronger vibrations (engine under load)
		waveformValue *= throttleScale

		// Ensure the magnitude stays within bounds
		waveformValue, _ = signal.LimitWindow(waveformValue, -1.0, 1.0)

		(*engineBuffer)[index] = amplitude * waveformValue
	}
}

// generatePrimaryWave generates engine-specific primary waveform characteristics.
func (a *App) generatePrimaryWave(primaryFreq, timeOffset, primaryImbalance, maxUsableFreq float64) float64 {
	primaryPhase := 2.0 * math.Pi * primaryFreq * timeOffset

	switch a.vehicle.Engine.Geometry {
	case "K": // Wankel
		// Triangular rotor creates smoother primary vibrations
		primaryWave := math.Sin(primaryPhase) * 0.8
		// Add rotor eccentricity harmonics
		primaryWave += math.Sin(primaryPhase*3.0) * 0.2 * primaryImbalance

		return primaryWave
	case "S": // 2-stroke
		// Sharp, aggressive primary pulses
		primaryWave := math.Sin(primaryPhase)
		// Add higher harmonic content for 2-stroke character
		if primaryFreq*2.0 < maxUsableFreq {
			primaryWave += math.Sin(primaryPhase*2.0) * 0.3 * primaryImbalance
		}

		return primaryWave
	default: // 4-stroke
		// Standard sinusoidal primary vibrations
		return math.Sin(primaryPhase)
	}
}

// generateSecondaryWave generates engine-specific secondary waveform characteristics.
func (a *App) generateSecondaryWave(secondaryFreq, timeOffset, secondaryImbalance float64) float64 {
	secondaryPhase := 2.0 * math.Pi * secondaryFreq * timeOffset

	switch a.vehicle.Engine.Geometry {
	case "K": // Wankel
		// Rotor housing vibrations - more complex waveform
		secondaryWave := math.Sin(secondaryPhase) * 0.6
		secondaryWave += math.Sin(secondaryPhase*1.5) * 0.4 * secondaryImbalance

		return secondaryWave
	case "S": // 2-stroke
		// Port scavenging creates irregular secondary vibrations
		secondaryWave := math.Sin(secondaryPhase) * 0.8
		secondaryWave += math.Sin(secondaryPhase*1.3) * 0.3 * secondaryImbalance

		return secondaryWave
	default: // 4-stroke
		// Standard secondary vibrations from reciprocating mass imbalance
		return math.Sin(secondaryPhase)
	}
}

package app

// TODO: this file is mostly LLM generated and needs heavy refactoring.

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/zetetos/simtezilo/app/config"
	"github.com/zetetos/simtezilo/app/haptics"
	"github.com/zetetos/simtezilo/app/signal"
	"github.com/zetetos/simtezilo/app/synthesizer"
	"github.com/zetetos/simtezilo/app/vehicle"
)

const maxPulseRate float64 = 300.0 // Max pulse rate for engine haptics

type engineState struct {
	lastSeq       uint32    // Last sequence ID for engine haptics
	lastKnownRPM  float64   // Cache last known RPM for fallback
	lastEventTime time.Time // Timestamp of last engine haptic event
	pulsePolarity bool      // Alternating polarity for engine pulses
}

// generateEngineHaptic creates a wavform to simulate engine vibrations.
func (a *App) generateEngineHaptic() {
	if !a.shouldGenerateEngineHaptic() {
		return
	}

	rpm := a.getCurrentRPM()
	if rpm < 0 {
		return // RPM unavailable due to telemetry timeout
	}

	engineBuffer, offset, lastPolarity := a.prepareEngineBuffer()

	// No haptics when engine is not running
	if rpm == 0 {
		a.synth.OverwriteBuffer(synthesizer.ChannelEngine, engineBuffer, offset)

		return
	}

	engineRoughness := a.calculateEngineRoughness(rpm)
	a.generatePulseWaveform(rpm, engineRoughness, &engineBuffer)

	// Alternative experimental waveform generation - comment out above line and uncomment below to test
	// a.generateBalancedWaveform(rpm, engineRoughness, &engineBuffer)

	// Engine-specific torque curve waveform - uncomment below to use torque curve based haptics
	// a.generateTorqueCurveWaveform(rpm, engineRoughness, &engineBuffer)

	a.adjustEngineBufferPolarity(engineBuffer, lastPolarity)
	a.synth.OverwriteBuffer(synthesizer.ChannelEngine, engineBuffer, offset)
}

// shouldGenerateEngineHaptic checks if engine haptic generation should proceed.
func (a *App) shouldGenerateEngineHaptic() bool {
	// Skip if in calibration mode
	if a.calibrator.IsEnabled() {
		return false
	}

	// Engine haptics are silenced
	if a.config.GetSynthEngineMute() {
		return false
	}

	// No engine haptics configured
	if a.vehicle.Engine.FiringFrequency == 0 {
		return false
	}

	if !a.state.telemetryActive {
		return false
	}

	return true
}

// getCurrentRPM gets the current RPM, managing cache for telemetry fallback.
func (a *App) getCurrentRPM() float64 {
	rpm := float64(a.gtClient.Telemetry.EngineRPM())
	currentTime := time.Now()

	switch {
	case a.state.current.sequenceNumber > a.state.engine.lastSeq:
		a.state.engine.lastKnownRPM = rpm
		a.state.engine.lastEventTime = currentTime
		a.state.engine.lastSeq = a.state.current.sequenceNumber

		return rpm
	case currentTime.Sub(a.state.engine.lastEventTime) > 1000*time.Millisecond:
		// stop engine haptics if no telemetry received for 1 second or more
		return -1
	default:
		// Use cached RPM if telemetry is unavailable
		return a.state.engine.lastKnownRPM
	}
}

// prepareEngineBuffer prepares the engine buffer for waveform generation.
func (a *App) prepareEngineBuffer() ([]float64, int, int) {
	// Generate engine vibration waveform for 2 frames worth of samples
	// This provides a small buffer to prevent underruns while keeping latency low
	sampleRate := float64(a.synth.GetSampleRate())
	samplesPerFrame := int(sampleRate / engineHapticFrameRate)
	bufferSamples := samplesPerFrame * 2
	engineBuffer := make([]float64, bufferSamples)

	offset := 0
	lastPolarity := 0
	lookback := 20

	// Stitch the new engine samples smoothly with the current buffer contents
	// Note: There's a small race window between Inspect and the subsequent OverwriteBuffer call.
	// The offset calculation may become slightly stale, but the impact is minimal (small audio glitch).
	// The offset is clamped to valid range to prevent out-of-bounds writes.
	inspectBuffer := a.synth.InspectChannelBuffer(synthesizer.ChannelEngine, samplesPerFrame+lookback, -lookback)
	if inspectBuffer != nil && len(inspectBuffer) >= samplesPerFrame {
		offset, lastPolarity = synthesizer.FindSampleZeroCrossing(inspectBuffer[lookback:samplesPerFrame])
	}

	// Clamp offset to valid range for the buffer we're about to write
	if offset >= bufferSamples {
		offset = 0
	}

	return engineBuffer, offset, lastPolarity
}

// adjustEngineBufferPolarity adjusts engine buffer polarity to be the inverse of the last pulse.
func (a *App) adjustEngineBufferPolarity(engineBuffer []float64, lastPolarity int) {
	lookback := 20
	if len(engineBuffer) < lookback {
		return
	}

	currentPolarity := synthesizer.SamplePolarity(engineBuffer[0:lookback])
	if currentPolarity == float64(lastPolarity) {
		synthesizer.InvertSamplePolarity(&engineBuffer)
	}
}

// getEngineCharacteristics retrieves engine characteristics based on a given engine geometry and speed.
func (a *App) getEngineCharacteristics(
	engineLayout string,
	cylinderAngle float32,
	crankPlaneAngle float32,
	revLimit uint16,
) (vehicle.EngineCharacteristics, error) {
	if engineLayout == "" {
		return vehicle.EngineCharacteristics{
			Haptics: &haptics.EngineProfile{},
		}, errors.New("engine layout not provided")
	}

	geometryCode := engineLayout[:1] // Get first character for geometry type

	chambers, err := strconv.Atoi(engineLayout[1:]) // Get remaining characters for chamber count
	if err != nil {
		return vehicle.EngineCharacteristics{}, err // Return error if conversion fails
	}

	characteristics := vehicle.EngineCharacteristics{
		Layout:          engineLayout,
		DBEntry:         "",
		Geometry:        geometryCode,
		Chambers:        chambers,
		RevLimit:        revLimit,
		FiringFrequency: getEngineFiringFrequency(geometryCode, chambers),
		PulseOverlap:    0.5 - calculatePulseOverlap(cylinderAngle, crankPlaneAngle, chambers, geometryCode),
		Haptics: &haptics.EngineProfile{
			PrimaryBalance:   1.0,
			SecondaryBalance: 1.0,
			Gain:             config.MinimumGain,
			PulseScale:       1.0,
		},
	}

	revRange := "std"

	switch {
	case revLimit > 13000:
		revRange = "high"
	case revLimit > 9000:
		revRange = "med"
	case revLimit < 6000:
		revRange = "low"
	}

	cylinderAngleStr := strconv.FormatFloat(float64(cylinderAngle), 'f', 0, 32)
	crankPlaneAngleStr := strconv.FormatFloat(float64(crankPlaneAngle), 'f', 0, 32)

	layoutVariations := []string{
		engineLayout + "_b" + cylinderAngleStr + "_c" + crankPlaneAngleStr + "_r" + revRange,
		engineLayout + "_b" + cylinderAngleStr + "_c" + crankPlaneAngleStr,
		engineLayout + "_c" + crankPlaneAngleStr + "_r" + revRange,
		engineLayout + "_c" + crankPlaneAngleStr,
		engineLayout + "_b" + cylinderAngleStr + "_r" + revRange,
		engineLayout + "_b" + cylinderAngleStr,
		engineLayout + "_r" + revRange,
		engineLayout,
	}

	for _, variation := range layoutVariations {
		profile := a.config.GetHapticsEngineProfile(variation)
		if profile == nil {
			continue
		}

		characteristics.DBEntry = variation
		characteristics.Haptics = profile

		break
	}

	return characteristics, nil
}

// getEngineFiringFrequency calculates the firing frequency based on engine geometry and chamber count.
func getEngineFiringFrequency(geometry string, chambers int) float64 {
	switch geometry {
	case "":
		return 0.0 // No engine haptics
	case "K": // Wankel rotary engines fire 3 times per rotor per revolution
		return (float64(chambers) * 3.0) / 60.0
	case "S": // Two stroke engines fire once per cylinder every revolution
		return float64(chambers) / 60.0
	default: // Four stroke engines fire once per cylinder every 2 revolutions
		return (float64(chambers) / 2.0) / 60.0
	}
}

// calculatePulseOverlap calculates pulse overlap based on alignment between crank plane angle and cylinder bank angle.
// Returns overlap factor from 0.0 (no overlap/perfect alignment) to 1.0 (maximum overlap/misalignment).
// This value is currently used for pulse width overlap. Timing clustering is disabled pending better implementation.
// - Low values (0.0-0.2): Well-aligned engines (e.g., 60° V6 with 120° crank).
// - High values (0.3-0.8): Misaligned engines (e.g., 90° V6 with 120° crank).
func calculatePulseOverlap(cylinderAngle, crankPlaneAngle float32, chambers int, geometry string) float64 {
	// Handle special cases first
	if overlap := getSpecialCaseOverlap(geometry, chambers); overlap >= 0 {
		return overlap
	}

	// Calculate firing offset and normalize it
	firingOffset := normalizeFiringOffset(cylinderAngle, crankPlaneAngle)

	// Calculate base overlap and alignment factor based on geometry
	baseOverlap, alignmentFactor := calculateGeometryBasedOverlap(geometry, chambers, firingOffset)

	// Apply chamber count scaling
	chamberScale := calculateChamberScale(chambers, geometry)

	// Calculate final overlap and clamp to valid range
	finalOverlap := baseOverlap * alignmentFactor * chamberScale

	return clampOverlap(finalOverlap)
}

// getSpecialCaseOverlap handles special cases that don't require complex calculations.
// Returns -1 if not a special case, otherwise returns the overlap value.
func getSpecialCaseOverlap(geometry string, chambers int) float64 {
	// Wankel overlap based on rotor count and housing design
	if geometry == "K" {
		if chambers > 1 {
			return 0.15 + (float64(chambers-1) * 0.05) // 15-25% overlap for multi-rotor
		}

		return 0.05 // Single rotor has minimal overlap
	}

	// Single cylinder engines have no overlap
	if chambers <= 1 {
		return 0.0
	}

	return -1
}

// normalizeFiringOffset calculates and normalizes the firing offset angle.
func normalizeFiringOffset(cylinderAngle, crankPlaneAngle float32) float64 {
	firingOffset := math.Abs(float64(cylinderAngle - crankPlaneAngle))

	// Normalize to 0-180 degree range using modulus (angles are symmetric)
	return math.Mod(firingOffset, 180.0)
}

// calculateGeometryBasedOverlap calculates base overlap and alignment factor based on engine geometry.
func calculateGeometryBasedOverlap(geometry string, chambers int, firingOffset float64) (baseOverlap, alignmentFactor float64) {
	switch geometry {
	case "K": // Wankel engines have unique firing characteristics
		return calculateWankelOverlap(chambers)
	case "S": // 2-strokes fire every revolution, creating more natural overlap
		return calculateTwoStrokeOverlap(firingOffset)
	default: // 4-strokes fire every other revolution, creating different overlap characteristics
		return calculateFourStrokeOverlap(chambers, firingOffset)
	}
}

// calculateWankelOverlap calculates overlap for Wankel engines.
func calculateWankelOverlap(chambers int) (baseOverlap, alignmentFactor float64) {
	baseOverlap = 0.2 // Lower base overlap due to rotor design

	// Cylinder count affects overlap potential
	cylinderFactor := math.Min(float64(chambers)/4.0, 1.0)
	baseOverlap *= (0.5 + cylinderFactor*0.5)
	alignmentFactor = 1.0 // Wankels don't use alignment factor in current logic

	return baseOverlap, alignmentFactor
}

// calculateTwoStrokeOverlap calculates overlap for 2-stroke engines.
func calculateTwoStrokeOverlap(firingOffset float64) (baseOverlap, alignmentFactor float64) {
	baseOverlap = 0.3 // Higher base overlap due to rapid firing

	alignmentFactor = getTwoStrokeAlignmentFactor(firingOffset)

	return baseOverlap, alignmentFactor
}

// getTwoStrokeAlignmentFactor calculates alignment factor for 2-stroke engines based on firing offset.
func getTwoStrokeAlignmentFactor(firingOffset float64) float64 {
	switch {
	case firingOffset <= 15.0:
		// Near-perfect alignment: minimal overlap due to synchronized firing
		return 0.3
	case firingOffset >= 75.0 && firingOffset <= 105.0:
		// Perpendicular arrangement: maximum overlap
		return 1.0
	case firingOffset < 45.0:
		// Progressive alignment: interpolate between min and max
		return 0.3 + ((firingOffset-15.0)/30.0)*0.4 // 0.3 to 0.7
	default:
		return 0.7 + ((75.0-firingOffset)/30.0)*0.3 // 0.7 to 1.0
	}
}

// calculateFourStrokeOverlap calculates overlap for 4-stroke engines.
func calculateFourStrokeOverlap(chambers int, firingOffset float64) (baseOverlap, alignmentFactor float64) {
	baseOverlap = 0.2 // Lower base overlap due to spaced firing intervals

	// Cylinder count affects overlap potential
	cylinderFactor := math.Min(float64(chambers)/8.0, 1.0) // Normalize to 8-cylinder reference
	baseOverlap *= (0.5 + cylinderFactor*0.5)              // Scale: 50% to 100% of base

	alignmentFactor = getFourStrokeAlignmentFactor(firingOffset)

	return baseOverlap, alignmentFactor
}

// getFourStrokeAlignmentFactor calculates alignment factor for 4-stroke engines based on firing offset.
func getFourStrokeAlignmentFactor(firingOffset float64) float64 {
	switch {
	case firingOffset <= 10.0:
		// Near-perfect alignment: synchronized banks, minimal overlap
		return 0.2
	case firingOffset >= 80.0 && firingOffset <= 100.0:
		// Near-perpendicular: optimal staggered firing, maximum overlap
		return 1.0
	case firingOffset >= 170.0:
		// Near-opposite: boxer-style layout, minimal overlap
		return 0.1
	case firingOffset < 45.0:
		// Moving away from alignment toward staggered
		return 0.2 + ((firingOffset-10.0)/35.0)*0.5 // 0.2 to 0.7
	case firingOffset < 90.0:
		// Approaching optimal stagger
		return 0.7 + ((firingOffset-45.0)/35.0)*0.3 // 0.7 to 1.0
	case firingOffset < 135.0:
		// Moving past optimal toward opposite
		return 1.0 - ((firingOffset-90.0)/45.0)*0.6 // 1.0 to 0.4
	default:
		// Approaching opposite layout
		return 0.4 - ((firingOffset-135.0)/35.0)*0.3 // 0.4 to 0.1
	}
}

// calculateChamberScale applies chamber count scaling for engines with many cylinders.
func calculateChamberScale(chambers int, geometry string) float64 {
	switch {
	case chambers >= 8:
		// More cylinders = more potential for overlap
		return 1.0 + (float64(chambers-8) * 0.05) // +5% per cylinder above 8
	case chambers <= 4 && geometry != "S":
		// Fewer cylinders = less overlap potential (except 2-strokes)
		return 0.6 + (float64(chambers-2) * 0.2) // 60% for 2-cyl, 80% for 3-cyl, 100% for 4-cyl
	default:
		return 1.0
	}
}

// clampOverlap ensures overlap stays within reasonable bounds.
func clampOverlap(overlap float64) float64 {
	switch {
	case overlap > 0.8:
		return 0.8 // Maximum 80% overlap
	case overlap < 0.0:
		return 0.0 // No negative overlap
	default:
		return overlap
	}
}

// calculateEngineRoughness calculates a roughness value based on the engine geometry and a given RPM.
func (a *App) calculateEngineRoughness(rpm float64) float64 {
	var engineRoughness float64

	switch a.vehicle.Engine.Geometry {
	case "K":
		roughnessPhase := float64(a.state.current.sequenceNumber) * 0.003
		apexSealRoughness := (1.0 - a.vehicle.Engine.Haptics.PrimaryBalance) * 0.08
		housingEccentricity := (1.0 - a.vehicle.Engine.Haptics.SecondaryBalance) * 0.05
		roughnessIntensity := apexSealRoughness + housingEccentricity*0.7

		// Wankels get smoother at higher RPM due to improved sealing
		rpmSmoothingFactor := math.Min(rpm/4000.0, 1.0)
		engineRoughness = math.Sin(roughnessPhase) * roughnessIntensity * (1.0 - rpmSmoothingFactor*0.8)

		// Add characteristic Wankel "chatter" at low RPM
		if rpm < 2000.0 {
			chatterPhase := float64(a.state.current.sequenceNumber) * 0.008
			chatterIntensity := (1.0 - a.vehicle.Engine.Haptics.SecondaryBalance) * 0.03
			engineRoughness += math.Sin(chatterPhase) * chatterIntensity * (1.0 - rpm/2000.0)
		}
	case "S":
		// 2-stroke engines have characteristic roughness due to scavenging process
		roughnessPhase := float64(a.state.current.sequenceNumber) * 0.007
		scavenging := (1.0 - a.vehicle.Engine.Haptics.PrimaryBalance) * 0.20
		exhaustBlowdown := (1.0 - a.vehicle.Engine.Haptics.SecondaryBalance) * 0.12
		intakeExhaustoverlap := 0.6
		baseRoughness := scavenging + exhaustBlowdown*intakeExhaustoverlap

		// Add characteristic 2-stroke "buzz" - more intense at mid RPM
		rpmFactor := math.Min(rpm/6000.0, 1.0)
		buzzIntensity := baseRoughness * (0.5 + rpmFactor*0.8)

		engineRoughness = math.Sin(roughnessPhase)*buzzIntensity + math.Sin(roughnessPhase*2.3)*buzzIntensity*0.4

		// Add port timing irregularities at low RPM
		if rpm < 3000.0 {
			portPhase := float64(a.state.current.sequenceNumber) * 0.012
			portIrregularity := (1.0 - a.vehicle.Engine.Haptics.SecondaryBalance) * 0.08
			engineRoughness += math.Sin(portPhase) * portIrregularity * (1.0 - rpm/3000.0)
		}

		// 2-strokes get slightly smoother at very high RPM due to better scavenging
		if rpm > 4000.0 {
			smoothing := math.Min((rpm-4000.0)/4000.0, 0.3)
			engineRoughness *= (1.0 - smoothing)
		}
	default:
		// Default to 4-stroke engine characteristics
		if rpm <= 2400.0 {
			roughnessPhase := float64(a.state.current.sequenceNumber) * 0.005
			// Poor primary balance creates more low-frequency roughness
			primaryRoughness := (1.0 - a.vehicle.Engine.Haptics.PrimaryBalance) * 0.15
			// Poor secondary balance creates more high-frequency roughness
			secondaryRoughness := (1.0 - a.vehicle.Engine.Haptics.SecondaryBalance) * 0.08
			roughnessIntensity := primaryRoughness + secondaryRoughness*0.5
			engineRoughness = math.Sin(roughnessPhase)*roughnessIntensity + math.Sin(roughnessPhase*1.7)*roughnessIntensity*0.5

			// Smooth out roughness as RPM increases
			rpmSmoothingFactor := rpm / 2400.0
			engineRoughness *= (1.0 - rpmSmoothingFactor*a.vehicle.Engine.Haptics.PrimaryBalance)
		} else {
			// High RPM: roughness based on engine balance characteristics
			if a.vehicle.Engine.Haptics.PrimaryBalance < 0.9 {
				roughnessPhase := float64(a.state.current.sequenceNumber) * 0.002
				highRpmRoughness := (1.0 - a.vehicle.Engine.Haptics.PrimaryBalance) * 0.02 // Poor primary balance creates roughness
				engineRoughness = math.Sin(roughnessPhase) * highRpmRoughness
			} else {
				engineRoughness = 0.0 // Well-balanced engines are smooth at high RPM
			}
		}
	}

	return engineRoughness
}

// generatePulseWaveform creates a vibration pulse waveform based on engine RPM and roughness characteristics.
func (a *App) generatePulseWaveform(rpm float64, engineRoughness float64, engineBuffer *[]float64) {
	params := a.calculatePulseWaveformParams(rpm, engineRoughness)
	a.generatePulseWaveformSamples(params, engineBuffer)
}

// pulseWaveformParams holds all calculated parameters for pulse waveform generation.
type pulseWaveformParams struct {
	rpmPercent      float64
	throttlePercent float64
	amplitude       float64
	sampleRate      float64
	pulseRate       float64
	pulseDutyCycle  float64
}

// calculatePulseWaveformParams calculates all parameters needed for pulse waveform generation.
func (a *App) calculatePulseWaveformParams(rpm, engineRoughness float64) *pulseWaveformParams {
	rpmPercent := rpm / float64(a.vehicle.RevLimit)
	rpmPercent, _ = signal.LimitWindow(rpmPercent, 0.0, 1.0)

	throttlePercent := float64(a.gtClient.Telemetry.ThrottleOutputPercent()) / 100
	throttlePercent, _ = signal.LimitWindow(throttlePercent, 0.0, 1.0)

	vehicleTypeGain := a.getVehicleTypeGainOffset()
	amplitude := a.calculatePulseAmplitude(throttlePercent, engineRoughness, rpmPercent, vehicleTypeGain)

	sampleRate := float64(a.synth.GetSampleRate())
	pulseRate := rpm * a.vehicle.Engine.FiringFrequency * a.vehicle.Engine.Haptics.PulseScale
	pulseDutyCycle := a.vehicle.Engine.PulseOverlap + (rpmPercent * a.vehicle.Engine.PulseOverlap * 2)

	return &pulseWaveformParams{
		rpmPercent:      rpmPercent,
		throttlePercent: throttlePercent,
		amplitude:       amplitude,
		sampleRate:      sampleRate,
		pulseRate:       pulseRate,
		pulseDutyCycle:  pulseDutyCycle,
	}
}

// getVehicleTypeGainOffset returns the gain offset based on vehicle type.
func (a *App) getVehicleTypeGainOffset() float64 {
	switch a.vehicle.VehicleType {
	case vehicle.TypeRace:
		return 0.0
	case vehicle.TypeTuned:
		return -3.0
	case vehicle.TypeStreet:
		fallthrough
	default:
		return -4.75
	}
}

// calculatePulseAmplitude calculates the pulse amplitude based on engine characteristics.
func (a *App) calculatePulseAmplitude(throttlePercent, engineRoughness, rpmPercent, vehicleTypeGain float64) float64 {
	engineLoadGainIncrease := 1.0
	engineLoadGain := (1 - throttlePercent) * engineLoadGainIncrease
	roughness := 1.0 - (engineRoughness * rpmPercent * 0.1)
	gain := a.vehicle.Engine.Haptics.Gain + vehicleTypeGain + engineLoadGain
	amplitude := synthesizer.GainToPowerRatio(gain) * roughness
	amplitude, _ = signal.LimitWindow(amplitude, 0, 1)

	return amplitude
}

// generatePulseWaveformSamples generates the actual pulse waveform samples.
func (a *App) generatePulseWaveformSamples(params *pulseWaveformParams, engineBuffer *[]float64) {
	for index := range *engineBuffer {
		pulseValue := a.calculatePulseValueAtIndex(index, params)
		(*engineBuffer)[index] = params.amplitude * pulseValue
	}
}

// calculatePulseValueAtIndex calculates the pulse value at a specific sample index.
func (a *App) calculatePulseValueAtIndex(index int, params *pulseWaveformParams) float64 {
	samplesPerPulse := params.sampleRate / params.pulseRate
	pulsePosition := float64(index) / samplesPerPulse

	// Detect pulse trigger point and manage polarity
	a.updatePulsePolarityIfNeeded(index, samplesPerPulse)

	pulsePhase := pulsePosition - math.Floor(pulsePosition) // 0.0 to 1.0 within each pulse cycle
	if pulsePhase >= params.pulseDutyCycle {
		return 0.0 // Outside the pulse
	}

	// Inside the pulse - create a sharp, distinct pulse
	pulsePhaseNormalized := pulsePhase / params.pulseDutyCycle // 0.0 to 1.0 within pulse width
	pulseValue := a.generatePulseValueByGeometry(pulsePhaseNormalized)

	// Apply polarity - alternating positive and negative pulses
	if !a.state.engine.pulsePolarity {
		pulseValue = -pulseValue
	}

	// Add roughness variation
	pulseValue = a.applyPulseRoughnessVariation(pulseValue, index, params.rpmPercent)

	// Ensure the magnitude stays within bounds
	pulseValue, _ = signal.LimitWindow(pulseValue, -1.0, 1.0)

	return pulseValue
}

// updatePulsePolarityIfNeeded updates the pulse polarity when crossing into a new pulse cycle.
func (a *App) updatePulsePolarityIfNeeded(index int, samplesPerPulse float64) {
	pulsePosition := float64(index) / samplesPerPulse
	currentPulseIndex := int(math.Floor(pulsePosition))

	var lastPulseIndex int

	if index > 0 {
		lastPulsePosition := float64(index-1) / samplesPerPulse
		lastPulseIndex = int(math.Floor(lastPulsePosition))
	} else {
		lastPulseIndex = -1
	}

	// Check if the waveform has crossed into a new pulse cycle
	pulseTriggered := (index > 0) && (currentPulseIndex != lastPulseIndex)
	if pulseTriggered {
		a.state.engine.pulsePolarity = !a.state.engine.pulsePolarity
	}
}

// generatePulseValueByGeometry generates pulse value based on engine geometry.
func (a *App) generatePulseValueByGeometry(pulsePhaseNormalized float64) float64 {
	switch a.vehicle.Engine.Geometry {
	case "K":
		return generatePulseWankel(pulsePhaseNormalized, a.vehicle.Engine.Haptics)
	case "S":
		return generatePulseTwoStroke(pulsePhaseNormalized, a.vehicle.Engine.Haptics)
	default:
		return generatePulseFourStroke(pulsePhaseNormalized, a.vehicle.Engine.Haptics)
	}
}

// applyPulseRoughnessVariation applies roughness variation to the pulse value.
func (a *App) applyPulseRoughnessVariation(pulseValue float64, index int, rpmPercent float64) float64 {
	secondaryImbalance := 1.0 - a.vehicle.Engine.Haptics.SecondaryBalance
	rpm := rpmPercent * float64(a.vehicle.RevLimit)

	if rpm <= 2400.0 && secondaryImbalance > 0.02 {
		roughnessPhase := (float64(a.state.current.sequenceNumber) + float64(index)) * 0.0005
		roughnessVariation := 1.0 + (math.Sin(roughnessPhase) * secondaryImbalance * 0.3)
		pulseValue *= roughnessVariation
	}

	return pulseValue
}

// generatePulswWankel creates a single pulse value for a Wankel engine based on a given phase value
// and engine geometry.
func generatePulseWankel(phase float64, engine *haptics.EngineProfile) (pulse float64) {
	if phase < 0.3 {
		// Quick attack (30% of pulse width)
		// Adjust attack characteristics based on engine balance
		// Poor balance = sharper attack, good balance = smoother attack
		attackSharpness := 1.0 - (engine.PrimaryBalance * 0.6) // 0.4 to 1.0 range
		attackPhase := phase / 0.3

		// Wankels have unique triangular rotor pulses - more gradual attack
		attackSharpness *= 0.7 // Reduce sharpness for Wankel characteristic
		pulse = math.Sin(attackPhase*math.Pi/2) * attackSharpness

		// Add slight rotor eccentricity modulation
		eccentricityFactor := 1.0 + (1.0-engine.SecondaryBalance)*0.1*math.Sin(attackPhase*math.Pi*3)
		pulse *= eccentricityFactor
	} else {
		// Quick decay (70% of pulse width)
		// Adjust decay based on both primary and secondary balance
		decayPhase := (phase - 0.3) / 0.7

		// Wankel decay characteristics - smoother, more gradual
		primaryDecayRate := 3.0 + (1.0-engine.PrimaryBalance)*1.5
		secondaryDecayFactor := 1.0 + (1.0-engine.SecondaryBalance)*0.3
		combinedDecayRate := primaryDecayRate * secondaryDecayFactor

		// Wankels have more gradual decay due to chamber expansion characteristics
		pulse = math.Exp(-decayPhase*combinedDecayRate) * (1.0 - decayPhase*0.1)
	}

	return pulse
}

// generatePulswTwoStroke creates a single pulse value for a 2-strok engine based on a given phase value
// and engine geometry.
func generatePulseTwoStroke(phase float64, engine *haptics.EngineProfile) (pulse float64) {
	if phase < 0.3 {
		// Quick attack (30% of pulse width)
		// Adjust attack characteristics based on engine balance
		// Poor balance = sharper attack, good balance = smoother attack
		attackSharpness := 1.0 - (engine.PrimaryBalance * 0.6) // 0.4 to 1.0 range
		attackPhase := phase / 0.3

		// 2-stroke engines have very sharp, aggressive attack due to rapid combustion
		attackSharpness *= 1.3 // Increase sharpness for 2-stroke characteristic

		// 2-strokes have a more explosive attack than 4-strokes
		pulse = math.Pow(math.Sin(attackPhase*math.Pi/2), attackSharpness*0.6)

		// Add port opening/closing noise during attack
		portNoise := (1.0 - engine.SecondaryBalance) * 0.15
		portPhase := attackPhase * math.Pi * 4 // Higher frequency port effects
		pulse += math.Sin(portPhase) * portNoise * attackPhase
	} else {
		// Quick decay (70% of pulse width)
		// Adjust decay based on both primary and secondary balance
		decayPhase := (phase - 0.3) / 0.7

		// 2-stroke decay characteristics - rapid but irregular due to exhaust blowdown
		primaryDecayRate := 5.0 + (1.0-engine.PrimaryBalance)*3.0
		secondaryDecayFactor := 1.0 + (1.0-engine.SecondaryBalance)*0.8

		combinedDecayRate := primaryDecayRate * secondaryDecayFactor

		// 2-strokes have rapid decay with exhaust port effects
		baseDecay := math.Exp(-decayPhase * combinedDecayRate)

		// Add exhaust blowdown characteristics - creates a "ragged" decay
		blowdownIntensity := (1.0 - engine.SecondaryBalance) * 0.25
		blowdownPhase := decayPhase * math.Pi * 3 // Higher frequency for port effects
		blowdownEffect := math.Sin(blowdownPhase) * blowdownIntensity * (1.0 - decayPhase)

		pulse = baseDecay + blowdownEffect
	}

	return pulse
}

// generatePulswFourStroke creates a single pulse value for a 4-stroke engine based on a given phase value
// and engine geometry.
func generatePulseFourStroke(phase float64, engine *haptics.EngineProfile) (pulse float64) {
	if phase < 0.3 {
		// Quick attack (30% of pulse width)
		// Adjust attack characteristics based on engine balance
		// Poor balance = sharper attack, good balance = smoother attack
		attackSharpness := 1.0 - (engine.PrimaryBalance * 0.6) // 0.4 to 1.0 range
		attackPhase := phase / 0.3

		pulse = math.Sin(attackPhase * math.Pi / 2)

		// Use different attack curves based on engine balance
		if engine.PrimaryBalance < 0.7 {
			// Sharp, aggressive attack for poorly balanced engines
			pulse = math.Pow(pulse, attackSharpness)
		} else {
			// Smoother attack for well-balanced engines
			pulse *= attackSharpness
		}
	} else {
		// Quick decay (70% of pulse width)
		// Adjust decay based on both primary and secondary balance
		decayPhase := (phase - 0.3) / 0.7

		// Primary balance affects base decay rate
		primaryDecayRate := 4.0 + (1.0-engine.PrimaryBalance)*2.0

		// Secondary balance affects decay smoothness
		secondaryDecayFactor := 1.0 + (1.0-engine.SecondaryBalance)*0.5

		combinedDecayRate := primaryDecayRate * secondaryDecayFactor
		pulse = math.Exp(-decayPhase * combinedDecayRate)
	}

	return pulse
}

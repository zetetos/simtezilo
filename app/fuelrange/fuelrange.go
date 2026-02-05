// Package fuelrange provides an estimator for the distance that can be travelled with the current fuel level.
package fuelrange

import (
	"github.com/rs/zerolog"
)

const (
	// Initial fuel level in percent.
	initialFuelLevel float64 = -1.0

	// Initial odometer reading in metres.
	initialOdometerReading float64 = -1.0

	// Default (very high) range in laps when unknown.
	rangeLapsUnknown float64 = 10000

	// Default (unknown) range in metres.
	rangeDistanceUnknown float64 = rangeLapsUnknown * 1000

	// Minimum number of samples required to provide a reliable range estimate (~120s).
	fuelRangeMinSamples int = 7200

	// Number of samples to store in the buffer (~120s).
	fuelRangeMaxSamples int = 7200
)

// Estimator defines the interface for fuel range calculations.
type Estimator interface {
	Reset()
	ResetEstimate()
	IsReady() bool
	SetLive(isLive bool)
	Update(odometerReading float64, fuelLevel float32)
	DistanceMetres() float64
	DistanceLaps(lengthMetres float64) float64
	UsageRatePerKm() float64
}

type fuelRangeSample struct {
	fuelPercent float32 // Fuel consumption sample in percent per km
	odometer    float64 // Odometer reading at the time of the sample
}

// FuelRange estimates the distance that can be travelled with the current fuel level.
// Conforms to the fuelrange.Estimator interface.
type FuelRange struct {
	log                     zerolog.Logger
	lastOdometerReading     float64           // Last processed odometer reading in metres
	fuelLevelAtLastUpdate   float64           // Last processed fuel level in percent
	distanceSinceLastUpdate float64           // Distance in metres travelled since last fuel range update
	fuelRateSamples         []fuelRangeSample // Buffer of fuel consumption samples
	fuelRate                float64           // Moving average fuel consumption rate in percent per km
	distanceMetres          float64           // Estimated distance in metres that can be travelled with current fuel level
	refueling               bool              // Flag to indicate if refuelling is in progress
	isLive                  bool              // Flag to indicate if a session is live or a replay
	maxSamples              int               // Maximum number of samples to store in the buffer
	minSamples              int               // Minimum number of samples required to provide a reliable range estimate
}

// New creates a new fuel range estimator.
func New(logger zerolog.Logger) *FuelRange {
	fuelRange := FuelRange{
		log: logger.With().Str("package", "fuel").Logger(),
	}

	fuelRange.Reset()

	return &fuelRange
}

// Reset clears the internal state of the fuel range estimator.
func (r *FuelRange) Reset() {
	r.fuelLevelAtLastUpdate = initialFuelLevel
	r.distanceSinceLastUpdate = 0
	r.fuelRate = 0
	r.distanceMetres = rangeDistanceUnknown
	r.refueling = false

	minSamples := fuelRangeMinSamples
	maxSamples := fuelRangeMaxSamples

	// Replays reduce fuel samples by ~100x compared to a live session
	if !r.isLive {
		minSamples /= 100
		maxSamples /= 100
	}

	r.minSamples = minSamples
	r.maxSamples = maxSamples
	r.fuelRateSamples = make([]fuelRangeSample, 0, r.maxSamples)

	r.log.Debug().
		Msg("Fuel range reset")
}

// ResetEstimate resets only the fuel range estimate but retains odometer and samples.
func (r *FuelRange) ResetEstimate() {
	r.distanceMetres = rangeDistanceUnknown
	r.fuelLevelAtLastUpdate = initialFuelLevel
	r.lastOdometerReading = initialOdometerReading
	r.distanceSinceLastUpdate = 0
	r.fuelRate = 0
	r.fuelRateSamples = make([]fuelRangeSample, 0, r.maxSamples)

	r.log.Debug().
		Msg("Fuel range estimate reset")
}

// SetLive sets the isLive flag to indicate if the current session is live or a replay.
func (r *FuelRange) SetLive(isLive bool) {
	if r.isLive == isLive {
		return
	}

	r.isLive = isLive

	r.Reset()

	r.log.Debug().
		Bool("is_live", isLive).
		Msg("Set fuel range live status")
}

// IsReady returns true if enough samples have been collected to provide a reliable range estimate.
func (r *FuelRange) IsReady() bool {
	return len(r.fuelRateSamples) >= r.minSamples
}

// Update updates fuel consumption based on the current coordinate and fuel level.
func (r *FuelRange) Update(odometerReading float64, fuelLevel float32) {
	if r.shouldInitialize(odometerReading, fuelLevel) {
		return
	}

	if r.shouldResetOnOdometerRollback(odometerReading) {
		return
	}

	r.updateDistance(odometerReading)
	consumed := r.fuelConsumed(float64(fuelLevel))

	r.updateFuelRateIfConsuming(consumed, fuelLevel, odometerReading)
	r.checkRefuelCompletion(consumed, fuelLevel)
	r.updateFuelLevelTracking(consumed, fuelLevel)
	r.detectRefueling(consumed, fuelLevel)
}

// DistanceMetres returns the estimated distance in metres that can be travelled with current fuel level.
func (r *FuelRange) DistanceMetres() float64 {
	// Fuel rate not available
	if r.fuelRate <= 0 {
		return rangeDistanceUnknown
	}

	// Not enough samples to provide a reliable estimate
	if len(r.fuelRateSamples) < r.minSamples {
		return rangeDistanceUnknown
	}

	return r.distanceMetres - r.distanceSinceLastUpdate
}

// DistanceLaps returns the estimated number of laps that can be completed on a circuit of given length.
func (r *FuelRange) DistanceLaps(lengthMetres float64) float64 {
	// Invalid circuit length
	if lengthMetres <= 0 {
		return rangeLapsUnknown
	}

	distanceMetres := r.DistanceMetres()

	return distanceMetres / lengthMetres
}

// UsageRatePerKm returns the current fuel consumption rate in percent per km.
func (r *FuelRange) UsageRatePerKm() float64 {
	return r.fuelRate * 1000
}

// fuelConsumed returns the fuel consumed since the last update.
func (r *FuelRange) fuelConsumed(currentFuelLevel float64) float64 {
	consumed := r.fuelLevelAtLastUpdate - currentFuelLevel

	return consumed
}

// fuelRateMA returns the moving average fuel consumption rate in percent per meter.
func (r *FuelRange) fuelRateMA() float64 {
	if len(r.fuelRateSamples) < 2 {
		return 0
	}

	first := r.fuelRateSamples[0]
	last := r.fuelRateSamples[len(r.fuelRateSamples)-1]

	distance := last.odometer - first.odometer

	if distance <= 0 {
		return 0
	}

	consumed := first.fuelPercent - last.fuelPercent

	return float64(consumed) / distance
}

// shouldInitialize checks if this is the first update and initializes if needed.
func (r *FuelRange) shouldInitialize(odometerReading float64, fuelLevel float32) bool {
	if r.fuelLevelAtLastUpdate == initialFuelLevel || r.lastOdometerReading == initialOdometerReading {
		r.fuelLevelAtLastUpdate = float64(fuelLevel)
		r.lastOdometerReading = odometerReading

		return true
	}

	return false
}

// shouldResetOnOdometerRollback checks if odometer has rolled back and resets if needed.
func (r *FuelRange) shouldResetOnOdometerRollback(odometerReading float64) bool {
	if odometerReading < r.lastOdometerReading {
		r.log.Debug().
			Float64("last_odometer", r.lastOdometerReading).
			Float64("current_odometer", odometerReading).
			Msg("Odometer reset detected")

		r.ResetEstimate()

		return true
	}

	return false
}

// updateDistance updates the distance tracking.
func (r *FuelRange) updateDistance(odometerReading float64) {
	r.distanceSinceLastUpdate += odometerReading - r.lastOdometerReading
	r.lastOdometerReading = odometerReading
}

// updateFuelRateIfConsuming updates fuel rate calculations if fuel is being consumed.
func (r *FuelRange) updateFuelRateIfConsuming(consumed float64, fuelLevel float32, odometerReading float64) {
	if !(consumed > 0 && r.distanceSinceLastUpdate > 0) {
		return
	}

	r.addFuelRateSample(fuelLevel, odometerReading)
	r.calculateNewFuelRate(consumed)
	r.resetDistanceAndFuelLevel(fuelLevel)
}

// addFuelRateSample adds a new fuel rate sample, removing oldest if at capacity.
func (r *FuelRange) addFuelRateSample(fuelLevel float32, odometerReading float64) {
	if len(r.fuelRateSamples) >= fuelRangeMaxSamples {
		r.fuelRateSamples = r.fuelRateSamples[1:]
	}

	r.fuelRateSamples = append(r.fuelRateSamples, fuelRangeSample{
		fuelPercent: fuelLevel,
		odometer:    odometerReading,
	})
}

// calculateNewFuelRate calculates the new fuel rate and distance estimates.
func (r *FuelRange) calculateNewFuelRate(consumed float64) {
	r.fuelRate = r.fuelRateMA()
	r.distanceMetres = float64(r.fuelRateSamples[len(r.fuelRateSamples)-1].fuelPercent) / r.fuelRate

	r.log.Debug().
		Float64("fuel_rate", r.fuelRate).
		Float64("consumed", consumed).
		Float64("distance_m", r.distanceSinceLastUpdate).
		Float64("range_m", r.distanceMetres).
		Int("samples", len(r.fuelRateSamples)).
		Msg("Update estimated fuel range")
}

// resetDistanceAndFuelLevel resets tracking variables after successful update.
func (r *FuelRange) resetDistanceAndFuelLevel(fuelLevel float32) {
	r.distanceSinceLastUpdate = 0
	r.fuelLevelAtLastUpdate = float64(fuelLevel)
}

// checkRefuelCompletion checks if refueling has completed.
func (r *FuelRange) checkRefuelCompletion(consumed float64, fuelLevel float32) {
	if r.refueling && consumed > 0 {
		r.log.Debug().
			Float32("fuel_level", fuelLevel).
			Float64("last_consumed", consumed).
			Msg("Refuel complete")
		r.refueling = false
	}
}

// updateFuelLevelTracking updates fuel level tracking for subsequent calculations.
func (r *FuelRange) updateFuelLevelTracking(consumed float64, fuelLevel float32) {
	if !(consumed > 0 && r.distanceSinceLastUpdate > 0) {
		r.fuelLevelAtLastUpdate = float64(fuelLevel)
	}
}

// detectRefueling detects if refueling is occurring.
func (r *FuelRange) detectRefueling(consumed float64, fuelLevel float32) {
	if consumed < -0.05 && !r.refueling {
		// Detect refuelling (-0.05 gives margin to avoid occasional noise)
		r.ResetEstimate()
		r.refueling = true

		r.log.Debug().
			Float32("fuel_level", fuelLevel).
			Float64("fuel_consumed", consumed).
			Msg("Refuel detected")
	}
}

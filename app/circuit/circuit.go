// Package circuit performs race circuit identification and lap tracking based on in-game 3D positional coordinates.
package circuit

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/zetetos/gt-telemetry/v2/pkg/circuits"
	"github.com/zetetos/gt-telemetry/v2/pkg/models"
)

const (
	shortestCircuitLengthMetres int           = 900 // Minimum length in metres (Northern Isle Speedway)
	shortestLapTime             time.Duration = 10 * time.Second
	minConfidenceThreshold      float64       = 0.3 // Minimum confidence threshold (30%) before choosing a circuit
)

// Manager defines the interface for circuit management and lap tracking.
type Manager interface {
	Reset()
	ResetLapProgress()
	Name() string
	Variation() string
	Country() string
	CandidateCount() int
	LengthMetres() float64
	LapProgress() float64
	LapProgressRemaining() float64
	UpdateCircuit(
		odometerReading float64,
		lap int16,
		lapTime time.Duration,
		coordinate models.Coordinate,
		coordinateType models.CoordinateType,
	) (didUpdate bool)
}

// Circuit provides circuit identification and lap tracking facilities.
// Conforms with the circuit.Manager interface.
type Circuit struct {
	database                circuits.CircuitDB   // Circuit database for track identification
	log                     zerolog.Logger       // Logger instance
	info                    circuits.CircuitInfo // Current circuit information
	lap                     int16                // Current lap number being tracked
	lapStartOdometerReading float64              // Distance at which the current lap started
	lapProgressMetres       float64              // Lap distance tracking for unknown circuits
	bestLapTime             time.Duration        // Time taken to complete the last lap
	observedLength          int                  // Observed circuit length of the circuit
	lastCoordinate          models.Coordinate    // Last known coordinate for distance tracking
	candidates              Candidates           // Circuit candidates with confidence tracking
	seenCoordinates         map[string]uint16    // Tracks which coordinate boxes have been processed
}

// New creates a new Circuit instance with the provided logger and initializes the circuit database.
func New(db circuits.CircuitDB, logger zerolog.Logger) (*Circuit, error) {
	return &Circuit{
		database:                db,
		log:                     logger.With().Str("package", "circuit").Logger(),
		lapStartOdometerReading: 0,
		lapProgressMetres:       0,
		bestLapTime:             100 * time.Hour,
		info:                    emptyCircuitInfo(),
		lastCoordinate:          models.Coordinate{},
		candidates:              make(Candidates),
		seenCoordinates:         make(map[string]uint16),
	}, nil
}

// Reset clears the current circuit information and lap start marker distance.
func (c *Circuit) Reset() {
	c.info = emptyCircuitInfo()
	c.candidates = make(Candidates)
	c.seenCoordinates = make(map[string]uint16)
	c.ResetLapProgress()

	c.log.Info().
		Msg("Circuit reset")
}

// ResetLapProgress clears the lap progress tracking without resetting the circuit information.
func (c *Circuit) ResetLapProgress() {
	c.lapStartOdometerReading = 0
	c.lapProgressMetres = 0
	c.bestLapTime = 100 * time.Hour
	c.lastCoordinate = models.Coordinate{}

	c.log.Info().
		Msg("Lap progress reset")
}

// Name returns the name of the current circuit.
func (c *Circuit) Name() string {
	return c.info.Name
}

// Variation returns the variation of the current circuit.
func (c *Circuit) Variation() string {
	return c.info.Variation
}

// Country returns the country of the current circuit.
func (c *Circuit) Country() string {
	return c.info.Country
}

// CandidateCount returns the number of circuit candidates currently being tracked.
func (c *Circuit) CandidateCount() int {
	return len(c.candidates)
}

// LengthMetres returns the length of the current circuit in metres.
func (c *Circuit) LengthMetres() float64 {
	if c.observedLength > shortestCircuitLengthMetres {
		return float64(c.observedLength)
	}

	return float64(c.info.Length)
}

// LapProgress returns the progress through the current lap as a value between 0 and 1.
func (c *Circuit) LapProgress() float64 {
	progress := c.lapProgressMetres / float64(c.info.Length)

	return max(min(progress, 1), 0)
}

// LapProgressRemaining returns the remaining progress through the current lap as a value between 0 and 1.
func (c *Circuit) LapProgressRemaining() float64 {
	progress := c.LapProgress()

	return 1.0 - progress
}

// UpdateCircuit updates the current circuit information by matching the provided coordinate with a circuit DB entry.
// Returns true if the circuit was updated.
func (c *Circuit) UpdateCircuit(
	odometerReading float64,
	lap int16,
	lapTime time.Duration,
	coordinate models.Coordinate,
	coordinateType models.CoordinateType,
) (didUpdate bool) {
	c.updateDistanceTravelled(odometerReading, lap, coordinateType)

	if coordinateType == models.CoordinateTypeStartLine {
		c.setCircuitLength(lapTime)
		c.setLapStartMarker(odometerReading)
	}

	coordinateNorm := circuits.NormaliseStartLineCoordinate(coordinate)
	key := coordinateNorm.String()

	circuitID, seenCoordinate := c.database.GetCircuitAtCoordinate(coordinate, coordinateType)
	if !seenCoordinate {
		return false
	}

	// Only update confidence for new coordinate boxes to avoid log spam
	_, seenCoordinate = c.seenCoordinates[key]
	if !seenCoordinate {
		c.seenCoordinates[key]++
		c.updateCandidateConfidence(circuitID, key)
	}

	bestCandidate := c.bestCandidate()
	if bestCandidate == nil {
		return false
	}

	if bestCandidate.info.ID != c.info.ID {
		c.info = bestCandidate.info

		if coordinateType == models.CoordinateTypeStartLine {
			c.info.StartLine = coordinateNorm
		}

		c.log.Info().
			Str("track", c.info.Variation).
			Str("confidence", fmt.Sprintf("%.0f%%", bestCandidate.confidence*100)).
			Int("matches", len(bestCandidate.matchedCoords)).
			Int("length_metres", c.info.Length).
			Msg("Circuit updated")

		return true
	}

	if bestCandidate.confidence == 100 {
		c.log.Info().
			Str("track", c.info.Variation).
			Str("confidence", fmt.Sprintf("%.0f%%", bestCandidate.confidence*100)).
			Int("matches", len(bestCandidate.matchedCoords)).
			Int("length_metres", c.info.Length).
			Msg("Circuit match confidence maximized")
	}

	return false
}

// updateDistanceTravelled updates the total distance travelled based on the provided coordinate.
// TODO: this is duplicated in fuelrange.distanceTravelled.
func (c *Circuit) updateDistanceTravelled(odometerReading float64, lap int16, coordinateType models.CoordinateType) {
	// New lap started
	if coordinateType == models.CoordinateTypeStartLine {
		c.lapStartOdometerReading = odometerReading
		c.lapProgressMetres = 0
		c.lap = lap

		return
	}

	// Ignore updates until lap start marker is set
	if c.lapStartOdometerReading < 0 {
		return
	}

	// Wait for next lap start when odometer rolled back/reset or unexpected lap change
	if odometerReading < c.lapProgressMetres || lap != c.lap {
		c.lapStartOdometerReading = -1
		c.lapProgressMetres = 0
		c.lap = lap
	}

	c.lapProgressMetres = odometerReading - c.lapStartOdometerReading
}

// setLapStartMarker sets the distance at which the current lap started.
func (c *Circuit) setLapStartMarker(odomoterReading float64) {
	c.lapStartOdometerReading = odomoterReading

	if c.circuitIsKnown() {
		return
	}

	c.lapProgressMetres = 0

	c.log.Debug().
		Float64("odometer", odomoterReading).
		Msg("Lap start marker set")
}

// setCircuitLength sets the circuit length if it can be determined from the distance travelled.
func (c *Circuit) setCircuitLength(lapTime time.Duration) {
	if lapTime < shortestLapTime || lapTime >= c.bestLapTime {
		return
	}

	circuitLength := int(c.lapProgressMetres)

	if circuitLength <= shortestCircuitLengthMetres {
		return
	}

	c.observedLength = circuitLength
	c.bestLapTime = lapTime

	c.log.Info().
		Int("length_metres", circuitLength).
		Str("lap_time", lapTime.String()).
		Msg("Observed circuit length updated")
}

// emptyCircuitInfo returns a default empty CircuitInfo instance.
func emptyCircuitInfo() circuits.CircuitInfo {
	return circuits.CircuitInfo{
		ID:        "unknown",
		Name:      "unknown",
		Length:    0,
		StartLine: models.CoordinateNorm{X: 0, Y: 0, Z: 0},
	}
}

// circuitIsKnown returns true if the current circuit has been identified.
func (c *Circuit) circuitIsKnown() bool {
	return c.info.ID != emptyCircuitInfo().ID
}

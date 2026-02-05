// Package odometer provides functionality to track the distance travelled based on coordinate updates.
package odometer

import (
	"math"

	"github.com/rs/zerolog"
	"github.com/zetetos/gt-telemetry/v2/pkg/models"
)

const (
	teleportDistanceMax float64 = 500
)

// Odometer tracks the total distance travelled based on coordinate updates.
type Odometer struct {
	log            zerolog.Logger    // Logger instance
	distanceMetres float64           // Total distance in metres
	lastCoordinate models.Coordinate // Last known coordinate for distance tracking
}

// New creates a new Odometer instance.
func New(logger zerolog.Logger) *Odometer {
	odometer := Odometer{
		log:            logger.With().Str("package", "odometer").Logger(),
		distanceMetres: 0,
		lastCoordinate: models.Coordinate{},
	}

	return &odometer
}

// Reset clears the odometer distance and last known coordinate.
func (m *Odometer) Reset() {
	m.distanceMetres = 0
	m.lastCoordinate = models.Coordinate{}

	m.log.Debug().Msg("Odometer reset")
}

// Add updates the odometer with the current position and returns the distance travelled since the last update.
func (m *Odometer) Add(currentPos models.Coordinate) float64 {
	dx := float64(currentPos.X - m.lastCoordinate.X)
	dy := float64(currentPos.Y - m.lastCoordinate.Y)
	dz := float64(currentPos.Z - m.lastCoordinate.Z)
	distance := math.Sqrt(dx*dx + dy*dy + dz*dz)

	// drop unreasonable distance increments (teleports/glitches)
	if distance > teleportDistanceMax {
		m.log.Debug().
			Float64("distance", distance).
			Int("Min", 0).
			Int("Max", int(teleportDistanceMax)).
			Msg("Distance travelled out of acceptable range")

		m.lastCoordinate = currentPos

		return m.distanceMetres
	}

	m.distanceMetres += distance
	m.lastCoordinate = currentPos

	return m.distanceMetres
}

// Read returns the current odometer reading in metres.
func (m *Odometer) Read() float64 {
	if math.IsNaN(m.distanceMetres) {
		return 0
	}

	if math.IsInf(m.distanceMetres, 0) {
		return 999999.9
	}

	return m.distanceMetres
}

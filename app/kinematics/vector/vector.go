// Package vector provides utility functions for 3D vector operations.
package vector

import (
	"math"

	"github.com/zetetos/gt-telemetry/v2/pkg/models"
)

// Delta returns the difference between two vectors.
func Delta(vector1 models.Vector, vector2 models.Vector) models.Vector {
	return models.Vector{
		X: vector1.X - vector2.X,
		Y: vector1.Y - vector2.Y,
		Z: vector1.Z - vector2.Z,
	}
}

// Magnitude returns the magnitude of a vector.
func Magnitude(vector models.Vector) float64 {
	return math.Sqrt(float64(vector.X*vector.X + vector.Y*vector.Y + vector.Z*vector.Z))
}

// Scale scales a vector by the given factors.
func Scale(vector models.Vector, xScale float32, yScale float32, zScale float32) models.Vector {
	return models.Vector{
		X: vector.X * xScale,
		Y: vector.Y * yScale,
		Z: vector.Z * zScale,
	}
}

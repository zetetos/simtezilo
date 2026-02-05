// Package rotationalenvelope provides utilities for 6DOF rotational envelope operations.
package rotationalenvelope

import (
	"math"

	"github.com/zetetos/gt-telemetry/v2/pkg/models"
)

// Delta returns the difference between two rotational envelopes.
func Delta(axis1 models.RotationalEnvelope, axis2 models.RotationalEnvelope) models.RotationalEnvelope {
	return models.RotationalEnvelope{
		Pitch: axis1.Pitch - axis2.Pitch,
		Yaw:   axis1.Yaw - axis2.Yaw,
		Roll:  axis1.Roll - axis2.Roll,
	}
}

// Magnitude returns the magnitude of a rotational envelope.
func Magnitude(axis models.RotationalEnvelope) float64 {
	return math.Sqrt(float64(axis.Pitch*axis.Pitch + axis.Yaw*axis.Yaw + axis.Roll*axis.Roll))
}

// Scale scales a rotational envelope by the given factors for each axis.
func Scale(axis models.RotationalEnvelope, pitchScale float32, yawScale float32, rollScale float32) models.RotationalEnvelope {
	return models.RotationalEnvelope{
		Pitch: axis.Pitch * pitchScale,
		Yaw:   axis.Yaw * yawScale,
		Roll:  axis.Roll * rollScale,
	}
}

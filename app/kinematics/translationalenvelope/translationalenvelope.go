// Package translationalenvelope provides functions for 6DOF translational envelope operations.
package translationalenvelope

import (
	"math"

	"github.com/zetetos/gt-telemetry/v2/pkg/models"
)

// Delta computes the difference between two translational envelopes.
func Delta(
	envelope1 models.TranslationalEnvelope,
	envelope2 models.TranslationalEnvelope,
) models.TranslationalEnvelope {
	return models.TranslationalEnvelope{
		Sway:  envelope1.Sway - envelope2.Sway,
		Heave: envelope1.Heave - envelope2.Heave,
		Surge: envelope1.Surge - envelope2.Surge,
	}
}

// Magnitude computes the magnitude of a translational envelope vector.
func Magnitude(e models.TranslationalEnvelope) float64 {
	return math.Sqrt(float64(e.Sway*e.Sway + e.Heave*e.Heave + e.Surge*e.Surge))
}

// Scale adjusts the scale of each of the axis within a translational envelope.
func Scale(
	envelope models.TranslationalEnvelope,
	swayScale float32,
	heaveScale float32,
	surgeScale float32,
) models.TranslationalEnvelope {
	return models.TranslationalEnvelope{
		Sway:  envelope.Sway * swayScale,
		Heave: envelope.Heave * heaveScale,
		Surge: envelope.Surge * surgeScale,
	}
}

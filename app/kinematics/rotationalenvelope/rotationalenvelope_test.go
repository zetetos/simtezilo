package rotationalenvelope_test

import (
	"math"
	"testing"

	"github.com/zetetos/gt-telemetry/v2/pkg/models"
	"github.com/zetetos/simtezilo/app/kinematics/rotationalenvelope"
)

func TestDeltaReturnsDifferenceBetweenEnvelopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		envelope1 models.RotationalEnvelope
		envelope2 models.RotationalEnvelope
		expected  models.RotationalEnvelope
	}{
		{
			name:      "both zero envelopes",
			envelope1: models.RotationalEnvelope{Pitch: 0, Yaw: 0, Roll: 0},
			envelope2: models.RotationalEnvelope{Pitch: 0, Yaw: 0, Roll: 0},
			expected:  models.RotationalEnvelope{Pitch: 0, Yaw: 0, Roll: 0},
		},
		{
			name:      "positive envelopes",
			envelope1: models.RotationalEnvelope{Pitch: 5, Yaw: 10, Roll: 15},
			envelope2: models.RotationalEnvelope{Pitch: 2, Yaw: 3, Roll: 4},
			expected:  models.RotationalEnvelope{Pitch: 3, Yaw: 7, Roll: 11},
		},
		{
			name:      "negative envelopes",
			envelope1: models.RotationalEnvelope{Pitch: -5, Yaw: -10, Roll: -15},
			envelope2: models.RotationalEnvelope{Pitch: -2, Yaw: -3, Roll: -4},
			expected:  models.RotationalEnvelope{Pitch: -3, Yaw: -7, Roll: -11},
		},
		{
			name:      "mixed positive and negative",
			envelope1: models.RotationalEnvelope{Pitch: 5, Yaw: -10, Roll: 15},
			envelope2: models.RotationalEnvelope{Pitch: -2, Yaw: 3, Roll: -4},
			expected:  models.RotationalEnvelope{Pitch: 7, Yaw: -13, Roll: 19},
		},
		{
			name:      "first envelope zero",
			envelope1: models.RotationalEnvelope{Pitch: 0, Yaw: 0, Roll: 0},
			envelope2: models.RotationalEnvelope{Pitch: 5, Yaw: 10, Roll: 15},
			expected:  models.RotationalEnvelope{Pitch: -5, Yaw: -10, Roll: -15},
		},
		{
			name:      "second envelope zero",
			envelope1: models.RotationalEnvelope{Pitch: 5, Yaw: 10, Roll: 15},
			envelope2: models.RotationalEnvelope{Pitch: 0, Yaw: 0, Roll: 0},
			expected:  models.RotationalEnvelope{Pitch: 5, Yaw: 10, Roll: 15},
		},
		{
			name:      "identical envelopes",
			envelope1: models.RotationalEnvelope{Pitch: 7, Yaw: 8, Roll: 9},
			envelope2: models.RotationalEnvelope{Pitch: 7, Yaw: 8, Roll: 9},
			expected:  models.RotationalEnvelope{Pitch: 0, Yaw: 0, Roll: 0},
		},
		{
			name:      "floating point values",
			envelope1: models.RotationalEnvelope{Pitch: 1.5, Yaw: 2.5, Roll: 3.5},
			envelope2: models.RotationalEnvelope{Pitch: 0.5, Yaw: 1.5, Roll: 2.5},
			expected:  models.RotationalEnvelope{Pitch: 1, Yaw: 1, Roll: 1},
		},
		{
			name:      "large values",
			envelope1: models.RotationalEnvelope{Pitch: 1000000, Yaw: 2000000, Roll: 3000000},
			envelope2: models.RotationalEnvelope{Pitch: 500000, Yaw: 1000000, Roll: 1500000},
			expected:  models.RotationalEnvelope{Pitch: 500000, Yaw: 1000000, Roll: 1500000},
		},
		{
			name:      "very small values",
			envelope1: models.RotationalEnvelope{Pitch: 0.0001, Yaw: 0.0002, Roll: 0.0003},
			envelope2: models.RotationalEnvelope{Pitch: 0.00005, Yaw: 0.0001, Roll: 0.00015},
			expected:  models.RotationalEnvelope{Pitch: 0.00005, Yaw: 0.0001, Roll: 0.00015},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := rotationalenvelope.Delta(testCase.envelope1, testCase.envelope2)
			if !envelopesEqual(result, testCase.expected, 1e-6) {
				t.Errorf("Delta(%v, %v) = %v, expected %v",
					testCase.envelope1, testCase.envelope2, result, testCase.expected)
			}
		})
	}
}

func TestMagnitudeReturnsEnvelopeLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		envelope models.RotationalEnvelope
		expected float64
	}{
		{
			name:     "zero envelope",
			envelope: models.RotationalEnvelope{Pitch: 0, Yaw: 0, Roll: 0},
			expected: 0,
		},
		{
			name:     "unit pitch only",
			envelope: models.RotationalEnvelope{Pitch: 1, Yaw: 0, Roll: 0},
			expected: 1,
		},
		{
			name:     "unit yaw only",
			envelope: models.RotationalEnvelope{Pitch: 0, Yaw: 1, Roll: 0},
			expected: 1,
		},
		{
			name:     "unit roll only",
			envelope: models.RotationalEnvelope{Pitch: 0, Yaw: 0, Roll: 1},
			expected: 1,
		},
		{
			name:     "negative unit pitch",
			envelope: models.RotationalEnvelope{Pitch: -1, Yaw: 0, Roll: 0},
			expected: 1,
		},
		{
			name:     "3-4-5 triangle in pitch-yaw plane",
			envelope: models.RotationalEnvelope{Pitch: 3, Yaw: 4, Roll: 0},
			expected: 5,
		},
		{
			name:     "3-4-5 triangle in pitch-roll plane",
			envelope: models.RotationalEnvelope{Pitch: 3, Yaw: 0, Roll: 4},
			expected: 5,
		},
		{
			name:     "3-4-5 triangle in yaw-roll plane",
			envelope: models.RotationalEnvelope{Pitch: 0, Yaw: 3, Roll: 4},
			expected: 5,
		},
		{
			name:     "positive envelope all components",
			envelope: models.RotationalEnvelope{Pitch: 1, Yaw: 2, Roll: 2},
			expected: 3,
		},
		{
			name:     "negative envelope all components",
			envelope: models.RotationalEnvelope{Pitch: -1, Yaw: -2, Roll: -2},
			expected: 3,
		},
		{
			name:     "mixed positive and negative",
			envelope: models.RotationalEnvelope{Pitch: 2, Yaw: -3, Roll: 6},
			expected: 7,
		},
		{
			name:     "floating point values",
			envelope: models.RotationalEnvelope{Pitch: 1.5, Yaw: 2.0, Roll: 2.5},
			expected: math.Sqrt(1.5*1.5 + 2.0*2.0 + 2.5*2.5),
		},
		{
			name:     "large values",
			envelope: models.RotationalEnvelope{Pitch: 1000, Yaw: 2000, Roll: 2000},
			expected: 3000,
		},
		{
			name:     "very small values",
			envelope: models.RotationalEnvelope{Pitch: 0.001, Yaw: 0.002, Roll: 0.002},
			expected: 0.003,
		},
		{
			name:     "pythagorean quadruple 1-2-2-3",
			envelope: models.RotationalEnvelope{Pitch: 2, Yaw: 2, Roll: 1},
			expected: 3,
		},
		{
			name:     "pythagorean quadruple 2-3-6-7",
			envelope: models.RotationalEnvelope{Pitch: 2, Yaw: 3, Roll: 6},
			expected: 7,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := rotationalenvelope.Magnitude(testCase.envelope)
			if !floatsEqual(result, testCase.expected, 1e-6) {
				t.Errorf("Magnitude(%v) = %v, expected %v",
					testCase.envelope, result, testCase.expected)
			}
		})
	}
}

func TestScaleMultipliesEnvelopeByFactors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		envelope   models.RotationalEnvelope
		pitchScale float32
		yawScale   float32
		rollScale  float32
		expected   models.RotationalEnvelope
	}{
		{
			name:       "zero envelope with any scale",
			envelope:   models.RotationalEnvelope{Pitch: 0, Yaw: 0, Roll: 0},
			pitchScale: 5,
			yawScale:   10,
			rollScale:  15,
			expected:   models.RotationalEnvelope{Pitch: 0, Yaw: 0, Roll: 0},
		},
		{
			name:       "any envelope with zero scales",
			envelope:   models.RotationalEnvelope{Pitch: 5, Yaw: 10, Roll: 15},
			pitchScale: 0,
			yawScale:   0,
			rollScale:  0,
			expected:   models.RotationalEnvelope{Pitch: 0, Yaw: 0, Roll: 0},
		},
		{
			name:       "unit scales identity",
			envelope:   models.RotationalEnvelope{Pitch: 5, Yaw: 10, Roll: 15},
			pitchScale: 1,
			yawScale:   1,
			rollScale:  1,
			expected:   models.RotationalEnvelope{Pitch: 5, Yaw: 10, Roll: 15},
		},
		{
			name:       "uniform positive scale",
			envelope:   models.RotationalEnvelope{Pitch: 2, Yaw: 3, Roll: 4},
			pitchScale: 2,
			yawScale:   2,
			rollScale:  2,
			expected:   models.RotationalEnvelope{Pitch: 4, Yaw: 6, Roll: 8},
		},
		{
			name:       "different positive scales",
			envelope:   models.RotationalEnvelope{Pitch: 2, Yaw: 3, Roll: 4},
			pitchScale: 2,
			yawScale:   3,
			rollScale:  4,
			expected:   models.RotationalEnvelope{Pitch: 4, Yaw: 9, Roll: 16},
		},
		{
			name:       "negative scales flip direction",
			envelope:   models.RotationalEnvelope{Pitch: 2, Yaw: 3, Roll: 4},
			pitchScale: -1,
			yawScale:   -1,
			rollScale:  -1,
			expected:   models.RotationalEnvelope{Pitch: -2, Yaw: -3, Roll: -4},
		},
		{
			name:       "mixed positive and negative scales",
			envelope:   models.RotationalEnvelope{Pitch: 2, Yaw: 3, Roll: 4},
			pitchScale: 2,
			yawScale:   -3,
			rollScale:  0.5,
			expected:   models.RotationalEnvelope{Pitch: 4, Yaw: -9, Roll: 2},
		},
		{
			name:       "negative envelope with positive scales",
			envelope:   models.RotationalEnvelope{Pitch: -2, Yaw: -3, Roll: -4},
			pitchScale: 2,
			yawScale:   3,
			rollScale:  4,
			expected:   models.RotationalEnvelope{Pitch: -4, Yaw: -9, Roll: -16},
		},
		{
			name:       "negative envelope with negative scales",
			envelope:   models.RotationalEnvelope{Pitch: -2, Yaw: -3, Roll: -4},
			pitchScale: -2,
			yawScale:   -3,
			rollScale:  -4,
			expected:   models.RotationalEnvelope{Pitch: 4, Yaw: 9, Roll: 16},
		},
		{
			name:       "fractional scales",
			envelope:   models.RotationalEnvelope{Pitch: 10, Yaw: 20, Roll: 30},
			pitchScale: 0.5,
			yawScale:   0.25,
			rollScale:  0.1,
			expected:   models.RotationalEnvelope{Pitch: 5, Yaw: 5, Roll: 3},
		},
		{
			name:       "large scales",
			envelope:   models.RotationalEnvelope{Pitch: 1, Yaw: 2, Roll: 3},
			pitchScale: 1000,
			yawScale:   1000,
			rollScale:  1000,
			expected:   models.RotationalEnvelope{Pitch: 1000, Yaw: 2000, Roll: 3000},
		},
		{
			name:       "very small scales",
			envelope:   models.RotationalEnvelope{Pitch: 1000, Yaw: 2000, Roll: 3000},
			pitchScale: 0.001,
			yawScale:   0.001,
			rollScale:  0.001,
			expected:   models.RotationalEnvelope{Pitch: 1, Yaw: 2, Roll: 3},
		},
		{
			name:       "scale only pitch",
			envelope:   models.RotationalEnvelope{Pitch: 5, Yaw: 10, Roll: 15},
			pitchScale: 3,
			yawScale:   1,
			rollScale:  1,
			expected:   models.RotationalEnvelope{Pitch: 15, Yaw: 10, Roll: 15},
		},
		{
			name:       "scale only yaw",
			envelope:   models.RotationalEnvelope{Pitch: 5, Yaw: 10, Roll: 15},
			pitchScale: 1,
			yawScale:   3,
			rollScale:  1,
			expected:   models.RotationalEnvelope{Pitch: 5, Yaw: 30, Roll: 15},
		},
		{
			name:       "scale only roll",
			envelope:   models.RotationalEnvelope{Pitch: 5, Yaw: 10, Roll: 15},
			pitchScale: 1,
			yawScale:   1,
			rollScale:  3,
			expected:   models.RotationalEnvelope{Pitch: 5, Yaw: 10, Roll: 45},
		},
		{
			name:       "floating point envelope with floating point scales",
			envelope:   models.RotationalEnvelope{Pitch: 1.5, Yaw: 2.5, Roll: 3.5},
			pitchScale: 1.5,
			yawScale:   2.5,
			rollScale:  3.5,
			expected:   models.RotationalEnvelope{Pitch: 2.25, Yaw: 6.25, Roll: 12.25},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := rotationalenvelope.Scale(
				testCase.envelope,
				testCase.pitchScale,
				testCase.yawScale,
				testCase.rollScale,
			)
			if !envelopesEqual(result, testCase.expected, 1e-6) {
				t.Errorf("Scale(%v, %v, %v, %v) = %v, expected %v",
					testCase.envelope,
					testCase.pitchScale,
					testCase.yawScale,
					testCase.rollScale,
					result,
					testCase.expected,
				)
			}
		})
	}
}

// envelopesEqual compares two rotational envelopes for equality within a tolerance.
func envelopesEqual(e1, e2 models.RotationalEnvelope, tolerance float64) bool {
	return floatsEqual(float64(e1.Pitch), float64(e2.Pitch), tolerance) &&
		floatsEqual(float64(e1.Yaw), float64(e2.Yaw), tolerance) &&
		floatsEqual(float64(e1.Roll), float64(e2.Roll), tolerance)
}

// floatsEqual compares two floats for equality within a tolerance.
func floatsEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

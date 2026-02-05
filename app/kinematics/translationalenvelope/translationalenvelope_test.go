package translationalenvelope_test

import (
	"math"
	"testing"

	"github.com/zetetos/gt-telemetry/v2/pkg/models"
	"github.com/zetetos/simtezilo/app/kinematics/translationalenvelope"
)

func TestDeltaReturnsDifferenceBetweenEnvelopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		envelope1 models.TranslationalEnvelope
		envelope2 models.TranslationalEnvelope
		expected  models.TranslationalEnvelope
	}{
		{
			name:      "both zero envelopes",
			envelope1: models.TranslationalEnvelope{Sway: 0, Heave: 0, Surge: 0},
			envelope2: models.TranslationalEnvelope{Sway: 0, Heave: 0, Surge: 0},
			expected:  models.TranslationalEnvelope{Sway: 0, Heave: 0, Surge: 0},
		},
		{
			name:      "positive envelopes",
			envelope1: models.TranslationalEnvelope{Sway: 5, Heave: 10, Surge: 15},
			envelope2: models.TranslationalEnvelope{Sway: 2, Heave: 3, Surge: 4},
			expected:  models.TranslationalEnvelope{Sway: 3, Heave: 7, Surge: 11},
		},
		{
			name:      "negative envelopes",
			envelope1: models.TranslationalEnvelope{Sway: -5, Heave: -10, Surge: -15},
			envelope2: models.TranslationalEnvelope{Sway: -2, Heave: -3, Surge: -4},
			expected:  models.TranslationalEnvelope{Sway: -3, Heave: -7, Surge: -11},
		},
		{
			name:      "mixed positive and negative",
			envelope1: models.TranslationalEnvelope{Sway: 5, Heave: -10, Surge: 15},
			envelope2: models.TranslationalEnvelope{Sway: -2, Heave: 3, Surge: -4},
			expected:  models.TranslationalEnvelope{Sway: 7, Heave: -13, Surge: 19},
		},
		{
			name:      "first envelope zero",
			envelope1: models.TranslationalEnvelope{Sway: 0, Heave: 0, Surge: 0},
			envelope2: models.TranslationalEnvelope{Sway: 5, Heave: 10, Surge: 15},
			expected:  models.TranslationalEnvelope{Sway: -5, Heave: -10, Surge: -15},
		},
		{
			name:      "second envelope zero",
			envelope1: models.TranslationalEnvelope{Sway: 5, Heave: 10, Surge: 15},
			envelope2: models.TranslationalEnvelope{Sway: 0, Heave: 0, Surge: 0},
			expected:  models.TranslationalEnvelope{Sway: 5, Heave: 10, Surge: 15},
		},
		{
			name:      "identical envelopes",
			envelope1: models.TranslationalEnvelope{Sway: 7, Heave: 8, Surge: 9},
			envelope2: models.TranslationalEnvelope{Sway: 7, Heave: 8, Surge: 9},
			expected:  models.TranslationalEnvelope{Sway: 0, Heave: 0, Surge: 0},
		},
		{
			name:      "floating point values",
			envelope1: models.TranslationalEnvelope{Sway: 1.5, Heave: 2.5, Surge: 3.5},
			envelope2: models.TranslationalEnvelope{Sway: 0.5, Heave: 1.5, Surge: 2.5},
			expected:  models.TranslationalEnvelope{Sway: 1, Heave: 1, Surge: 1},
		},
		{
			name:      "large values",
			envelope1: models.TranslationalEnvelope{Sway: 1000000, Heave: 2000000, Surge: 3000000},
			envelope2: models.TranslationalEnvelope{Sway: 500000, Heave: 1000000, Surge: 1500000},
			expected:  models.TranslationalEnvelope{Sway: 500000, Heave: 1000000, Surge: 1500000},
		},
		{
			name:      "very small values",
			envelope1: models.TranslationalEnvelope{Sway: 0.0001, Heave: 0.0002, Surge: 0.0003},
			envelope2: models.TranslationalEnvelope{Sway: 0.00005, Heave: 0.0001, Surge: 0.00015},
			expected:  models.TranslationalEnvelope{Sway: 0.00005, Heave: 0.0001, Surge: 0.00015},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := translationalenvelope.Delta(testCase.envelope1, testCase.envelope2)
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
		envelope models.TranslationalEnvelope
		expected float64
	}{
		{
			name:     "zero envelope",
			envelope: models.TranslationalEnvelope{Sway: 0, Heave: 0, Surge: 0},
			expected: 0,
		},
		{
			name:     "unit sway only",
			envelope: models.TranslationalEnvelope{Sway: 1, Heave: 0, Surge: 0},
			expected: 1,
		},
		{
			name:     "unit heave only",
			envelope: models.TranslationalEnvelope{Sway: 0, Heave: 1, Surge: 0},
			expected: 1,
		},
		{
			name:     "unit surge only",
			envelope: models.TranslationalEnvelope{Sway: 0, Heave: 0, Surge: 1},
			expected: 1,
		},
		{
			name:     "negative unit sway",
			envelope: models.TranslationalEnvelope{Sway: -1, Heave: 0, Surge: 0},
			expected: 1,
		},
		{
			name:     "3-4-5 triangle in sway-heave plane",
			envelope: models.TranslationalEnvelope{Sway: 3, Heave: 4, Surge: 0},
			expected: 5,
		},
		{
			name:     "3-4-5 triangle in sway-surge plane",
			envelope: models.TranslationalEnvelope{Sway: 3, Heave: 0, Surge: 4},
			expected: 5,
		},
		{
			name:     "3-4-5 triangle in heave-surge plane",
			envelope: models.TranslationalEnvelope{Sway: 0, Heave: 3, Surge: 4},
			expected: 5,
		},
		{
			name:     "positive envelope all components",
			envelope: models.TranslationalEnvelope{Sway: 1, Heave: 2, Surge: 2},
			expected: 3,
		},
		{
			name:     "negative envelope all components",
			envelope: models.TranslationalEnvelope{Sway: -1, Heave: -2, Surge: -2},
			expected: 3,
		},
		{
			name:     "mixed positive and negative",
			envelope: models.TranslationalEnvelope{Sway: 2, Heave: -3, Surge: 6},
			expected: 7,
		},
		{
			name:     "floating point values",
			envelope: models.TranslationalEnvelope{Sway: 1.5, Heave: 2.0, Surge: 2.5},
			expected: math.Sqrt(1.5*1.5 + 2.0*2.0 + 2.5*2.5),
		},
		{
			name:     "large values",
			envelope: models.TranslationalEnvelope{Sway: 1000, Heave: 2000, Surge: 2000},
			expected: 3000,
		},
		{
			name:     "very small values",
			envelope: models.TranslationalEnvelope{Sway: 0.001, Heave: 0.002, Surge: 0.002},
			expected: 0.003,
		},
		{
			name:     "pythagorean quadruple 1-2-2-3",
			envelope: models.TranslationalEnvelope{Sway: 2, Heave: 2, Surge: 1},
			expected: 3,
		},
		{
			name:     "pythagorean quadruple 2-3-6-7",
			envelope: models.TranslationalEnvelope{Sway: 2, Heave: 3, Surge: 6},
			expected: 7,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := translationalenvelope.Magnitude(testCase.envelope)
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
		envelope   models.TranslationalEnvelope
		swayScale  float32
		heaveScale float32
		surgeScale float32
		expected   models.TranslationalEnvelope
	}{
		{
			name:       "zero envelope with any scale",
			envelope:   models.TranslationalEnvelope{Sway: 0, Heave: 0, Surge: 0},
			swayScale:  5,
			heaveScale: 10,
			surgeScale: 15,
			expected:   models.TranslationalEnvelope{Sway: 0, Heave: 0, Surge: 0},
		},
		{
			name:       "any envelope with zero scales",
			envelope:   models.TranslationalEnvelope{Sway: 5, Heave: 10, Surge: 15},
			swayScale:  0,
			heaveScale: 0,
			surgeScale: 0,
			expected:   models.TranslationalEnvelope{Sway: 0, Heave: 0, Surge: 0},
		},
		{
			name:       "unit scales identity",
			envelope:   models.TranslationalEnvelope{Sway: 5, Heave: 10, Surge: 15},
			swayScale:  1,
			heaveScale: 1,
			surgeScale: 1,
			expected:   models.TranslationalEnvelope{Sway: 5, Heave: 10, Surge: 15},
		},
		{
			name:       "uniform positive scale",
			envelope:   models.TranslationalEnvelope{Sway: 2, Heave: 3, Surge: 4},
			swayScale:  2,
			heaveScale: 2,
			surgeScale: 2,
			expected:   models.TranslationalEnvelope{Sway: 4, Heave: 6, Surge: 8},
		},
		{
			name:       "different positive scales",
			envelope:   models.TranslationalEnvelope{Sway: 2, Heave: 3, Surge: 4},
			swayScale:  2,
			heaveScale: 3,
			surgeScale: 4,
			expected:   models.TranslationalEnvelope{Sway: 4, Heave: 9, Surge: 16},
		},
		{
			name:       "negative scales flip direction",
			envelope:   models.TranslationalEnvelope{Sway: 2, Heave: 3, Surge: 4},
			swayScale:  -1,
			heaveScale: -1,
			surgeScale: -1,
			expected:   models.TranslationalEnvelope{Sway: -2, Heave: -3, Surge: -4},
		},
		{
			name:       "mixed positive and negative scales",
			envelope:   models.TranslationalEnvelope{Sway: 2, Heave: 3, Surge: 4},
			swayScale:  2,
			heaveScale: -3,
			surgeScale: 0.5,
			expected:   models.TranslationalEnvelope{Sway: 4, Heave: -9, Surge: 2},
		},
		{
			name:       "negative envelope with positive scales",
			envelope:   models.TranslationalEnvelope{Sway: -2, Heave: -3, Surge: -4},
			swayScale:  2,
			heaveScale: 3,
			surgeScale: 4,
			expected:   models.TranslationalEnvelope{Sway: -4, Heave: -9, Surge: -16},
		},
		{
			name:       "negative envelope with negative scales",
			envelope:   models.TranslationalEnvelope{Sway: -2, Heave: -3, Surge: -4},
			swayScale:  -2,
			heaveScale: -3,
			surgeScale: -4,
			expected:   models.TranslationalEnvelope{Sway: 4, Heave: 9, Surge: 16},
		},
		{
			name:       "fractional scales",
			envelope:   models.TranslationalEnvelope{Sway: 10, Heave: 20, Surge: 30},
			swayScale:  0.5,
			heaveScale: 0.25,
			surgeScale: 0.1,
			expected:   models.TranslationalEnvelope{Sway: 5, Heave: 5, Surge: 3},
		},
		{
			name:       "large scales",
			envelope:   models.TranslationalEnvelope{Sway: 1, Heave: 2, Surge: 3},
			swayScale:  1000,
			heaveScale: 1000,
			surgeScale: 1000,
			expected:   models.TranslationalEnvelope{Sway: 1000, Heave: 2000, Surge: 3000},
		},
		{
			name:       "very small scales",
			envelope:   models.TranslationalEnvelope{Sway: 1000, Heave: 2000, Surge: 3000},
			swayScale:  0.001,
			heaveScale: 0.001,
			surgeScale: 0.001,
			expected:   models.TranslationalEnvelope{Sway: 1, Heave: 2, Surge: 3},
		},
		{
			name:       "scale only sway",
			envelope:   models.TranslationalEnvelope{Sway: 5, Heave: 10, Surge: 15},
			swayScale:  3,
			heaveScale: 1,
			surgeScale: 1,
			expected:   models.TranslationalEnvelope{Sway: 15, Heave: 10, Surge: 15},
		},
		{
			name:       "scale only heave",
			envelope:   models.TranslationalEnvelope{Sway: 5, Heave: 10, Surge: 15},
			swayScale:  1,
			heaveScale: 3,
			surgeScale: 1,
			expected:   models.TranslationalEnvelope{Sway: 5, Heave: 30, Surge: 15},
		},
		{
			name:       "scale only surge",
			envelope:   models.TranslationalEnvelope{Sway: 5, Heave: 10, Surge: 15},
			swayScale:  1,
			heaveScale: 1,
			surgeScale: 3,
			expected:   models.TranslationalEnvelope{Sway: 5, Heave: 10, Surge: 45},
		},
		{
			name:       "floating point envelope with floating point scales",
			envelope:   models.TranslationalEnvelope{Sway: 1.5, Heave: 2.5, Surge: 3.5},
			swayScale:  1.5,
			heaveScale: 2.5,
			surgeScale: 3.5,
			expected:   models.TranslationalEnvelope{Sway: 2.25, Heave: 6.25, Surge: 12.25},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := translationalenvelope.Scale(
				testCase.envelope,
				testCase.swayScale,
				testCase.heaveScale,
				testCase.surgeScale,
			)
			if !envelopesEqual(result, testCase.expected, 1e-6) {
				t.Errorf("Scale(%v, %v, %v, %v) = %v, expected %v",
					testCase.envelope,
					testCase.swayScale,
					testCase.heaveScale,
					testCase.surgeScale,
					result,
					testCase.expected,
				)
			}
		})
	}
}

// envelopesEqual compares two translational envelopes for equality within a tolerance.
func envelopesEqual(e1, e2 models.TranslationalEnvelope, tolerance float64) bool {
	return floatsEqual(float64(e1.Sway), float64(e2.Sway), tolerance) &&
		floatsEqual(float64(e1.Heave), float64(e2.Heave), tolerance) &&
		floatsEqual(float64(e1.Surge), float64(e2.Surge), tolerance)
}

// floatsEqual compares two floats for equality within a tolerance.
func floatsEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

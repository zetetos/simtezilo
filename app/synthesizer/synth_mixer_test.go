package synthesizer_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test constants for EQ curve parameters.
const (
	testMinFreq    = 10.0
	testMaxFreq    = 70.0
	testResolution = 0.5
	testNumBuckets = int((testMaxFreq-testMinFreq)/testResolution) + 1
	testTolerance  = 0.001
	testNotchDB    = -12.0
)

// TestCalibrationEQAmplitudeCalculation tests the EQ amplitude calculation logic
// used during calibration mode.
func TestCalibrationEQAmplitudeCalculation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		frequency    float64
		curve        []float64
		expectedAmp  float64
		curveEnabled bool
	}{
		{
			name:         "flat curve at 20Hz returns unity",
			frequency:    20.0,
			curve:        newFlatCurve(),
			expectedAmp:  1.0,
			curveEnabled: true,
		},
		{
			name:         "notch at 20Hz returns -12dB amplitude",
			frequency:    20.0,
			curve:        newNotchCurve(20.0),
			expectedAmp:  dbToLinear(testNotchDB),
			curveEnabled: true,
		},
		{
			name:         "notch at 20Hz but querying 30Hz returns unity",
			frequency:    30.0,
			curve:        newNotchCurve(20.0),
			expectedAmp:  1.0,
			curveEnabled: true,
		},
		{
			name:         "notch at 30Hz returns -12dB at 30Hz",
			frequency:    30.0,
			curve:        newNotchCurve(30.0),
			expectedAmp:  dbToLinear(testNotchDB),
			curveEnabled: true,
		},
		{
			name:         "frequency below range returns unity",
			frequency:    5.0,
			curve:        newNotchCurve(20.0),
			expectedAmp:  1.0,
			curveEnabled: true,
		},
		{
			name:         "frequency above range returns unity",
			frequency:    100.0,
			curve:        newNotchCurve(20.0),
			expectedAmp:  1.0,
			curveEnabled: true,
		},
		{
			name:         "empty curve returns unity",
			frequency:    20.0,
			curve:        nil,
			expectedAmp:  1.0,
			curveEnabled: true,
		},
		{
			name:         "EQ disabled returns unity",
			frequency:    20.0,
			curve:        newNotchCurve(20.0),
			expectedAmp:  1.0,
			curveEnabled: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			frequency := testCase.frequency
			curve := testCase.curve
			enabled := testCase.curveEnabled

			// Act
			amp := calculateEQAmplitude(frequency, curve, enabled)

			// Assert
			assert.InDelta(t, testCase.expectedAmp, amp, testTolerance)
		})
	}
}

// TestCalibrationPerChannelEQ verifies that different channels can have different EQ curves.
func TestCalibrationPerChannelEQ(t *testing.T) {
	t.Parallel()

	t.Run("channel 0 attenuated at 20Hz while channel 1 is full", func(t *testing.T) {
		t.Parallel()

		// Arrange
		ch0Curve := newNotchCurve(20.0)
		ch1Curve := newNotchCurve(30.0)
		freq := 20.0

		// Act
		ch0Amp := calculateEQAmplitude(freq, ch0Curve, true)
		ch1Amp := calculateEQAmplitude(freq, ch1Curve, true)

		// Assert
		assert.InDelta(t, dbToLinear(testNotchDB), ch0Amp, testTolerance, "Channel 0 at 20Hz")
		assert.InDelta(t, 1.0, ch1Amp, testTolerance, "Channel 1 at 20Hz")
	})

	t.Run("channel 1 attenuated at 30Hz while channel 0 is full", func(t *testing.T) {
		t.Parallel()

		// Arrange
		ch0Curve := newNotchCurve(20.0)
		ch1Curve := newNotchCurve(30.0)
		freq := 30.0

		// Act
		ch0Amp := calculateEQAmplitude(freq, ch0Curve, true)
		ch1Amp := calculateEQAmplitude(freq, ch1Curve, true)

		// Assert
		assert.InDelta(t, 1.0, ch0Amp, testTolerance, "Channel 0 at 30Hz")
		assert.InDelta(t, dbToLinear(testNotchDB), ch1Amp, testTolerance, "Channel 1 at 30Hz")
	})
}

// newFlatCurve creates a flat EQ curve (all 1.0 = 0dB).
func newFlatCurve() []float64 {
	curve := make([]float64, testNumBuckets)
	for i := range curve {
		curve[i] = 1.0
	}

	return curve
}

// newNotchCurve creates a flat curve with a notch filter at the specified frequency.
func newNotchCurve(notchFreqHz float64) []float64 {
	curve := newFlatCurve()
	index := int((notchFreqHz - testMinFreq) / testResolution)

	if index >= 0 && index < len(curve) {
		curve[index] = dbToLinear(testNotchDB)
	}

	return curve
}

// dbToLinear converts decibels to linear amplitude.
//
//nolint:unparam // test helper function designed for general use
func dbToLinear(db float64) float64 {
	return math.Pow(10, db/20.0)
}

// calculateEQAmplitude is a pure function that calculates the EQ amplitude multiplier
// for a given frequency. This mirrors the logic in MixToMaster's calibration path.
func calculateEQAmplitude(frequency float64, curve []float64, enabled bool) float64 {
	if !enabled {
		return 1.0
	}

	if len(curve) == 0 {
		return 1.0
	}

	index := int((frequency - testMinFreq) / testResolution)
	if index < 0 || index >= len(curve) {
		return 1.0
	}

	return curve[index]
}

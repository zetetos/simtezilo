package synthesizer_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zetetos/simtezilo/app/calibrator"
	"github.com/zetetos/simtezilo/app/config"
	"github.com/zetetos/simtezilo/app/synthesizer"
)

func TestCalibrationModeOutputsSignal(t *testing.T) {
	t.Parallel()

	// Arrange
	mockCalibrator := calibrator.NewMockCalibrator(
		true,
		50, // 50 Hz test tone
		0,  // 0 dB (max)
		calibrator.OutputChannelBoth,
	)

	mockMixer := synthesizer.NewMockMixer(8000, mockCalibrator)

	synth, _ := synthesizer.New(&synthesizer.SynthOpts{
		Calibrator: mockCalibrator,
		Mixer:      mockMixer,
		Config:     &config.Synthesizer{InternalSampleRateHz: 8000},
	})

	streamer := synthesizer.NewStreamer(synth)
	samples := make([][2]float64, 100)

	// Act
	n, ok := streamer.Stream(samples)

	// Assert
	assert.True(t, ok, "Stream() should succeed")
	assert.Equal(t, len(samples), n, "Stream() should return expected number of samples")

	anyNonZero := false

	for _, s := range samples {
		if math.Abs(s[0]) > 1e-6 || math.Abs(s[1]) > 1e-6 {
			anyNonZero = true

			break
		}
	}

	assert.True(t, anyNonZero, "Calibration mode should output non-zero signal")
}

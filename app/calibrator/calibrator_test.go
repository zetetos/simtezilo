package calibrator_test

import (
	"testing"
	"time"

	"github.com/zetetos/simtezilo/app/calibrator"
)

func TestCalibratorDisable_StopsSweep(t *testing.T) {
	t.Parallel()

	cal, err := calibrator.NewToneGenerator(nil)
	if err != nil {
		t.Skip("Skipping test: config required for calibrator")
	}

	// Start a sweep
	cal.StartSweep()

	// Verify sweep is active
	if !cal.IsSweeping() {
		t.Error("Expected sweep to be active after StartSweep")
	}

	// Disable calibration
	cal.SetEnabled(false)

	// Give a moment for the sweep goroutine to be cancelled
	time.Sleep(10 * time.Millisecond)

	// Verify sweep is stopped
	if cal.IsSweeping() {
		t.Error("Expected sweep to stop when calibration is disabled")
	}

	// Verify frequency reset to minimum
	freq := cal.GetSweepFrequency()

	expectedMin := cal.GetSweepMin()
	if freq != expectedMin {
		t.Errorf("Expected sweep frequency to reset to minimum (%f), got %f", expectedMin, freq)
	}

	// With zero-crossing detection, calibration remains enabled but in stopping state
	// until mixer reaches zero crossing
	if !cal.IsStopping() {
		t.Error("Expected calibration to be in stopping state")
	}

	// Simulate mixer reaching zero crossing
	cal.ConfirmStopped()

	// Now calibration should be fully disabled
	if cal.IsEnabled() {
		t.Error("Expected calibration to be disabled after ConfirmStopped")
	}

	if cal.IsStopping() {
		t.Error("Expected stopping flag to be cleared after ConfirmStopped")
	}
}

func TestCalibratorDisable_ResetsToMinFrequency(t *testing.T) {
	t.Parallel()

	cal, err := calibrator.NewToneGenerator(nil)
	if err != nil {
		t.Skip("Skipping test: config required for calibrator")
	}

	// Enable calibration and start sweep
	cal.SetEnabled(true)
	cal.StartSweep()

	// Wait for sweep to progress
	time.Sleep(100 * time.Millisecond)

	// Get current frequency (should have progressed beyond min)
	currentFreq := cal.GetSweepFrequency()
	minFreq := cal.GetSweepMin()

	if currentFreq <= minFreq {
		t.Skip("Sweep didn't progress enough to test reset behavior")
	}

	// Disable calibration
	cal.SetEnabled(false)

	// Verify frequency is back to minimum
	freq := cal.GetSweepFrequency()
	if freq != minFreq {
		t.Errorf("Expected frequency to reset to minimum (%f), got %f", minFreq, freq)
	}
}

func TestCalibratorEnable_DoesNotAffectSweep(t *testing.T) {
	t.Parallel()

	cal, err := calibrator.NewToneGenerator(nil)
	if err != nil {
		t.Skip("Skipping test: config required for calibrator")
	}

	// Enable calibration (but don't start sweep)
	cal.SetEnabled(true)

	// Verify sweep is not active
	if cal.IsSweeping() {
		t.Error("Expected sweep to not be active after just enabling calibration")
	}

	// Disable and re-enable
	cal.SetEnabled(false)
	cal.SetEnabled(true)

	// Verify sweep is still not active
	if cal.IsSweeping() {
		t.Error("Expected sweep to remain inactive after enable/disable cycle")
	}
}

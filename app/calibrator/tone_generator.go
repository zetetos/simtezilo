package calibrator

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zetetos/simtezilo/app/config"
)

// OutputChannel represents which audio channels should receive the calibration tone.
type OutputChannel string

const (
	// OutputChannelBoth sends calibration tone to both left and right channels.
	OutputChannelBoth OutputChannel = "both"

	// OutputChannelLeft sends calibration tone to left channel only.
	OutputChannelLeft OutputChannel = "left"

	// OutputChannelRight sends calibration tone to right channel only.
	OutputChannelRight OutputChannel = "right"
)

// SweepRangeMode represents the frequency range used for sweeps.
type SweepRangeMode string

const (
	// SweepRangeHaptic uses a haptic-optimized frequency range (5-80Hz).
	SweepRangeHaptic SweepRangeMode = "haptic"

	// SweepRangeFull uses the full frequency range (5-160Hz).
	SweepRangeFull SweepRangeMode = "full"
)

// ToneGenerator manages tone generation state for calibration mode.
type ToneGenerator struct {
	isEnabled          bool
	isStopping         bool // Signal to continue generating until zero crossing
	frequency          float64
	gain               float64
	channel            OutputChannel
	frequencyIncrement float64        // Frequency adjustment increment in Hz
	config             *config.Config // Reference to config for accessing haptic range and gain increment
	mu                 sync.RWMutex
	isSweeping         bool
	sweepCancel        context.CancelFunc
	sweepFrequency     float64        // Current frequency during sweep
	sweepMin           float64        // Minimum sweep frequency
	sweepMax           float64        // Maximum sweep frequency
	sweepDuration      float64        // Sweep duration in seconds
	sweepRangeMode     SweepRangeMode // Current sweep range mode
}

// NewToneGenerator creates a new Calibrator instance with default values.
// Returns an error if cfg is nil.
func NewToneGenerator(cfg *config.Config) (*ToneGenerator, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	newCalibrator := &ToneGenerator{
		isEnabled:          false,
		frequency:          5,
		gain:               -30,
		channel:            OutputChannelBoth,
		frequencyIncrement: 1.0, // Default 1Hz increment
		config:             cfg,
		sweepDuration:      10,
		sweepRangeMode:     SweepRangeHaptic,
		sweepMin:           cfg.GetHapticsPulseMinHz(),
		sweepMax:           cfg.GetHapticsPulseMaxHz(),
	}

	return newCalibrator, nil
}

// IsEnabled returns whether calibration mode is enabled.
func (c *ToneGenerator) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.isEnabled
}

// IsStopping returns whether calibration is waiting to stop at zero crossing.
func (c *ToneGenerator) IsStopping() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.isStopping
}

// ConfirmStopped marks calibration as fully stopped (called after zero crossing).
func (c *ToneGenerator) ConfirmStopped() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.isEnabled = false
	c.isStopping = false
}

// SetEnabled sets whether calibration mode is enabled.
func (c *ToneGenerator) SetEnabled(requestEnable bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if requestEnable {
		// Enabling calibration
		c.isEnabled = true
		c.isStopping = false
	} else if c.isEnabled {
		// Disabling calibration - set stopping flag but keep enabled until the mixer
		// detects zero crossing
		c.isStopping = true
		// Stop any sweep immediately
		if c.sweepCancel != nil {
			c.sweepCancel()
			c.sweepCancel = nil
		}

		c.isSweeping = false
		c.sweepFrequency = c.sweepMin
	}
}

// GetFrequency returns the calibrator frequency in Hz.
func (c *ToneGenerator) GetFrequency() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.frequency
}

// SetFrequency sets the calibrator frequency in Hz.
func (c *ToneGenerator) SetFrequency(frequency float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clamp frequency between 5 and 160 Hz
	c.frequency = max(5, min(160, frequency))
}

// IncreaseFrequency increases the calibrator frequency by the configured frequency increment.
func (c *ToneGenerator) IncreaseFrequency() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.frequency = max(5, min(160, c.frequency+c.frequencyIncrement))

	return c.frequency
}

// DecreaseFrequency decreases the calibrator frequency by the configured frequency increment.
func (c *ToneGenerator) DecreaseFrequency() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.frequency = max(5, min(160, c.frequency-c.frequencyIncrement))

	return c.frequency
}

// GetGain returns the calibrator volume in dB.
func (c *ToneGenerator) GetGain() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.gain
}

// SetGain sets the calibrator gain in dB.
func (c *ToneGenerator) SetGain(volume float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clamp gain between -60 and 0 dB
	c.gain = max(-60, min(0, volume))
}

// IncreaseGain increases the calibrator gain by the configured gain increment.
func (c *ToneGenerator) IncreaseGain() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	increment := 0.25
	if c.config != nil {
		increment = c.config.GetSynthGainIncrement()
	}

	c.gain = max(-60, min(0, c.gain+increment))

	return c.gain
}

// DecreaseGain decreases the calibrator gain by the configured gain increment.
func (c *ToneGenerator) DecreaseGain() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	increment := 0.25
	if c.config != nil {
		increment = c.config.GetSynthGainIncrement()
	}

	c.gain = max(-60, min(0, c.gain-increment))

	return c.gain
}

// GetChannel returns the calibrator output channel selection.
func (c *ToneGenerator) GetChannel() OutputChannel {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.channel
}

// SetChannel sets the calibrator output channel selection.
func (c *ToneGenerator) SetChannel(channel OutputChannel) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Validate channel value
	switch channel {
	case OutputChannelBoth, OutputChannelLeft, OutputChannelRight:
		c.channel = channel
	default:
		c.channel = OutputChannelBoth
	}
}

// GetSweepMin returns the minimum sweep frequency in Hz.
func (c *ToneGenerator) GetSweepMin() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.sweepMin
}

// SetSweepMin sets the minimum sweep frequency in Hz.
func (c *ToneGenerator) SetSweepMin(freq float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clamp between 5 and sweepMax-1
	c.sweepMin = max(5, min(c.sweepMax-1, freq))
}

// GetSweepMax returns the maximum sweep frequency in Hz.
func (c *ToneGenerator) GetSweepMax() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.sweepMax
}

// SetSweepMax sets the maximum sweep frequency in Hz.
func (c *ToneGenerator) SetSweepMax(freq float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clamp between sweepMin+1 and 160
	c.sweepMax = max(c.sweepMin+1, min(160, freq))
}

// GetSweepDuration returns the sweep duration in seconds.
func (c *ToneGenerator) GetSweepDuration() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.sweepDuration
}

// SetSweepDuration sets the sweep duration in seconds.
func (c *ToneGenerator) SetSweepDuration(duration float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clamp between 1 and 60 seconds
	c.sweepDuration = max(1, min(60, duration))
}

// IsSweeping returns whether a frequency sweep is currently active.
func (c *ToneGenerator) IsSweeping() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.isSweeping
}

// GetSweepRangeMode returns the current sweep range mode.
func (c *ToneGenerator) GetSweepRangeMode() SweepRangeMode {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.sweepRangeMode
}

// SetSweepRangeMode sets the sweep range mode and adjusts min/max accordingly.
func (c *ToneGenerator) SetSweepRangeMode(mode SweepRangeMode) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sweepRangeMode = mode

	switch mode {
	case SweepRangeHaptic:
		c.sweepMin, c.sweepMax = c.getHapticRangeFromConfig()
	case SweepRangeFull:
		c.sweepMin = 5
		c.sweepMax = 160
	default:
		c.sweepRangeMode = SweepRangeHaptic
		c.sweepMin, c.sweepMax = c.getHapticRangeFromConfig()
	}
}

// ToggleSweepRangeMode toggles between haptic and full range modes.
func (c *ToneGenerator) ToggleSweepRangeMode() SweepRangeMode {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sweepRangeMode == SweepRangeHaptic {
		c.sweepRangeMode = SweepRangeFull
		c.sweepMin = 5
		c.sweepMax = 160
	} else {
		c.sweepRangeMode = SweepRangeHaptic
		c.sweepMin, c.sweepMax = c.getHapticRangeFromConfig()
	}

	return c.sweepRangeMode
}

// GetSweepFrequency returns the current frequency during a sweep.
// Returns the static frequency if not sweeping.
func (c *ToneGenerator) GetSweepFrequency() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.isSweeping {
		return c.sweepFrequency
	}

	return c.frequency
}

// StartSweep starts a frequency sweep from sweepMin to sweepMax.
// The sweep takes approximately 10 seconds to complete.
// If calibration is not enabled, it will be enabled automatically.
func (c *ToneGenerator) StartSweep() {
	c.mu.Lock()

	// Stop any existing sweep
	if c.sweepCancel != nil {
		c.sweepCancel()
		c.sweepCancel = nil
	}

	// Enable calibration mode if not already enabled
	if !c.isEnabled {
		c.isEnabled = true
	}

	// Create cancellation context
	ctx, cancel := context.WithCancel(context.Background())
	c.sweepCancel = cancel
	c.isSweeping = true
	c.sweepFrequency = c.sweepMin // Start at configured minimum frequency

	c.mu.Unlock()

	// Run sweep in goroutine
	go c.runSweep(ctx)
}

// StopSweep stops an active frequency sweep.
func (c *ToneGenerator) StopSweep() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sweepCancel != nil {
		c.sweepCancel()
		c.sweepCancel = nil
	}

	c.isSweeping = false
	c.sweepFrequency = c.frequency
}

// runSweep executes the frequency sweep loop.
func (c *ToneGenerator) runSweep(ctx context.Context) {
	const stepSize = 1.0

	// Helper function to calculate step duration based on current range and sweep duration
	calcStepDuration := func(minFreq, maxFreq, sweepSecs float64) time.Duration {
		stepCount := (maxFreq - minFreq) / stepSize
		if stepCount <= 0 {
			stepCount = 1
		}

		sweepDuration := time.Duration(sweepSecs * float64(time.Second))

		return sweepDuration / time.Duration(stepCount)
	}

	// Get initial values
	c.mu.RLock()
	minFreq := c.sweepMin
	maxFreq := c.sweepMax
	sweepSecs := c.sweepDuration
	prevMin := minFreq
	prevMax := maxFreq
	prevSweepSecs := sweepSecs

	c.mu.RUnlock()

	stepDuration := calcStepDuration(minFreq, maxFreq, sweepSecs)

	ticker := time.NewTicker(stepDuration)
	defer ticker.Stop()

	currentFreq := minFreq

	for {
		select {
		case <-ctx.Done():
			// Sweep cancelled
			return
		case <-ticker.C:
			// Read current min/max/duration values (they may have changed)
			c.mu.RLock()
			minFreq = c.sweepMin
			maxFreq = c.sweepMax
			sweepSecs = c.sweepDuration
			c.mu.RUnlock()

			// If min/max/duration changed, recalculate step duration and reset ticker
			if minFreq != prevMin || maxFreq != prevMax || sweepSecs != prevSweepSecs {
				prevMin = minFreq
				prevMax = maxFreq
				prevSweepSecs = sweepSecs
				stepDuration = calcStepDuration(minFreq, maxFreq, sweepSecs)
				ticker.Reset(stepDuration)
			}

			// Clamp current frequency to new bounds if needed
			if currentFreq < minFreq {
				currentFreq = minFreq
			}

			if currentFreq > maxFreq {
				currentFreq = minFreq
			}

			c.mu.Lock()
			c.sweepFrequency = currentFreq
			c.mu.Unlock()

			currentFreq += stepSize
			if currentFreq > maxFreq {
				// Sweep complete - restart from minimum
				currentFreq = minFreq
			}
		}
	}
}

// getHapticRangeFromConfig returns the haptic frequency range from config.
// Must be called with lock held.
func (c *ToneGenerator) getHapticRangeFromConfig() (minHz, maxHz float64) {
	if c.config != nil {
		return c.config.GetHapticsPulseMinHz(), c.config.GetHapticsPulseMaxHz()
	}

	return 5, 80
}

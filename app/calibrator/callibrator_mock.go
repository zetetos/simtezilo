package calibrator

// MockCalibrator implements CalibratorInterface for testing.
type MockCalibrator struct {
	enabled   bool
	frequency float64
	gain      float64
	channel   OutputChannel
}

// NewMockCalibrator creates a new mockCalibrator instance.
func NewMockCalibrator(enabled bool, frequency, gain float64, channel OutputChannel) *MockCalibrator {
	return &MockCalibrator{
		enabled:   enabled,
		frequency: frequency,
		gain:      gain,
		channel:   channel,
	}
}

// IsEnabled indicates if calibration mode is active.
func (c *MockCalibrator) IsEnabled() bool {
	return c.enabled
}

// IsStopping indicates if calibration is in the process of stopping.
func (c *MockCalibrator) IsStopping() bool {
	return false
}

// GetFrequency returns the calibration mode frequency.
func (c *MockCalibrator) GetFrequency() float64 {
	return c.frequency
}

// GetGain returns the calibration gain in dB.
func (c *MockCalibrator) GetGain() float64 {
	return c.gain
}

// GetChannel returns the output channel for calibration.
func (c *MockCalibrator) GetChannel() OutputChannel {
	return c.channel
}

// GetSweepFrequency returns the sweep frequency for calibration.
func (c *MockCalibrator) GetSweepFrequency() float64 {
	return c.frequency
}

// ConfirmStopped confirms that calibration has fully stopped.
func (c *MockCalibrator) ConfirmStopped() {}

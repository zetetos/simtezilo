// Package console provides a display and HID implementation that interfaces with the keyboard and a text console.
package console

import (
	"fmt"

	"github.com/zetetos/simtezilo/app/hardware/display"
)

// Console is a display implementation that outputs to a text console.
type Console struct {
	Orientation int
	sleeping    bool
}

// New creates a new Console instance.
func New() *Console {
	return &Console{
		Orientation: 0,
		sleeping:    false,
	}
}

// Clear clears the display (no-op for a text console).
func (d *Console) Clear() {}

// Close closes the display (no-op for a text console).
func (d *Console) Close() {}

// Wakeup wakes up the display from sleep mode.
func (d *Console) Wakeup() {
	d.sleeping = false
}

// Sleep puts the display into sleep mode.
func (d *Console) Sleep() {
	d.sleeping = true
}

// ToggleSleep toggles the sleep state of the display and returns the new state.
func (d *Console) ToggleSleep() bool {
	d.sleeping = !d.sleeping

	return d.sleeping
}

// IsSleeping returns true if the display is in sleep mode.
func (d *Console) IsSleeping() bool {
	return d.sleeping
}

// IsAwake returns true if the display is awake.
func (d *Console) IsAwake() bool {
	return !d.sleeping
}

// GetResolution returns the display resolution (always 0, 0 for a text console).
func (d *Console) GetResolution() (uint16, uint16) {
	return 0, 0
}

// GetDPI returns the display DPI (always 0 for a text console).
func (d *Console) GetDPI() float64 {
	return 0
}

// Write writes content to the text console.
func (d *Console) Write(content *display.Content) error {
	fmt.Println(content.Text) //nolint:forbidigo // expected console output

	return nil
}

// GetOrientation returns the current display orientation in degrees.
func (d *Console) GetOrientation() int {
	return d.Orientation
}

// SetOrientation sets the display orientation to the specified degrees.
func (d *Console) SetOrientation(o int) {
	d.Orientation = o
}

// RotateCW rotates the display orientation 90 degrees clockwise and returns the new orientation.
func (d *Console) RotateCW() int {
	d.Orientation += 90
	if d.Orientation >= 360 {
		d.Orientation = 0
	}

	return d.Orientation
}

// RotateCCW rotates the display orientation 90 degrees counter-clockwise and returns the new orientation.
func (d *Console) RotateCCW() int {
	d.Orientation -= 90
	if d.Orientation < 0 {
		d.Orientation = 270
	}

	return d.Orientation
}

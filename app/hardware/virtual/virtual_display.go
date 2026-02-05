package virtual

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/zetetos/simtezilo/app/hardware/display"
)

// Display is a virtual display that captures rendered content for saving as PNG.
type Display struct {
	width       uint16
	height      uint16
	dpi         float64
	orientation int
	sleeping    bool
	canvas      *image.RGBA
}

// NewDisplay creates a new Display instance.
func NewDisplay(width uint16, height uint16, dpi float64) *Display {
	return &Display{
		width:       width,
		height:      height,
		dpi:         dpi,
		orientation: 0,
		sleeping:    false,
		canvas:      nil,
	}
}

// Clear clears the display.
func (d *Display) Clear() {
	d.canvas = nil
}

// Close closes the display.
func (d *Display) Close() {}

// Wakeup wakes up the display from sleep mode.
func (d *Display) Wakeup() {
	d.sleeping = false
}

// Sleep puts the display into sleep mode.
func (d *Display) Sleep() {
	d.sleeping = true
}

// ToggleSleep toggles the sleep state of the display.
func (d *Display) ToggleSleep() bool {
	d.sleeping = !d.sleeping

	return d.sleeping
}

// IsSleeping returns true if the display is in sleep mode.
func (d *Display) IsSleeping() bool {
	return d.sleeping
}

// IsAwake returns true if the display is awake.
func (d *Display) IsAwake() bool {
	return !d.sleeping
}

// GetResolution returns the display resolution.
func (d *Display) GetResolution() (uint16, uint16) {
	return d.width, d.height
}

// GetDPI returns the display DPI.
func (d *Display) GetDPI() float64 {
	return d.dpi
}

// Write captures the content canvas for later saving.
func (d *Display) Write(content *display.Content) error {
	d.canvas = content.Canvas

	return nil
}

// GetOrientation returns the current display orientation in degrees.
func (d *Display) GetOrientation() int {
	return d.orientation
}

// SetOrientation sets the display orientation to the specified degrees.
func (d *Display) SetOrientation(degrees int) {
	d.orientation = degrees
}

// RotateCW rotates the display orientation 90 degrees clockwise.
func (d *Display) RotateCW() int {
	d.orientation += 90
	if d.orientation >= 360 {
		d.orientation = 0
	}

	return d.orientation
}

// RotateCCW rotates the display orientation 90 degrees counter-clockwise.
func (d *Display) RotateCCW() int {
	d.orientation -= 90
	if d.orientation < 0 {
		d.orientation = 270
	}

	return d.orientation
}

// GetCanvas returns the captured canvas.
func (d *Display) GetCanvas() *image.RGBA {
	return d.canvas
}

// SavePNG saves the captured canvas to a PNG file.
func (d *Display) SavePNG(filename string) error {
	if d.canvas == nil {
		return errors.New("no canvas to save")
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	err = png.Encode(file, d.canvas)
	if err != nil {
		return fmt.Errorf("encode PNG: %w", err)
	}

	return nil
}

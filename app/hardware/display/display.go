// Package display provides types and functions for managing display content.
package display

import "image"

// Content represents the content to be displayed, either as text or as a graphical canvas.
type Content struct {
	Text   string
	Canvas *image.RGBA
}

// SumAngle90 sums two angles in degrees with the result being a valid 90-degree angle between 0 and 270 degrees.
// If a non-90 degree rotation is provided, the first angle is returned unmodified.
func SumAngle90(angle1 int, angle2 int) int {
	angle := angle1 + angle2

	if angle%90 != 0 {
		return angle1
	}

	return angle % 360
}

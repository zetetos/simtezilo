// Package haptics provides definitions and profiles for simulating engine haptic feedback.
package haptics

// Profile names are structured as follows:
// <Layout><Chambers>.B<Bank angle>.C<Crank Plane angle>
//
//
// Engine Layouts:
// D: Radial (reserved)
// H: Horizontally opposed
// I: Inline/straight
// K: Wankel rotary (kreiskolbenmotor/KKM)
// R: Rotary (reserved)
// S: 2 stroke
// V: V
// W: W

// EngineProfile defines the characteristics of an engine's haptic feedback.
// PulseScale reduces the number of pulses per revolution. For engines with high chamber counts
// and/or very high RPM the pulse rate results in haptic feedback at too high a frequency which
// falls outside of the tactile range.
type EngineProfile struct {
	PrimaryBalance   float64 `json:"primaryBalance"`   // 0.0 (unbalanced) to 1.0 (perfectly balanced)
	SecondaryBalance float64 `json:"secondaryBalance"` // 0.0 (unbalanced) to 1.0 (perfectly balanced)
	Gain             float64 `json:"gain"`             // additional gain offset to apply to the haptic feedback
	PulseScale       float64 `json:"pulseScale"`       // 1.0 for no scaling, <1.0 for reduced pulse rate
}

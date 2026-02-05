package config

import "time"

const (
	// MinimumGain is the minimum gain level at which haptic output will be silenced.
	MinimumGain = -60.0

	// MaximumGain is the maximum allowable gain level for haptic output.
	MaximumGain = 0.0

	// FadeInDuration is the duration over which the haptic gain fade in will occur.
	FadeInDuration = 1 * time.Second
)

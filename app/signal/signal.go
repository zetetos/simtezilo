// Package signal provides various signal processing functions.
package signal

import (
	"math"

	"github.com/zetetos/simtezilo/app/config"
)

// Abs returns the absolute value of a float64.
func Abs(value float64) float64 {
	if value < 0 {
		return -value
	}

	return value
}

// Equalize applies equalization based on pulse width and synthesizer settings for a specific channel.
// Uses a precomputed EQ curve for efficient lookup instead of calculating per-sample.
func Equalize(value float64, pulseWidth float64, channel int, cfg *config.Config) float64 {
	// Check if EQ is enabled for this channel
	if !cfg.GetSynthChannelEqEnabled(channel) {
		return value
	}

	sampleRate := cfg.GetSynthInternalSampleRateHz()
	freq := float64(sampleRate) / (2 * pulseWidth)

	curve, minFreq, resolution := cfg.GetSynthChannelEqCurve(channel)
	if len(curve) == 0 {
		return value // No EQ curve computed
	}

	// Calculate bucket index for this frequency
	index := int((freq - minFreq) / resolution)

	// Bounds check
	if index < 0 || index >= len(curve) {
		return value // Outside EQ range
	}

	// Apply precomputed amplitude ratio
	return value * curve[index]
}

// Exponent raises a value to a given exponent.
// If the value is negative, the exponentiation is applied to its absolute value, and the sign is restored.
func Exponent(value float64, exponent float64) float64 {
	isNeg := false

	if value < 0 {
		isNeg = true
		value = -value
	}

	result := math.Pow(value, exponent)

	if isNeg {
		result = -result
	}

	return result
}

// LargestMagnitude returns the largest magnitude of two values, preserving their signs.
func LargestMagnitude(valueA float64, valueB float64) float64 {
	aIsNeg := false
	bIsNeg := false

	if valueA < 0 {
		aIsNeg = true
		valueA = -valueA
	}

	if valueB < 0 {
		bIsNeg = true
		valueB = -valueB
	}

	var maxVal float64

	if valueA > valueB {
		maxVal = valueA

		if aIsNeg {
			maxVal = -maxVal
		}
	} else {
		maxVal = valueB

		if bIsNeg {
			maxVal = -maxVal
		}
	}

	return maxVal
}

// LimitWindow constrains a value within a specified minimum and maximum range.
// If the value is negative, the constraints are applied to its absolute value, and the sign is restored.
func LimitWindow(value float64, minValue float64, maxValue float64) (float64, bool) {
	var cMin, cMax bool

	value, cMin = LimitMin(value, minValue)
	value, cMax = LimitMax(value, maxValue)

	if cMin || cMax {
		return value, true
	}

	return value, false
}

// LimitMin constrains a value to be no less than the specified minimum.
// If the value is negative, the minimum constraint is applied to its absolute value, and the sign is restored.
func LimitMin(value float64, minValue float64) (float64, bool) {
	isNeg := false
	if value < 0 {
		isNeg = true
		value = -value
	}

	value = max(value, minValue)

	if value < minValue {
		value = minValue
	}

	if isNeg {
		value = -value
	}

	return value, false
}

// LimitMax constrains a value to be no greater than the specified maximum.
// If the value is negative, the maximum constraint is applied to its absolute value, and the sign is restored.
func LimitMax(value float64, maxValue float64) (float64, bool) {
	isNeg := false
	if value < 0 {
		isNeg = true
		value = -value
	}

	value = min(value, maxValue)

	if isNeg {
		value = -value
	}

	return value, false
}

// Log2 applies a base 2 logarithmic transformation to the given value.
// If the value is negative, the transformation is applied to its absolute value, and the sign is restored.
func Log2(value float64) float64 {
	isNeg := false

	if value < 0 {
		isNeg = true
		value = -value
	}

	compressed := math.Log2(value + 1)

	if isNeg {
		compressed = -compressed
	}

	return compressed
}

// Log10 applies a base 10 logarithmic transformation to the given value.
// If the value is negative, the transformation is applied to its absolute value, and the sign is restored.
func Log10(value float64) float64 {
	isNeg := false

	if value < 0 {
		isNeg = true
		value = -value
	}

	compressed := math.Log10(value + 1)

	if isNeg {
		compressed = -compressed
	}

	return compressed
}

// Scale multiplies the given value by the specified scale factor.
func Scale(value float64, scale float64) float64 {
	return value * scale
}

// Polarity returns -1.0 for negative values and +1.0 for zero or positive values.
func Polarity(value float64) float64 {
	if value < 0 {
		return -1.0
	}

	return 1.0
}

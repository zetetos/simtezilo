package synthesizer

import (
	"math"
)

// GainToPowerRatio converts a gain value in dB to a power ratio magnitude.
func GainToPowerRatio(gain float64) float64 {
	if gain == 0 {
		return 1.0
	}

	powerRatio := math.Pow(10, gain/10)

	return powerRatio
}

// GainToAmplitudeRatio converts a gain value in dB to an amplitude magnitude.
func GainToAmplitudeRatio(gain float64) float64 {
	if gain == 0 {
		return 1.0
	}

	amplitudeRatio := math.Pow(10, gain/20)

	return amplitudeRatio
}

// FindSampleZeroCrossing searches for the first zero point or crossing in a given array of samples
// Returns the position of the zero crossing and the polarity just before the crossing (-1 or 1)
// If no zero point or crossing is found then it returns the first index and polarity of first sample.
func FindSampleZeroCrossing(samples []float64) (offset int, polarity int) {
	if len(samples) <= 1 {
		return handleShortSampleArray(samples)
	}

	polarity = getInitialPolarity(samples[0])
	offset = searchForZeroCrossing(samples)

	return offset, polarity
}

// handleShortSampleArray handles arrays with 0 or 1 samples.
func handleShortSampleArray(samples []float64) (int, int) {
	offset := 0
	polarity := 1

	if len(samples) == 1 && samples[0] < 0 {
		polarity = -1
	}

	return offset, polarity
}

// getInitialPolarity determines the initial polarity from the first sample.
func getInitialPolarity(firstSample float64) int {
	if firstSample < 0 {
		return -1
	}

	return 1
}

// searchForZeroCrossing searches for zero crossings in the sample array.
func searchForZeroCrossing(samples []float64) int {
	searchRange := len(samples)

	for index := range searchRange - 1 {
		currentSample := samples[index]
		nextSample := samples[index+1]

		if offset, found := checkExactZero(samples, index); found {
			return offset
		}

		if offset, found := checkPolarityChange(currentSample, nextSample, index); found {
			return offset
		}
	}

	return 0 // No zero crossing found, return first index
}

// checkExactZero checks if the current sample is exactly zero.
func checkExactZero(samples []float64, index int) (int, bool) {
	if samples[index] == 0.0 {
		return index, true
	}

	return 0, false
}

// checkPolarityChange checks if there's a polarity change between current and next sample.
func checkPolarityChange(currentSample, nextSample float64, index int) (int, bool) {
	polarityChanged := (currentSample > 0 && nextSample < 0) || (currentSample < 0 && nextSample > 0)
	if polarityChanged {
		return index + 1, true
	}

	return 0, false
}

// SamplePolarity returns the polarity of the given samples (-1 negative, 1 positive).
// The samples are checked from the last to the first and the polority is returned
// as soon as a non-zero sample is encountered.
// If all samples are zero, it returns positive polarity.
func SamplePolarity(samples []float64) float64 {
	for i := len(samples) - 1; i >= 0; i-- {
		if samples[i] < 0 {
			return -1
		}
	}

	return 1
}

// InvertSamplePolarity inverts the polarity of the given samples.
func InvertSamplePolarity(samples *[]float64) {
	for i := range *samples {
		(*samples)[i] = -(*samples)[i]
	}
}

// ScaleSamples adjusts the scale of the samples by the given magnitude.
func ScaleSamples(samples *[]float64, magnitude float64) {
	for i := range *samples {
		(*samples)[i] *= magnitude
	}
}

// scaleSamplesPeak adjusts the gain on a slice of samples using the peak value.
func scaleSamplesPeak(samples *[]float64, peak float64) {
	if peak < 1.0 {
		return
	}

	scale := 1.0 / peak

	ScaleSamples(samples, scale)
}

// mixSampleSum mixes two samples using a simple sum algorithm.
// Returns the mixed sample and the peak value which is later used to scale a slice of samples.
func mixSampleSum(sample1 float64, sample2 float64, peak *float64) float64 {
	sum := sample1 + sample2

	sumAbs := math.Abs(sum)

	if sumAbs > *peak {
		*peak = sumAbs
	}

	return sum
}

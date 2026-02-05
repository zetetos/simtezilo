package synthesizer

import (
	"math"
	"time"

	"github.com/zetetos/simtezilo/app/codec"
)

const (
	effectsSampleRateHz = 32000 // Base sample rate at which sound effects are rendered
)

// EffectSample represents a pre-generated audio sample for a sound effect.
type EffectSample struct {
	Name   string
	Sample map[int]codec.PCMFloat64
}

// EffectsSampleBank holds pre-generated audio samples for various sound effects.
type EffectsSampleBank struct {
	samples map[string]EffectSample
}

// NewEffectsSampleBank initializes and returns a new EffectsSampleBank with pre-generated samples.
func NewEffectsSampleBank() *EffectsSampleBank {
	return &EffectsSampleBank{
		samples: map[string]EffectSample{
			"gearShift": {
				Name: "gearShift",
				Sample: map[int]codec.PCMFloat64{
					effectsSampleRateHz: generateGearShiftSample(),
				},
			},
			"talkPermitTone": {
				Name: "talkPermitTone",
				Sample: map[int]codec.PCMFloat64{
					effectsSampleRateHz: generateTalkPermitToneSample(),
				},
			},
			"recordingStartTone": {
				Name: "recordingStartTone",
				Sample: map[int]codec.PCMFloat64{
					effectsSampleRateHz: generateRecordingStartToneSample(),
				},
			},
			"recordingStopTone": {
				Name: "recordingStopTone",
				Sample: map[int]codec.PCMFloat64{
					effectsSampleRateHz: generateRecordingStopToneSample(),
				},
			},
			"errorTone": {
				Name: "errorTone",
				Sample: map[int]codec.PCMFloat64{
					effectsSampleRateHz: generateErrorToneSample(),
				},
			},
		},
	}
}

// GetSample retrieves a pre-generated sample with the given name and sample rate.
// If the sample does not exist, it returns an empty slice.
// The effect is resampled to the requested sample rate if necessary.
func (s *EffectsSampleBank) GetSample(name string, sampleRate int) codec.PCMFloat64 {
	if _, ok := s.samples[name]; !ok {
		return codec.PCMFloat64{}
	}

	effect := s.samples[name]

	// Use the cached sample when it exists
	sample, ok := effect.Sample[sampleRate]
	if !ok {
		// Resample and cache the new sample rate when it doesn't exist
		baseSample := effect.Sample[effectsSampleRateHz]
		if baseSample.Len() == 0 {
			return codec.PCMFloat64{}
		}

		sample = baseSample.Resample(sampleRate)
		// Update the cache by modifying the original map entry
		s.samples[name].Sample[sampleRate] = sample
	}

	// Check if sample is empty after potential resampling
	if sample.Len() == 0 {
		return codec.PCMFloat64{}
	}

	// TODO: copying to a new sample as the slice is scaled by magnitude in-place which
	// causes the effect volume to be reduced every time it is played
	copied := make([]float64, sample.Len())
	copy(copied, sample.Samples())

	sampleCopy := *codec.NewPCMFloat64(
		copied,
		sample.SampleRate(),
		sample.Channels(),
	)

	return sampleCopy
}

// generateGearShiftSample creates a sample for the gear shift sound effect.
func generateGearShiftSample() codec.PCMFloat64 {
	sampleLengthSeconds := 0.1
	pulseAmplitude := 2.0
	pulseHz := 30
	decayRate := 1 - (5 / (float64(effectsSampleRateHz) * sampleLengthSeconds))

	sampleCount := int(sampleLengthSeconds * float64(effectsSampleRateHz))

	pulseWidth := effectsSampleRateHz / (2 * pulseHz)
	waveSamplePeriod := math.Pi / float64(pulseWidth)
	waveOffset := float64(pulseWidth)

	samples := make([]float64, sampleCount)

	for i := range samples {
		angle := waveSamplePeriod * (float64(i) - waveOffset)
		samples[i] = pulseAmplitude * math.Sin(angle)

		pulseAmplitude *= decayRate
	}

	return *codec.NewPCMFloat64(samples, effectsSampleRateHz, 1)
}

// generateTalkPermitToneSample creates a sample for the talk permit tone sequence.
func generateTalkPermitToneSample() codec.PCMFloat64 {
	toneSequence := "746839456"

	return generateDTMFSequence(toneSequence, 20*time.Millisecond, 0*time.Millisecond)
}

// generateRecordingStartToneSample creates a sample for the recording start tone sequence.
func generateRecordingStartToneSample() codec.PCMFloat64 {
	toneSequence := "79"

	return generateDTMFSequence(toneSequence, 48*time.Millisecond, 2*time.Millisecond)
}

// generateRecordingStopToneSample creates a sample for the recording stop tone sequence.
func generateRecordingStopToneSample() codec.PCMFloat64 {
	toneSequence := "97"

	return generateDTMFSequence(toneSequence, 48*time.Millisecond, 2*time.Millisecond)
}

// generateErrorToneSample creates a sample for the error tone sequence.
func generateErrorToneSample() codec.PCMFloat64 {
	toneSequence := "1111"

	return generateDTMFSequence(toneSequence, 24*time.Millisecond, 2*time.Millisecond)
}

// generateTalkPermitToneSample creates a sample for the talk permit tone sequence.
func generateDTMFSequence(toneSequence string, toneLength time.Duration, silenceLength time.Duration) codec.PCMFloat64 {
	tones := [][]float64{}

	samples := make([]float64, 0)

	// Generate tones for each character in the value
	for _, char := range toneSequence {
		tone := generateDTMFTone(string(char), toneLength)
		if len(tone) == 0 {
			// Skip empty tones but continue processing
			continue
		}

		tones = append(tones, tone)
	}

	// Append tones with silence in between
	for i, tone := range tones {
		samples = append(samples, tone...)

		// Add silence between tones
		if i < len(tones)-1 {
			silenceSampleCount := int(float64(silenceLength.Milliseconds()) * float64(effectsSampleRateHz) / 1000.0)
			silence := make([]float64, silenceSampleCount)
			samples = append(samples, silence...)
		}
	}

	return *codec.NewPCMFloat64(samples, effectsSampleRateHz, 1)
}

// generateDTMFTone generates a DTMF tone for a given character and duration.
func generateDTMFTone(value string, length time.Duration) []float64 {
	tones := map[rune][2]float64{
		'1': {697, 1209},
		'2': {697, 1336},
		'3': {697, 1477},
		'A': {697, 1633},
		'4': {770, 1209},
		'5': {770, 1336},
		'6': {770, 1477},
		'B': {770, 1633},
		'7': {852, 1209},
		'8': {852, 1336},
		'9': {852, 1477},
		'C': {852, 1633},
		'*': {941, 1209},
		'0': {941, 1336},
		'#': {941, 1477},
		'D': {941, 1633},
	}

	sampleCount := int(length.Seconds() * float64(effectsSampleRateHz))
	audioSample := make([]float64, sampleCount)

	if len(value) != 1 {
		return audioSample
	}

	frequencies, ok := tones[rune(value[0])]
	if !ok {
		return audioSample
	}

	fadeLength := int(float64(sampleCount) * 0.15)

	for index := range audioSample {
		phase := float64(index) / float64(effectsSampleRateHz)

		// Generate the base DTMF tone
		sample := 0.25 * (math.Sin(2*math.Pi*frequencies[0]*phase) + math.Sin(2*math.Pi*frequencies[1]*phase))

		// Apply fade in/out envelope
		envelope := float64(1.0)

		if index < fadeLength {
			envelope = float64(index) / float64(fadeLength)
		} else if index >= sampleCount-fadeLength {
			envelope = float64(sampleCount-index-1) / float64(fadeLength)
		}

		audioSample[index] = sample * envelope
	}

	return audioSample
}

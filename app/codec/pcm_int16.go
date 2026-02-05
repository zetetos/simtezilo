package codec

import (
	"fmt"
)

// PCMInt16 represents PCM audio samples in int16 format.
type PCMInt16 struct {
	samples    []int16
	sampleRate int
	channels   int
}

// NewPCMInt16 creates a new PCMInt16 instance with the given samples, sample rate, and number of channels.
func NewPCMInt16(samples []int16, sampleRate, channels int) *PCMInt16 {
	return &PCMInt16{
		samples:    samples,
		sampleRate: sampleRate,
		channels:   channels,
	}
}

// Samples returns the PCM int16 audio samples.
func (p *PCMInt16) Samples() []int16 {
	return p.samples
}

// SampleRate returns the sample rate of the PCM data.
func (p *PCMInt16) SampleRate() int {
	return p.sampleRate
}

// Channels returns the number of channels in the PCM data.
func (p *PCMInt16) Channels() int {
	return p.channels
}

// Len returns the number of samples in the PCM data.
func (p *PCMInt16) Len() int {
	return len(p.samples)
}

// Resample resamples PCM int16 audio samples to the specified sample rate and returns a new PCMInt16 struct.
func (p *PCMInt16) Resample(toSampleRate int) PCMInt16 {
	if p.sampleRate == toSampleRate {
		return *p
	}

	// Convert int16 samples to float64 format for processing
	float64Samples := make([]float64, len(p.samples))
	for i, sample := range p.samples {
		float64Samples[i] = float64(sample) / 32767.0
	}

	pcmFloat64 := PCMFloat64{
		samples:    float64Samples,
		sampleRate: p.sampleRate,
		channels:   p.channels,
	}

	// Resample using PCMFloat64 method
	resampledPCMFloat64 := pcmFloat64.Resample(toSampleRate)

	// Convert back to int16 format
	resampledPCMInt16 := make([]int16, len(resampledPCMFloat64.samples))
	for i, sample := range resampledPCMFloat64.samples {
		resampledPCMInt16[i] = int16(sample * 32767.0)
	}

	return PCMInt16{
		samples:    resampledPCMInt16,
		sampleRate: toSampleRate,
		channels:   p.channels,
	}
}

// ToStereo converts mono PCM int16 audio samples to stereo by duplicating the mono channel.
func (p *PCMInt16) ToStereo() PCMInt16 {
	if p.channels > 1 {
		return *p
	}

	stereoSamples := make([]int16, len(p.samples)*2)
	for i, sample := range p.samples {
		stereoSamples[i*2] = sample   // Left channel
		stereoSamples[i*2+1] = sample // Right channel (duplicate for mono)
	}

	return PCMInt16{
		samples:    stereoSamples,
		sampleRate: p.sampleRate,
		channels:   2,
	}
}

// ToDCA encodes PCM int16 audio samples to Discord audio format (DCA).
func (p *PCMInt16) ToDCA() ([]byte, error) {
	pcmInt16 := p.Resample(OpusSampleRate)

	pcmInt16 = pcmInt16.ToStereo()

	dcaData, err := encodeDCA(pcmInt16)
	if err != nil {
		return nil, fmt.Errorf("int16 -> DCA: %w", err)
	}

	return dcaData, nil
}

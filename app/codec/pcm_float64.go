package codec

import (
	"fmt"

	"github.com/gopxl/beep"
)

// PCMFloat64 represents PCM audio samples in float64 format.
type PCMFloat64 struct {
	samples    []float64
	sampleRate int
	channels   int
}

// NewPCMFloat64 creates a new PCMFloat64 instance with the given samples, sample rate, and number of channels.
func NewPCMFloat64(samples []float64, sampleRate, channels int) *PCMFloat64 {
	return &PCMFloat64{
		samples:    samples,
		sampleRate: sampleRate,
		channels:   channels,
	}
}

// Samples returns the PCM float64 audio samples.
func (p *PCMFloat64) Samples() []float64 {
	return p.samples
}

// SampleRate returns the sample rate of the PCM data.
func (p *PCMFloat64) SampleRate() int {
	return p.sampleRate
}

// Channels returns the number of channels in the PCM data.
func (p *PCMFloat64) Channels() int {
	return p.channels
}

// Len returns the number of samples in the PCM data.
func (p *PCMFloat64) Len() int {
	return len(p.samples)
}

// Resample resamples PCM float64 audio samples to the specified sample rate and returns a new PCMFloat64 struct.
func (p *PCMFloat64) Resample(toSampleRate int) PCMFloat64 {
	if p.sampleRate == toSampleRate {
		return *p
	}

	// Convert float64 samples to beep format (stereo float64 pairs)
	// Since we're dealing with mono samples, duplicate to both channels
	beepSamples := make([][2]float64, len(p.samples))
	for i, sample := range p.samples {
		beepSamples[i][0] = sample // Left channel
		beepSamples[i][1] = sample // Right channel (duplicate for mono)
	}

	// Create a slice streamer from the samples
	streamer := &sliceStreamer{
		samples: beepSamples,
		pos:     0,
	}

	// Create resampler
	resampler := newResampleStreamer(
		streamer,
		beep.SampleRate(p.sampleRate),
		beep.SampleRate(toSampleRate),
	)

	// Calculate expected output length
	ratio := float64(toSampleRate) / float64(p.sampleRate)
	outputLen := int(float64(len(p.samples)) * ratio)
	outputSamples := make([][2]float64, outputLen)

	// Resample the audio
	totalSamples := 0
	for totalSamples < outputLen {
		n, ok := resampler.Stream(outputSamples[totalSamples:])
		if !ok {
			break
		}

		totalSamples += n
	}

	newPCMFloat64 := PCMFloat64{
		samples:    make([]float64, totalSamples),
		sampleRate: toSampleRate,
		channels:   p.channels,
	}

	// Convert back to float64 mono (using left channel since we duplicated mono to stereo)
	for i := range totalSamples {
		newPCMFloat64.samples[i] = outputSamples[i][0]
	}

	return newPCMFloat64
}

// ToStereo converts mono PCM float64 samples to stereo by duplicating the mono channel.
func (p *PCMFloat64) ToStereo() PCMFloat64 {
	if p.channels > 1 {
		return *p
	}

	stereoSamples := make([]float64, len(p.samples)*2)
	for i, sample := range p.samples {
		stereoSamples[i*2] = sample   // Left channel
		stereoSamples[i*2+1] = sample // Right channel (duplicate for mono)
	}

	return PCMFloat64{
		samples:    stereoSamples,
		sampleRate: p.sampleRate,
		channels:   2,
	}
}

// ToDCA converts Float64 PCM samples to Discord audio format (DCA).
func (p *PCMFloat64) ToDCA() ([]byte, error) {
	pcmInt16 := p.ToInt16()

	dcaData, err := pcmInt16.ToDCA()
	if err != nil {
		return []byte{}, fmt.Errorf("float64 -> int16 -> DCA: %w", err)
	}

	return dcaData, nil
}

// ToInt16 converts Float64 PCM samples to int16 format.
func (p *PCMFloat64) ToInt16() PCMInt16 {
	samples := make([]int16, len(p.samples))

	for i, sample := range p.samples {
		samples[i] = int16(sample * 32767.0)
	}

	return PCMInt16{
		samples:    samples,
		sampleRate: p.sampleRate,
		channels:   p.channels,
	}
}

// sliceStreamer is a simple streamer that reads from a slice of samples.
type sliceStreamer struct {
	samples [][2]float64
	pos     int
}

func (s *sliceStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	if s.pos >= len(s.samples) {
		return 0, false
	}

	n = copy(samples, s.samples[s.pos:])
	s.pos += n

	return n, n > 0
}

func (s *sliceStreamer) Err() error {
	return nil
}

// resampleStreamer performs simple linear interpolation resampling.
type resampleStreamer struct {
	streamer   beep.Streamer
	ratio      float64
	buffer     [][2]float64
	bufferPos  float64
	bufferFill int
}

// newResampleStreamer creates a new resampleStreamer to convert from oldRate to newRate.
func newResampleStreamer(streamer beep.Streamer, oldRate, newRate beep.SampleRate) *resampleStreamer {
	return &resampleStreamer{
		streamer: streamer,
		ratio:    float64(oldRate) / float64(newRate),
		buffer:   make([][2]float64, 1024),
	}
}

// Stream fills the provided samples slice with resampled audio data.
func (r *resampleStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	for index := range samples {
		// Check if we need more data in buffer
		for int(r.bufferPos)+1 >= r.bufferFill {
			var bufOk bool

			r.bufferFill, bufOk = r.streamer.Stream(r.buffer)
			if !bufOk {
				return index, index > 0
			}

			r.bufferPos = 0
		}

		// Linear interpolation
		pos := int(r.bufferPos)
		frac := r.bufferPos - float64(pos)

		if pos+1 < r.bufferFill {
			// Interpolate left channel
			samples[index][0] = r.buffer[pos][0]*(1-frac) + r.buffer[pos+1][0]*frac
			// Interpolate right channel
			samples[index][1] = r.buffer[pos][1]*(1-frac) + r.buffer[pos+1][1]*frac
		} else {
			// Use last sample if at end
			samples[index] = r.buffer[pos]
		}

		r.bufferPos += r.ratio
		n++
	}

	return n, true
}

// Err returns any error encountered during streaming.
func (r *resampleStreamer) Err() error {
	return r.streamer.Err()
}

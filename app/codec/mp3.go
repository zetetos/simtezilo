package codec

import (
	"bytes"
	"fmt"
	"io"

	"github.com/gopxl/beep/mp3"
)

type MP3 struct {
	data []byte
}

func NewMP3(data []byte) *MP3 {
	return &MP3{data: data}
}

// ToDCA converts MP3 audio data to Discord audio format (DCA).
func (m *MP3) ToDCA() ([]byte, error) {
	pcmInt16, err := m.ToPCMInt16()
	if err != nil {
		return []byte{}, fmt.Errorf("convert MP3 to int16: %w", err)
	}

	dcaData, err := pcmInt16.ToDCA()
	if err != nil {
		return []byte{}, fmt.Errorf("MP3 -> int16 -> DCA: %w", err)
	}

	return dcaData, nil
}

// ToPCMInt16 decodes MP3 data to PCM int16 format.
func (m *MP3) ToPCMInt16() (PCMInt16, error) {
	reader := io.NopCloser(bytes.NewReader(m.data))

	// Decode the MP3 data
	streamer, format, err := mp3.Decode(reader)
	if err != nil {
		return PCMInt16{}, fmt.Errorf("failed to decode MP3: %w", err)
	}
	defer streamer.Close()

	pcmInt16 := PCMInt16{
		samples:    make([]int16, 0, 512),
		sampleRate: int(format.SampleRate),
		channels:   format.NumChannels,
	}

	decodedSamples := make([][2]float64, 512)

	for {
		sampleCount, ok := streamer.Stream(decodedSamples)
		if !ok {
			break
		}

		// Convert stereo samples to mono/interleaved format
		for index := range sampleCount {
			pcmInt16.samples = append(pcmInt16.samples, int16(decodedSamples[index][0]*32767)) // Left channel
			pcmInt16.samples = append(pcmInt16.samples, int16(decodedSamples[index][1]*32767)) // Right channel
		}
	}

	return pcmInt16, nil
}

// ToPCMFloat64 decodes MP3 data to PCM float64 format.
func (m *MP3) ToPCMFloat64() (PCMFloat64, error) {
	reader := io.NopCloser(bytes.NewReader(m.data))

	// Decode the MP3 data
	streamer, format, err := mp3.Decode(reader)
	if err != nil {
		return PCMFloat64{}, fmt.Errorf("failed to decode MP3: %w", err)
	}
	defer streamer.Close()

	pcmFloat64 := PCMFloat64{
		samples:    make([]float64, 0, 512),
		sampleRate: int(format.SampleRate),
		channels:   format.NumChannels,
	}

	decodedSamples := make([][2]float64, 512)

	for {
		sampleCount, ok := streamer.Stream(decodedSamples)
		if !ok {
			break
		}

		// Convert stereo samples to mono/interleaved format
		for index := range sampleCount {
			pcmFloat64.samples = append(pcmFloat64.samples, decodedSamples[index][0]) // Left channel
			pcmFloat64.samples = append(pcmFloat64.samples, decodedSamples[index][1]) // Right channel
		}
	}

	return pcmFloat64, nil
}

package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"layeh.com/gopus"
)

const (
	OpusSampleRate   = 48000 // Discord requires 48kHz sample rate
	OpusChannels     = 2     // Discord requires stereo audio
	OpusFrameSize    = 960   // 20ms frame size at 48kHz
	OpusMaxFrameSize = 3840  // Maximum bytes per Opus frame
)

// encodeDCA encodes PCM int16 samples to Discord audio format (DCA) using the provided Opus encoder.
func encodeDCA(pcm PCMInt16) ([]byte, error) {
	var dcaBuffer bytes.Buffer

	frameSamples := OpusFrameSize * OpusChannels

	// Create stereo Opus encoder for Discord (48kHz, stereo)
	encoder, err := gopus.NewEncoder(OpusSampleRate, OpusChannels, gopus.Voip)
	if err != nil {
		return nil, fmt.Errorf("create Opus encoder: %w", err)
	}

	// Process audio in frames
	for index := 0; index < pcm.Len(); index += frameSamples {
		var frame []int16

		end := index + frameSamples

		if end > pcm.Len() {
			// Pad the last frame with zeros
			frame = make([]int16, frameSamples)
			copy(frame, pcm.Samples()[index:])
		} else {
			frame = pcm.Samples()[index:end]
		}

		// Encode the frame
		opusData, err := encoder.Encode(frame, OpusFrameSize, OpusMaxFrameSize)
		if err != nil {
			return nil, fmt.Errorf("encode Opus frame: %w", err)
		}

		if len(opusData) > 0 {
			// Write DCA format: length (2 bytes) + opus data
			frameLen := len(opusData)
			if frameLen <= 65535 { // max int16 value
				// #nosec G115: frameLen is checked to be within int16 range
				_ = binary.Write(&dcaBuffer, binary.LittleEndian, int16(frameLen))
				dcaBuffer.Write(opusData)
			}
		}
	}

	return dcaBuffer.Bytes(), nil
}

package synthesizer

import (
	"github.com/gopxl/beep"
	"github.com/rs/zerolog"
)

type OutputDevice struct {
	samples map[string]*beep.Buffer
	log     zerolog.Logger
}

type SynthOutDeviceOpts struct {
	OutputFile string
	Log        zerolog.Logger
}

func NewOutputDevice(opts SynthOutDeviceOpts) (*OutputDevice, error) {
	buffers := map[string]*beep.Buffer{}

	return &OutputDevice{
		samples: buffers,
		log:     opts.Log,
	}, nil
}

type HapticStream struct {
	streamer beep.Streamer
}

func NewHapticStream(synth *Synthesizer, outputSampleRate beep.SampleRate) *HapticStream {
	// Create the internal streamer that works at the synthesizer's native sample rate (8kHz)
	internalStreamer := &Streamer{synth: synth}

	var streamer beep.Streamer

	internalSampleRate := beep.SampleRate(synth.GetSampleRate())
	if internalSampleRate == outputSampleRate {
		// No resampling needed, return the internal streamer directly
		streamer = internalStreamer
	} else {
		// Create a streamer from 8kHz to the output sample rate (32kHz)
		// Quality level 4 provides good performance for real-time resampling with good quality
		streamer = beep.Resample(4, beep.SampleRate(synth.GetSampleRate()), outputSampleRate, internalStreamer)
	}

	return &HapticStream{
		streamer: streamer,
	}
}

// Streamer handles streaming at the synthesizer's native sample rate (8kHz).
type Streamer struct {
	synth *Synthesizer
}

// OutputChannelSettings holds the settings for an output channel.
type OutputChannelSettings struct {
	Gain float64
	Mute bool
}

// NewStreamer creates a new Streamer for the given Synthesizer.
func NewStreamer(synth *Synthesizer) *Streamer {
	return &Streamer{
		synth: synth,
	}
}

func (s *Streamer) Stream(samples [][2]float64) (n int, ok bool) {
	// Trigger mixing to all master channels
	s.synth.mixer.MixToMaster(len(samples))

	// If master is muted, fill output with zeros and return early
	if s.synth.mixer.GetMasterMute() {
		return zeroFill(samples), true
	}

	// Read from output channels
	buffers := make([][]float64, NumOutputChannels)
	for ch := range NumOutputChannels {
		channelName := OutputChannelName(ch)
		buffers[ch] = s.synth.mixer.ReadChannel(channelName, len(samples))
	}

	// Get combined gains and mute states for all channels
	outputChannels := s.getOutputChannels()

	// Fill output from master channel buffers, applying gain and mute
	// Calibration signal is generated in MixToMaster() with proper phase tracking
	for chanelIndex := range buffers {
		buf := buffers[chanelIndex]
		gain := outputChannels[chanelIndex].Gain
		mute := outputChannels[chanelIndex].Mute

		for sampleIndex := range samples {
			if buf != nil && sampleIndex < len(buf) {
				if mute {
					samples[sampleIndex][chanelIndex] = 0
				} else {
					samples[sampleIndex][chanelIndex] = buf[sampleIndex] * gain
				}
			} else {
				samples[sampleIndex][chanelIndex] = 0
			}
		}
	}

	return len(samples), true
}

func (s *Streamer) Err() error {
	return nil
}

func (b *HapticStream) Stream(samples [][2]float64) (n int, ok bool) {
	// Stream the resampled audio using the persistent resampler
	return b.streamer.Stream(samples)
}

func (b *HapticStream) Err() error {
	return b.streamer.Err()
}

// getOutputChannels retrieves the combined gains and mute states for all channels.
// Note: Master mute is handled earlier in Stream() for efficiency.
func (s *Streamer) getOutputChannels() []OutputChannelSettings {
	masterGainDB, _ := s.synth.mixer.GetChannelGain(ChannelMaster)

	channelGains := make([]OutputChannelSettings, NumOutputChannels)

	for ch := range NumOutputChannels {
		channelName := OutputChannelName(ch)
		channelGainDB, _ := s.synth.mixer.GetChannelGain(channelName)
		channelGains[ch] = OutputChannelSettings{
			Gain: GainToPowerRatio(channelGainDB + masterGainDB),
			Mute: s.synth.GetChannelMute(ch),
		}
	}

	return channelGains
}

// zeroFill fills all samples with zeros and returns the sample count.
func zeroFill(samples [][2]float64) int {
	for i := range samples {
		samples[i][0] = 0
		samples[i][1] = 0
	}

	return len(samples)
}

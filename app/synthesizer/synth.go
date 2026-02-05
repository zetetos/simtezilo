package synthesizer

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/zetetos/simtezilo/app/calibrator"
	"github.com/zetetos/simtezilo/app/codec"
	"github.com/zetetos/simtezilo/app/config"
	"github.com/zetetos/simtezilo/app/kinematics"
)

// Mixer channel names.
const (
	ChannelMaster       = "_master"
	ChannelEngine       = "engine"
	ChannelTransmission = "transmission"
	ChannelCalibrator   = "calibration"

	// NumOutputChannels defines the number of output channels (stereo = 2).
	NumOutputChannels = 2

	// Channel name prefixes for pattern matching.
	chassisChannelPrefix = "chassis_"
	outputChannelPrefix  = "_output_"
)

// OutputChannelName returns the channel name for output channel n (e.g., "_output_0").
func OutputChannelName(n int) string {
	return fmt.Sprintf("%s%d", outputChannelPrefix, n)
}

// ChassisChannelName returns the channel name for chassis channel n (e.g., "chassis_0").
func ChassisChannelName(n int) string {
	return fmt.Sprintf("%s%d", chassisChannelPrefix, n)
}

// IsChassisChannel returns true if the channel name is a chassis channel.
func IsChassisChannel(name string) bool {
	return len(name) > len(chassisChannelPrefix) && name[:len(chassisChannelPrefix)] == chassisChannelPrefix
}

// IsOutputChannel returns true if the channel name is an output channel.
func IsOutputChannel(name string) bool {
	return len(name) > len(outputChannelPrefix) && name[:len(outputChannelPrefix)] == outputChannelPrefix
}

// ParseOutputChannelIndex extracts the channel index from an output channel name.
// Returns -1 if the name is not a valid output channel.
func ParseOutputChannelIndex(name string) int {
	if !IsOutputChannel(name) {
		return -1
	}

	var index int

	_, err := fmt.Sscanf(name, outputChannelPrefix+"%d", &index)
	if err != nil {
		return -1
	}

	return index
}

// Synthesizer is the main synthesizer structure that holds the mixer, effects, and output device.
type Synthesizer struct {
	effects      *EffectsSampleBank
	log          zerolog.Logger
	mixer        Mixer
	outputDevice *OutputDevice
	kinematics   *kinematics.State
	sampleRate   int
	outFile      *os.File

	// Calibration mode state
	calibrator         calibrator.Calibrator
	wasCalibrating     bool
	originalMasterGain float64
}

// SynthOpts holds the options for creating a new Synthesizer.
type SynthOpts struct {
	Config     *config.Synthesizer
	BaseConfig *config.Config // Base config for lock-free reads in mixer
	Logger     zerolog.Logger
	Kinematics *kinematics.State
	Calibrator calibrator.Calibrator
	Mixer      Mixer
}

// New creates a new Synthesizer instance with the provided options.
func New(opts *SynthOpts) (*Synthesizer, error) {
	synthesizer := &Synthesizer{
		effects:            NewEffectsSampleBank(),
		kinematics:         opts.Kinematics,
		sampleRate:         opts.Config.InternalSampleRateHz,
		log:                opts.Logger.With().Str("package", "synth").Logger(),
		calibrator:         opts.Calibrator,
		mixer:              opts.Mixer,
		wasCalibrating:     false,
		originalMasterGain: 0,
	}

	var err error

	bufferLength := 2 * time.Second

	if synthesizer.mixer == nil {
		// Pass full config for lock-free reads
		synthesizer.mixer, err = NewStereoMixer(StereoMixerConfig{
			Config:       opts.BaseConfig,
			Calibrator:   opts.Calibrator,
			BufferLength: bufferLength,
			SampleRateHz: opts.Config.InternalSampleRateHz,
			Log:          opts.Logger.With().Str("package", "synth mixer").Logger(),
		})
		if err != nil {
			return nil, fmt.Errorf("create mixer: %w", err)
		}

		// Add channels with initial values from config
		_ = synthesizer.mixer.AddChannel(ChannelTransmission, opts.Config.TransmissionGain)
		_ = synthesizer.mixer.AddChannel(ChannelEngine, opts.Config.EngineGain)
		_ = synthesizer.mixer.AddChannel(ChannelCalibrator, 0)

		// Add per-channel chassis buffers for per-channel EQ support
		for ch := range NumOutputChannels {
			_ = synthesizer.mixer.AddChannel(ChassisChannelName(ch), opts.Config.ChassisGain)
		}
	}

	synthesizer.outputDevice, err = NewOutputDevice(SynthOutDeviceOpts{
		Log: opts.Logger.With().Str("package", "synth output device").Logger(),
	})
	if err != nil {
		return nil, err
	}

	if opts.Config.OutputFile != "" {
		synthesizer.outFile, err = os.Create(opts.Config.OutputFile)
		if err != nil {
			return nil, fmt.Errorf("create output wav file: %w", err)
		}

		log.Info().Str("file", opts.Config.OutputFile).Msg("saving audio output")
	}

	return synthesizer, nil
}

// Close gracefully shuts down the synthesizer, closing the mixer and output file if applicable.
func (s *Synthesizer) Close() (err error) {
	s.mixer.Close()

	if s.outFile != nil {
		err = s.outFile.Close()
	}

	return err
}

// GetSampleRate returns the internal sample rate of the synthesizer.
func (s *Synthesizer) GetSampleRate() int {
	return s.sampleRate
}

// GetBufferCapacity returns the buffer capacity of the mixer in samples.
func (s *Synthesizer) GetBufferCapacity() int {
	return s.mixer.GetBufferCapacity()
}

// InspectChannelBuffer returns a copy of the specified channel buffer for inspection.
func (s *Synthesizer) InspectChannelBuffer(name string, length int, offset int) []float64 {
	return s.mixer.InspectChannelBuffer(name, length, offset)
}

// ReadBuffer mixes all channels to the master and returns the mixed sample data.
func (s *Synthesizer) ReadBuffer(length int) []float64 {
	s.mixer.MixToMaster(length)

	return s.mixer.ReadChannel(ChannelMaster, length)
}

// GetChannelMute returns the mute state for the specified channel index.
func (s *Synthesizer) GetChannelMute(channel int) bool {
	return s.mixer.GetChannelMute(channel)
}

// WriteBuffer writes the provided sample data to the specified channel buffer at the given offset.
func (s *Synthesizer) WriteBuffer(channel string, sample []float64, offset int) {
	magnitude, err := s.mixer.GetChannelPowerRatio(channel)
	if err != nil {
		s.log.Error().Err(err).Str("channel", channel).Msg("failed to get channel power ratio")

		return
	}

	_ = s.mixer.WriteChannel(channel, sample, magnitude, offset, false)
}

// OverwriteBuffer overwrites the specified channel buffer with the provided sample data at the given offset.
func (s *Synthesizer) OverwriteBuffer(channel string, sample []float64, offset int) {
	magnitude, err := s.mixer.GetChannelPowerRatio(channel)
	if err != nil {
		s.log.Error().Err(err).Str("channel", channel).Msg("failed to get channel power ratio")

		return
	}

	_ = s.mixer.WriteChannel(channel, sample, magnitude, offset, true)
}

// GetChannelMagnitude returns the current magnitude (gain) of the specified channel.
func (s *Synthesizer) GetChannelMagnitude(name string) (float64, error) {
	return s.mixer.GetChannelPowerRatio(name)
}

// FadeIn gradually increases the master gain from minimum to the configured level over the specified period.
func (s *Synthesizer) FadeIn(period time.Duration) {
	s.mixer.FadeIn(period)
}

// ApplyMasterGain applies the current master gain to the provided value and returns the adjusted value.
func (s *Synthesizer) ApplyMasterGain(value float64) float64 {
	outputGain, _ := s.mixer.GetChannelPowerRatio(ChannelMaster)

	return value * outputGain
}

// Silence immediately silences all mixeroutput and clears buffers.
func (s *Synthesizer) Silence() {
	s.mixer.SetFader(config.MinimumGain)
	s.mixer.SetSilenced(true)

	s.mixer.ClearBuffers()
}

// EffectSampleBank returns the effects sample bank.
func (s *Synthesizer) EffectSampleBank() *EffectsSampleBank {
	return s.effects
}

// GetEffectSample returns the raw sample data for the specified effect name.
func (s *Synthesizer) GetEffectSample(name string, sampleRate int) codec.PCMFloat64 {
	effectSample := s.effects.GetSample(name, sampleRate)

	return effectSample
}

// PlayEffect plays an effect at a given magnitude on a given channel.
func (s *Synthesizer) PlayEffect(name string, magnitude float64, channel string) {
	channelMagnitude, err := s.mixer.GetChannelPowerRatio(channel)
	if err != nil {
		s.log.Error().Err(err).Str("channel", name).Msg("failed to get channel power ratio")

		return
	}

	magnitude *= channelMagnitude

	effectSample := s.effects.GetSample(name, s.sampleRate)

	_ = s.mixer.WriteChannel(channel, effectSample.Samples(), magnitude, 0, false)
}

// StartFadeIn initiates a fade-in of the master gain over the specified duration.
func (s *Synthesizer) StartFadeIn(duration time.Duration) {
	s.mixer.FadeIn(duration)
}

// UpdateCalibrator checks calibration state and manages channel switching.
func (s *Synthesizer) UpdateCalibrator() {
	calibratorEnabled := s.calibrator.IsEnabled()
	calibratorStopping := s.calibrator.IsStopping()

	// Handle calibrator state transitions
	if calibratorEnabled && !s.wasCalibrating {
		// Entering calibration mode - flush all haptic buffers first
		s.startCalibrator()
	} else if !calibratorEnabled && !calibratorStopping && s.wasCalibrating {
		// Exiting calibration mode - zero crossing has been reached
		s.stopCalibrator()
	}
}

func (s *Synthesizer) startCalibrator() {
	s.log.Info().Msg("Entering calibration mode")
	s.wasCalibrating = true

	// Clear all haptic channel buffers first to prevent volume spike
	for ch := range NumOutputChannels {
		s.mixer.ClearChannelBuffer(ChassisChannelName(ch))
	}

	s.mixer.ClearChannelBuffer(ChannelEngine)
	s.mixer.ClearChannelBuffer(ChannelTransmission)
	s.log.Debug().Msg("Flushed haptic channel buffers")

	// Clear calibrator buffer to start fresh
	s.mixer.ClearChannelBuffer(ChannelCalibrator)
	s.log.Debug().Msg("Cleared calibrator buffer")

	// Reset sine wave phase to start from zero
	s.mixer.ResetSineWavePhase()

	// Store original master gain for restoration after calibration
	currentGain, err := s.mixer.GetChannelGain(ChannelMaster)
	if err == nil {
		s.originalMasterGain = currentGain

		// Clear master buffer to remove any previously mixed audio
		s.mixer.ClearChannelBuffer(ChannelMaster)
		s.log.Debug().Msg("Flushed master channel buffer")
	}
}

func (s *Synthesizer) stopCalibrator() {
	s.log.Info().Msg("Exiting calibration mode after zero crossing")
	s.wasCalibrating = false

	// Add a small delay to allow the sine wave to complete naturally without cutting
	// off the tail end of the waveform
	time.Sleep(50 * time.Millisecond)

	// Clear the calibrator channel buffer
	s.mixer.ClearChannelBuffer(ChannelCalibrator)

	// Clear master buffer to remove any calibration audio mixed at calibration volume
	s.mixer.ClearChannelBuffer(ChannelMaster)
	s.log.Debug().Msg("Cleared master buffer")

	// Clear all haptic channel buffers to ensure clean start
	for ch := range NumOutputChannels {
		s.mixer.ClearChannelBuffer(ChassisChannelName(ch))
	}

	s.mixer.ClearChannelBuffer(ChannelEngine)
	s.mixer.ClearChannelBuffer(ChannelTransmission)
	s.log.Debug().Msg("Cleared haptic channel buffers")

	// Start fade-in from minimum to original gain over 100ms to prevent blip
	// Set master to minimum first
	_ = s.mixer.SetChannelGain(ChannelMaster, config.MinimumGain)
	// Fade to original gain
	s.mixer.FadeIn(100 * time.Millisecond)
	s.log.Debug().Float64("target_gain", s.originalMasterGain).Msg("Started fade-in after calibration")
}

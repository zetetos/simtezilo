package synthesizer

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/zetetos/simtezilo/app/calibrator"
	"github.com/zetetos/simtezilo/app/config"
	"github.com/zetetos/simtezilo/app/signal"
)

// StereoMixer handles two audio channels, mixing them into a master output channel.
type StereoMixer struct {
	config *config.Config

	channels     map[string]*MixerChannel
	bufferLength time.Duration
	sampleRateHz int
	log          zerolog.Logger
	faderGain    float64
	fadeInActive bool
	silenced     bool

	// Calibration mode state
	calibrator     calibrator.Calibrator
	sineWavePhaseL float64
	sineWavePhaseR float64

	// Buffer monitoring
	lastHealthCheck     time.Time
	healthCheckInterval time.Duration

	// Lifecycle management
	ctx    context.Context //nolint:containedctx // Context for managing lifecycle
	cancel context.CancelFunc

	mu sync.RWMutex
}

// MixerChannel represents an individual audio channel within the mixer.
type MixerChannel struct {
	activeGain float64
	buffer     Buffer
}

// StereoMixerConfig holds configuration options for the Mixer.
type StereoMixerConfig struct {
	Config       *config.Config        // Full config reference for lock-free reads
	Calibrator   calibrator.Calibrator // Calibration mode signal manager
	BufferLength time.Duration         // Duration of audio the buffer should hold
	SampleRateHz int                   // Sample rate in Hz
	Log          zerolog.Logger        // Logger instance for logging
}

// NewStereoMixer creates a new Mixer instance with the provided configuration.
func NewStereoMixer(mixerConfig StereoMixerConfig) (*StereoMixer, error) {
	if mixerConfig.Config == nil {
		return nil, errors.New("config must be a valid pointer")
	}

	ctx, cancel := context.WithCancel(context.Background())

	mixer := &StereoMixer{
		config: mixerConfig.Config,

		bufferLength:   mixerConfig.BufferLength,
		sampleRateHz:   mixerConfig.SampleRateHz,
		channels:       map[string]*MixerChannel{},
		log:            mixerConfig.Log,
		faderGain:      config.MinimumGain,
		fadeInActive:   false,
		silenced:       true,
		calibrator:     mixerConfig.Calibrator,
		sineWavePhaseL: 0,
		sineWavePhaseR: 0,

		// Initialize buffer monitoring
		lastHealthCheck:     time.Now(),
		healthCheckInterval: 5 * time.Second,

		// Lifecycle management
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize master with lock-free config read
	masterGain := mixer.config.GetSynthMasterGain()

	err := mixer.AddChannel(ChannelMaster, masterGain)
	if err != nil {
		return nil, fmt.Errorf("add master channel: %w", err)
	}

	// Initialize per-channel output channels
	for ch := range NumOutputChannels {
		channelGain := mixer.config.GetSynthChannelGain(ch)

		err = mixer.AddChannel(OutputChannelName(ch), channelGain)
		if err != nil {
			return nil, fmt.Errorf("add output channel %d: %w", ch, err)
		}
	}

	go mixer.watchForConfigChanges()

	return mixer, nil
}

// Close gracefully shuts down the mixer, silencing output.
func (m *StereoMixer) Close() {
	_ = m.SetChannelGain(ChannelMaster, config.MinimumGain)

	// Cancel context to stop background goroutines
	if m.cancel != nil {
		m.cancel()
	}
}

// GetBufferCapacity returns the configured buffer length duration in samples.
func (m *StereoMixer) GetBufferCapacity() int {
	return int(m.bufferLength.Seconds() * float64(m.sampleRateHz))
}

// AddChannel adds a new channel to the mixer with the specified name and initial gain.
func (m *StereoMixer) AddChannel(name string, initialGain float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.channels[name]; ok {
		return fmt.Errorf("channel %q already exists", name)
	}

	m.channels[name] = &MixerChannel{
		activeGain: initialGain,
		buffer:     NewAdaptiveBuffer(m.bufferLength, m.sampleRateHz),
	}

	return nil
}

// Read reads the specified number of samples from the channel's buffer.
// All samples read are removed from the buffer.
func (m *MixerChannel) Read(length int) []float64 {
	return m.buffer.Read(length)
}

// Write writes samples to the channel's buffer with the specified magnitude and offset.
func (m *MixerChannel) Write(samples []float64, magnitude float64, offset int, overwrite bool) {
	ScaleSamples(&samples, magnitude)

	m.buffer.Write(samples, offset, overwrite)
}

// WriteChannel writes the provided sample data to the specified channel buffer at the given offset.
func (m *StereoMixer) WriteChannel(name string, samples []float64, magnitude float64, offset int, overwrite bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.channels[name]; !ok {
		return fmt.Errorf("channel not found: %q", name)
	}

	m.channels[name].Write(samples, magnitude, offset, overwrite)

	return nil
}

// ReadChannel reads the specified number of samples from the channel's buffer.
func (m *StereoMixer) ReadChannel(name string, length int) []float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, ok := m.channels[name]
	if !ok {
		return nil
	}

	// Check if channel is muted using lock-free config reads
	var muted bool

	switch {
	case name == ChannelMaster:
		muted = m.config.GetSynthMasterMute()
	case IsChassisChannel(name):
		muted = m.config.GetSynthChassisMute()
	case name == ChannelTransmission:
		muted = m.config.GetSynthTransmissionMute()
	case name == ChannelEngine:
		muted = m.config.GetSynthEngineMute()
	case name == ChannelCalibrator:
		muted = false
	}

	if muted {
		// Return silence for muted channels
		return make([]float64, length)
	}

	// Check buffer health periodically
	m.checkBufferHealth()

	return channel.Read(length)
}

// InspectChannelBuffer returns a copy of the specified channel buffer for inspection.
func (m *StereoMixer) InspectChannelBuffer(name string, length int, offset int) []float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if channel, ok := m.channels[name]; ok {
		return channel.buffer.Inspect(length, offset)
	}

	return nil
}

// GetChannelBufferLength returns the current length of samples in the specified channel's buffer.
func (m *StereoMixer) GetChannelBufferLength(name string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if channel, ok := m.channels[name]; ok {
		return channel.buffer.Length()
	}

	return 0
}

// GetChannelNames returns a list of all channel names configured in the mixer.
func (m *StereoMixer) GetChannelNames() []string {
	names := []string{}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for name := range m.channels {
		// skip internal channels
		if name[0:1] == "_" {
			continue
		}

		names = append(names, name)
	}

	return names
}

// GetChannelGain returns the current gain of the specified channel.
func (m *StereoMixer) GetChannelGain(name string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, ok := m.channels[name]
	if !ok {
		return 0, fmt.Errorf("channel %q does not exist", name)
	}

	return channel.activeGain, nil
}

// SetChannelGain sets the gain of the specified channel.
func (m *StereoMixer) SetChannelGain(name string, gain float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel, ok := m.channels[name]
	if !ok {
		return fmt.Errorf("channel %q does not exist", name)
	}

	if channel.activeGain == gain {
		return nil
	}

	channel.activeGain = gain
	m.channels[name] = channel

	return nil
}

// GetChannelPowerRatio returns the current power ratio of the specified channel.
func (m *StereoMixer) GetChannelPowerRatio(name string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, ok := m.channels[name]
	if !ok {
		return 0, fmt.Errorf("channel %q does not exist", name)
	}

	return GainToPowerRatio(channel.activeGain), nil
}

// GetChannelAmplitudeRatio returns the current amplitude ratio of the specified channel.
func (m *StereoMixer) GetChannelAmplitudeRatio(name string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, ok := m.channels[name]
	if !ok {
		return 0, fmt.Errorf("channel %q does not exist", name)
	}

	return GainToAmplitudeRatio(channel.activeGain), nil
}

// SetFader sets the fader gain, which controls the overall output level.
func (m *StereoMixer) SetFader(gain float64) {
	m.mu.Lock()
	m.faderGain = gain
	m.mu.Unlock()

	_ = m.SetChannelGain(ChannelMaster, m.faderGain)
}

// FadeIn gradually increases the master gain from minimum to the configured level over the specified period.
func (m *StereoMixer) FadeIn(period time.Duration) {
	m.mu.RLock()
	master := m.channels[ChannelMaster]
	m.mu.RUnlock()

	// Lock-free config read
	targetGain := m.config.GetSynthMasterGain()

	if master.activeGain == targetGain || m.fadeInActive {
		return
	}

	go func() {
		m.silenced = false
		m.fadeInActive = true

		fadeInInterval := 50 * time.Millisecond
		incrementTime := float64(period.Milliseconds() / fadeInInterval.Milliseconds())
		fadeInIncrement := (targetGain - m.faderGain) / incrementTime

		m.log.Debug().
			Float64("current", m.faderGain).
			Float64("target", targetGain).
			Str("state", "begin").
			Msg("fade in")

		for {
			m.faderGain += fadeInIncrement

			// fade in complete
			if m.faderGain >= targetGain {
				m.mu.Lock()
				m.faderGain = targetGain
				m.mu.Unlock()
				_ = m.SetChannelGain(ChannelMaster, targetGain)

				break
			}

			_ = m.SetChannelGain(ChannelMaster, m.faderGain)

			time.Sleep(fadeInInterval)
		}

		m.fadeInActive = false

		m.mu.RLock()
		master := m.channels[ChannelMaster]
		m.mu.RUnlock()

		m.log.Debug().
			Float64("current", master.activeGain).
			Float64("target", targetGain).
			Str("state", "complete").
			Msg("fade in")
	}()
}

// MixToMaster mixes all active channels into the master channel buffer using an alternative algorithm.
func (m *StereoMixer) MixToMaster(length int) {
	outSamples := make([]float64, length)

	m.mu.RLock()

	// Check if calibration mode is enabled or stopping (waiting for zero crossing)
	if m.calibrator != nil && (m.calibrator.IsEnabled() || m.calibrator.IsStopping()) {
		frequency := m.calibrator.GetSweepFrequency() // Use sweep frequency if sweeping, otherwise static frequency
		isStopping := m.calibrator.IsStopping()

		// Get per-channel EQ amplitude multipliers
		eqAmplitudes := make([]float64, NumOutputChannels)
		for channelIndex := range NumOutputChannels {
			eqAmplitudes[channelIndex] = 1.0

			if m.config.GetSynthChannelEqEnabled(channelIndex) {
				curve, minFreq, resolution := m.config.GetSynthChannelEqCurve(channelIndex)
				if len(curve) > 0 {
					// Calculate bucket index for this frequency
					index := int((frequency - minFreq) / resolution)
					// Apply EQ if frequency is within range
					if index >= 0 && index < len(curve) {
						eqAmplitudes[channelIndex] = curve[index]
					}
				}
			}
		}

		// Create per-channel output buffers
		channelSamples := make([][]float64, NumOutputChannels)
		for ch := range NumOutputChannels {
			channelSamples[ch] = make([]float64, length)
		}

		var prevPhase float64

		// Generate sine wave samples
		for offset := range outSamples {
			prevPhase = m.sineWavePhaseL

			// Generate base sine wave (mono source)
			baseSample := math.Sin(m.sineWavePhaseL)
			outSamples[offset] = baseSample // Master gets unmodified

			// Apply per-channel EQ
			for ch := range NumOutputChannels {
				channelSamples[ch][offset] = baseSample * eqAmplitudes[ch]
			}

			// Increment phase
			m.sineWavePhaseL += 2 * math.Pi * frequency / float64(m.sampleRateHz)

			// Keep phase in reasonable range
			if m.sineWavePhaseL > 2*math.Pi {
				m.sineWavePhaseL -= 2 * math.Pi
			}

			// If stopping, check for zero crossing
			if isStopping {
				// Detect zero crossing: previous phase was negative, current is positive
				// or we wrapped around through zero
				prevSin := math.Sin(prevPhase)
				currSin := math.Sin(m.sineWavePhaseL)

				if prevSin <= 0 && currSin >= 0 {
					// Zero crossing detected - stop here
					outSamples[offset] = 0 // End on zero to ensure clean stop
					for ch := range NumOutputChannels {
						channelSamples[ch][offset] = 0
					}

					m.mu.RUnlock()
					m.calibrator.ConfirmStopped()
					// Pad remaining samples with zeros
					for j := offset + 1; j < len(outSamples); j++ {
						outSamples[j] = 0
						for ch := range NumOutputChannels {
							channelSamples[ch][j] = 0
						}
					}

					// Write calibration output to all channels
					m.channels[ChannelMaster].Write(outSamples, 1.0, 0, true)

					for ch := range NumOutputChannels {
						m.channels[OutputChannelName(ch)].Write(channelSamples[ch], 1.0, 0, true)
					}

					return
				}
			}
		}

		m.mu.RUnlock()

		// Write calibration output to all channels
		m.channels[ChannelMaster].Write(outSamples, 1.0, 0, true)

		for ch := range NumOutputChannels {
			m.channels[OutputChannelName(ch)].Write(channelSamples[ch], 1.0, 0, true)
		}

		return
	}

	// Normal haptic mode - mix per-channel chassis with transmission and engine
	// Create separate output buffers for each channel to support per-channel EQ
	channelSamples := make([][]float64, NumOutputChannels)
	peaks := make([]float64, NumOutputChannels)

	for ch := range NumOutputChannels {
		channelSamples[ch] = make([]float64, length)
	}

	// Mix per-channel chassis with appropriate EQ
	chassisMuted := m.config.GetSynthChassisMute()
	if !chassisMuted {
		for ch := range NumOutputChannels {
			if chassisCh, ok := m.channels[ChassisChannelName(ch)]; ok {
				samples := chassisCh.Read(length)
				for i, sample := range samples {
					channelSamples[ch][i] = mixSampleSum(channelSamples[ch][i], sample, &peaks[ch])
				}
			}
		}
	}

	// Mix transmission into all outputs (shared channel)
	if transmissionChannel, ok := m.channels[ChannelTransmission]; ok {
		transmissionMuted := m.config.GetSynthTransmissionMute()
		if !transmissionMuted {
			samples := transmissionChannel.Read(length)
			for i, sample := range samples {
				for ch := range NumOutputChannels {
					channelSamples[ch][i] = mixSampleSum(channelSamples[ch][i], sample, &peaks[ch])
				}
			}
		}
	}

	// Scale peaks for each channel
	for ch := range NumOutputChannels {
		if peaks[ch] > 1.0 {
			scaleSamplesPeak(&channelSamples[ch], peaks[ch])
		}
	}

	// Mix engine channel into all outputs with lower priority
	m.mixEngineChannelMulti(channelSamples, length)

	m.mu.RUnlock()

	magnitude := 1.0

	// Mix output channels for master (average of all)
	masterSamples := make([]float64, length)
	for sampleIdx := range length {
		sum := 0.0
		for channel := range NumOutputChannels {
			sum += channelSamples[channel][sampleIdx]
		}

		masterSamples[sampleIdx] = sum / float64(NumOutputChannels)
	}

	// Write to master and per-channel outputs
	m.channels[ChannelMaster].Write(masterSamples, magnitude, 0, true)

	for channel := range NumOutputChannels {
		m.channels[OutputChannelName(channel)].Write(channelSamples[channel], magnitude, 0, true)
	}
}

// ClearBuffers clears all channel buffers in the mixer.
func (m *StereoMixer) ClearBuffers() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, channel := range m.channels {
		channel.buffer.Clear()
	}
}

// ClearChannelBuffer clears a specific channel's buffer.
func (m *StereoMixer) ClearChannelBuffer(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if channel, ok := m.channels[name]; ok {
		channel.buffer.Clear()
	}
}

// ResetSineWavePhase resets the sine wave phase to zero for the calibrator.
func (m *StereoMixer) ResetSineWavePhase() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sineWavePhaseL = 0
	m.sineWavePhaseR = 0
}

// GetChannelMute returns the mute state for the specified channel index.
func (m *StereoMixer) GetChannelMute(channel int) bool {
	return m.config.GetSynthChannelMute(channel)
}

// GetMasterMute returns the master mute state.
func (m *StereoMixer) GetMasterMute() bool {
	return m.config.GetSynthMasterMute()
}

// SetSilenced sets the silenced state of the mixer.
func (m *StereoMixer) SetSilenced(silenced bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.silenced = silenced
}

// mixEngineChannelMulti mixes the engine channel into all output samples with lower priority.
func (m *StereoMixer) mixEngineChannelMulti(outSamples [][]float64, length int) {
	channel, ok := m.channels[ChannelEngine]
	if !ok {
		m.log.Error().Str("channel", ChannelEngine).Msg("channel not found in mixer")

		return
	}

	// Lock-free config read for mute state
	if m.config.GetSynthEngineMute() {
		return
	}

	engineSamples := channel.Read(length)
	if len(engineSamples) == 0 {
		return
	}

	// Process engine for all output channels
	m.processEngineSamplesMulti(outSamples, engineSamples)
}

// processEngineSamplesMulti processes and mixes engine samples into all output channels.
func (m *StereoMixer) processEngineSamplesMulti(outSamples [][]float64, engineSamples []float64) {
	// Create work buffers for each channel
	outSamplesWork := make([][]float64, len(outSamples))
	for channel := range outSamples {
		outSamplesWork[channel] = make([]float64, len(outSamples[channel]))
	}

	for index, engineSample := range engineSamples {
		for channel := range outSamples {
			peak := 0.0
			engineScaled := engineSample
			engineMax := 1.0 - signal.Abs(outSamples[channel][index])

			if engineSample > engineMax || engineSample < -engineMax {
				engineScaled = engineMax * engineSample
			}

			outSamplesWork[channel][index] = mixSampleSum(outSamples[channel][index], engineScaled, &peak)
		}
	}

	for channel := range outSamples {
		copy(outSamples[channel], outSamplesWork[channel])
	}
}

// checkBufferHealth monitors buffer health and logs issues.
func (m *StereoMixer) checkBufferHealth() {
	now := time.Now()
	if now.Sub(m.lastHealthCheck) < m.healthCheckInterval {
		return
	}

	m.lastHealthCheck = now

	for name, channel := range m.channels {
		if name == ChannelMaster {
			continue
		}

		// Check if channel is muted using lock-free config reads
		var muted bool

		switch {
		case IsChassisChannel(name):
			muted = m.config.GetSynthChassisMute()
		case name == ChannelTransmission:
			muted = m.config.GetSynthTransmissionMute()
		case name == ChannelEngine:
			muted = m.config.GetSynthEngineMute()
		}

		if muted {
			continue
		}

		// Check if the buffer supports health monitoring
		if adaptiveBuffer, ok := channel.buffer.(*AdaptiveBuffer); ok {
			overflows, underruns, fillRatio := adaptiveBuffer.Health()

			if overflows > 0 || underruns > 0 || fillRatio > 0.9 || fillRatio < 0.1 {
				m.log.Debug().
					Bool("healthy", false).
					Str("channel", name).
					Int("overflows", overflows).
					Int("underruns", underruns).
					Float64("fillRatio", fillRatio).
					Msg("buffer")
			}
		}
	}
}

// watchForConfigChanges monitors configuration changes and applies them to the mixer channels.
func (m *StereoMixer) watchForConfigChanges() {
	m.log.Debug().Str("event", "start").Msg("config watch")

	// Track previous mute states to detect changes
	previousMuteStates := make(map[string]bool)

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			m.log.Debug().Str("event", "stop").Msg("config watch")

			return
		case <-ticker.C:
		}

		if m.fadeInActive || m.silenced {
			continue
		}

		m.mu.RLock()

		for name, channel := range m.channels {
			// Lock-free config reads
			var (
				configGain float64
				configMute bool
			)

			switch {
			case name == ChannelMaster:
				configGain = m.config.GetSynthMasterGain()
				configMute = m.config.GetSynthMasterMute()
			case IsChassisChannel(name):
				configGain = m.config.GetSynthChassisGain()
				configMute = m.config.GetSynthChassisMute()
			case name == ChannelTransmission:
				configGain = m.config.GetSynthTransmissionGain()
				configMute = m.config.GetSynthTransmissionMute()
			case name == ChannelEngine:
				configGain = m.config.GetSynthEngineGain()
				configMute = m.config.GetSynthEngineMute()
			case IsOutputChannel(name):
				chIndex := ParseOutputChannelIndex(name)
				if chIndex >= 0 {
					configGain = m.config.GetSynthChannelGain(chIndex)
					configMute = m.config.GetSynthChannelMute(chIndex)
				} else {
					continue
				}
			default:
				continue
			}

			// Check if mute state changed
			prevMute, existed := previousMuteStates[name]
			if !existed || prevMute != configMute {
				previousMuteStates[name] = configMute

				// Clear buffer immediately when channel is muted for instant response
				if configMute {
					m.mu.RUnlock()
					m.mu.Lock()
					channel.buffer.Clear()
					m.mu.Unlock()
					m.log.Debug().Str("channel", name).Bool("muted", configMute).Str("event", "mute").Msg("config watch")
					m.mu.RLock()
				}
			}

			// Check if gain changed
			if channel.activeGain == configGain {
				continue
			}

			m.mu.RUnlock()
			_ = m.SetChannelGain(name, configGain)

			if name == ChannelMaster {
				if m.faderGain != channel.activeGain {
					m.mu.Lock()
					m.faderGain = channel.activeGain
					m.mu.Unlock()
				}
			}

			m.log.Debug().Str("channel", name).Float64("gain", configGain).Str("event", "change").Msg("config watch")
			m.mu.RLock()
		}

		m.mu.RUnlock()
	}
}

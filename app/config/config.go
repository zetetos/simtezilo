// Package config provides configuration management for the application.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	appHaptics "github.com/zetetos/simtezilo/app/haptics"
	"github.com/zetetos/simtezilo/app/i18n"
)

type app struct {
	Language       string  `json:"language"`
	Accent         string  `json:"accent"`
	LogLevel       string  `json:"logLevel"`
	BaseDir        string  `json:"baseDir"`
	Update         *update `json:"update,omitempty"`
	VehicleDBFile  string  `json:"vehicleDBFile"` //nolint:tagliatelle // schema uses Go-style acronym
	EnabledWebUI   bool    `json:"enabledWebUI"`  //nolint:tagliatelle // schema uses Go-style acronym
	WebUIPort      int     `json:"webUIPort"`     //nolint:tagliatelle // schema uses Go-style acronym
	EnableDevTools bool    `json:"enableDevTools"`
}

type discord struct {
	Token          string `json:"token"`
	GuildID        string `json:"guildID"`        //nolint:tagliatelle // schema uses Go-style acronym
	ChannelID      string `json:"channelID"`      //nolint:tagliatelle // schema uses Go-style acronym
	VoiceChannelID string `json:"voiceChannelID"` //nolint:tagliatelle // schema uses Go-style acronym
}

type fuelMonitoring struct {
	Enabled                 bool    `json:"enabled"`
	PreWarnNotifyLaps       float64 `json:"preWarnNotifyLaps"`
	StrategyNotifyLaps      float64 `json:"strategyNotifyLaps"`
	RangeSafetyMarginLaps   float64 `json:"rangeSafetyMarginLaps"`
	RangeSafetyMarginMetres float64 `json:"rangeSafetyMarginMetres"`
}

type haptics struct {
	EnableReplay                 bool                                `json:"enableReplay"`
	DynamicTransmissionFeedback  bool                                `json:"dynamicTransmissionFeedback"`
	DynamicTransmissionCurve     int                                 `json:"dynamicTransmissionCurve"`
	DynamicTransmissionGforceMax float64                             `json:"dynamicTransmissionGforceMax"`
	JerkCurve                    int                                 `json:"jerkCurve"`
	JerkMax                      int                                 `json:"jerkMax"`
	_jerkScale                   float64                             `json:"-"`
	SnapCurve                    int                                 `json:"snapCurve"`
	SnapMax                      int                                 `json:"snapMax"`
	_snapScale                   float64                             `json:"-"`
	PulseMaxAmplitude            float64                             `json:"pulseMaxAmplitude"`
	PulseMaxFrequencyHz          float64                             `json:"pulseMaxFrequencyHz"`
	PulseMinFrequencyHz          float64                             `json:"pulseMinFrequencyHz"`
	_pulseWidthMax               float64                             `json:"-"`
	_pulseWidthMin               float64                             `json:"-"`
	EngineProfiles               map[string]appHaptics.EngineProfile `json:"engineProfiles,omitempty"`
	_engineProfile               *appHaptics.EngineProfile           `json:"-"`
	_engineProfileName           string                              `json:"-"`
}

type hardware struct {
	Model              string `json:"model"`
	DisplayOrientation int    `json:"displayOrientation"`
}

type notifications struct {
	EnableRaceProgress      bool    `json:"enabledRaceProgress"`
	RaceProgressMinLaps     int     `json:"raceProgressMinLaps"`
	RaceProgressIntervalPc  int     `json:"raceProgressIntervalPc"`
	EnableRaceLaps          bool    `json:"enabledRaceLaps"`
	RaceLapsIntervalLaps    int     `json:"raceLapsIntervalLaps"`
	RaceLapsCountdownLaps   int     `json:"raceLapsCountdownLaps"`
	EnableLapTimes          bool    `json:"enabledLapTimes"`
	LapTimesMaxDeltaSeconds float64 `json:"lapTimesMaxDeltaSeconds"`
	EnableCircuitMatching   bool    `json:"enabledCircuitMatching"`
}

type pitRadio struct {
	Enabled               bool            `json:"enabled"`
	Output                string          `json:"output"`
	MessageSendIntervalMs int             `json:"messageSendIntervalMs"`
	Notifications         *notifications  `json:"notifications,omitempty"`
	Discord               *discord        `json:"discord,omitempty"`
	FuelMonitoring        *fuelMonitoring `json:"fuelMonitoring,omitempty"`
	TyreMonitoring        *tyreMonitoring `json:"tyreMonitoring,omitempty"`
}

// EQBand represents a parametric equalizer band with center frequency, gain, and Q factor.
type EQBand struct {
	Frequency float64 `json:"frequency"` // Center frequency in Hz
	Gain      float64 `json:"gain"`      // Gain in dB (-12 to +6)
	Q         float64 `json:"q"`         // Q factor (0.1 to 20, higher = narrower)
}

// Synthesizer represents an audio synthesizer used for haptic feedback.
type Synthesizer struct {
	InternalSampleRateHz      int         `json:"internalSampleRateHz"`
	OutputSampleRateHz        int         `json:"outputSampleRateHz"`
	OutputFile                string      `json:"outputFile,omitempty"`
	MasterMute                bool        `json:"masterMute"`
	MasterGain                float64     `json:"masterGain"`
	ChannelMute               []bool      `json:"channelMute"`
	ChannelGain               []float64   `json:"channelGain"`
	ChassisMute               bool        `json:"chassisMute"`
	ChassisGain               float64     `json:"chassisGain"`
	TransmissionMute          bool        `json:"transmissionMute"`
	TransmissionGain          float64     `json:"transmissionGain"`
	TransmissionGainMinRace   float64     `json:"transmissionGainMinRace"`
	TransmissionGainMinStreet float64     `json:"transmissionGainMinStreet"`
	EngineMute                bool        `json:"engineMute"`
	EngineGain                float64     `json:"engineGain"`
	GainIncrement             float64     `json:"gainIncrement"`
	EnableEq                  []bool      `json:"enableEq"`
	EqBands                   [][]EQBand  `json:"eqBands,omitempty"`
	_eqCurve                  [][]float64 `json:"-"` // Computed curve for fast lookup (per channel)
	_eqMinFreq                float64     `json:"-"` // Minimum frequency for curve
	_eqMaxFreq                float64     `json:"-"` // Maximum frequency for curve
	_eqResolution             float64     `json:"-"` // Frequency resolution (Hz per bucket)}
}

// Telemetry represents the telemetry data source configuration.
type Telemetry struct {
	Source string `json:"source"`
}

// Status represents the status of the configuration.
type Status struct {
	LastUpdate      int64
	RestartRequired bool
}

type tyreMonitoring struct {
	Enabled                    bool    `json:"enabled"`
	TemperatureOptimalCelsius  float32 `json:"temperatureOptimalCelsius"`
	TemperatureOperatingWindow float32 `json:"temperatureOperatingWindow"`
	TemperatureMarginCelsius   float32 `json:"temperatureMarginCelsius"`
}

type update struct {
	BaseURL              string `json:"baseURL"` //nolint:tagliatelle // schema uses Go-style acronym
	Channel              string `json:"channel"`
	AutoCheck            bool   `json:"autoCheck"`
	AutoInstall          bool   `json:"autoInstall"`
	CheckIntervalMinutes int    `json:"checkIntervalMinutes"`
}

type viperConfig struct {
	Schema        string       `json:"$schema,omitempty"`
	SchemaVersion string       `json:"schemaVersion"`
	App           *app         `json:"app,omitempty"`
	Hardware      *hardware    `json:"hardware,omitempty"`
	Haptics       *haptics     `json:"haptics,omitempty"`
	PitRadio      *pitRadio    `json:"pitRadio,omitempty"`
	Synthesizer   *Synthesizer `json:"synthesizer,omitempty"`
	Telemetry     *Telemetry   `json:"telemetry,omitempty"`
}

// Snapshot holds frequently-accessed configuration values for lock-free reads.
type Snapshot struct {
	// Synthesizer gain settings
	MasterMute                bool
	MasterGain                float64
	ChannelMute               []bool
	ChannelGain               []float64
	ChassisMute               bool
	ChassisGain               float64
	TransmissionMute          bool
	TransmissionGain          float64
	TransmissionGainMinRace   float64
	TransmissionGainMinStreet float64
	EngineMute                bool
	EngineGain                float64
	GainIncrement             float64
	InternalSampleRateHz      int
	OutputSampleRateHz        int

	// Haptics jerk settings (chassis amplitude)
	JerkCurve float64
	JerkMax   int
	JerkScale float64

	// Haptics snap settings (chassis frequency)
	SnapCurve float64
	SnapMax   int
	SnapScale float64

	// Haptics pulse settings
	PulseMaxAmplitude   float64
	PulseMaxFrequencyHz float64
	PulseMinFrequencyHz float64
	PulseWidthMin       float64
	PulseWidthMax       float64

	// Dynamic transmission settings
	DynamicTransmissionFeedback  bool
	DynamicTransmissionCurve     int
	DynamicTransmissionGforceMax float64

	// EQ settings (per channel)
	EqEnabled []bool

	// Monitoring flags
	FuelMonitoringEnabled bool
	TyreMonitoringEnabled bool

	// Hardware settings
	DisplayOrientation int
}

// Config holds the application configuration and provides methods for accessing and modifying the data.
type Config struct {
	viper           *viperConfig
	snapshot        atomic.Pointer[Snapshot]
	i18n            *i18n.I18n
	configFile      string
	lastSavedConfig []byte // Last config written to disk (to avoid unnecessary writes)
	status          Status
	mu              sync.RWMutex
}

type Options struct {
	ConfigFile string
	Logger     zerolog.Logger
}

// New creates a new Config instance loading configuration from the specified filename.
func New(opts Options) *Config {
	config := &Config{
		viper:      defaultConfig(),
		configFile: opts.ConfigFile,
		status: Status{
			RestartRequired: false,
			LastUpdate:      0,
		},
	}

	viper.SetEnvPrefix("SIMTEZILO")
	viper.SetEnvKeyReplacer(strings.NewReplacer(`.`, `_`))
	viper.AutomaticEnv()
	viper.SetConfigType("json")

	if opts.ConfigFile != "" {
		opts.Logger.Debug().Str("filename", opts.ConfigFile).Msg("Loading config file")

		viper.SetConfigFile(opts.ConfigFile)
	} else {
		opts.Logger.Debug().Msg("No config file specified, searching default locations")

		viper.SetConfigName("simtezilo.conf")
		viper.AddConfigPath("/boot/firmware/simtezilo/")
		viper.AddConfigPath("/boot/simtezilo/")
		viper.AddConfigPath("/opt/simtezilo/etc/")
		viper.AddConfigPath("/opt/simtezilo/")
		viper.AddConfigPath(".")
	}

	err := viper.ReadInConfig()
	if err != nil {
		log.Error().
			Str("filename", viper.ConfigFileUsed()).
			Err(err).
			Msg("read config file")
	} else {
		err = viper.Unmarshal(config.viper)
		if err != nil {
			log.Error().Err(err).Msg("unmarshal config")
		}

		config.configFile = viper.ConfigFileUsed()
		log.Debug().Str("source", config.configFile).Msg("config loaded")

		// Initialize lastSavedConfig with current state to prevent false restart indicators
		jsonData, err := json.Marshal(config.viper)
		if err == nil {
			config.lastSavedConfig = jsonData
		}
	}

	// When config is loaded from defaults set a default config file for file save operations
	if config.configFile == "" {
		config.configFile = filepath.Join(".", "simtezilo.conf")
	}

	config.finalise()
	config.rebuildSnapshot()

	return config
}

// NewFromJSON creates a new Config instance loading configuration from the provided JSON byte slice.
func NewFromJSON(json []byte, log zerolog.Logger) *Config {
	config := &Config{
		viper: defaultConfig(),
	}

	vConf := viper.New()
	vConf.SetConfigType("json")

	err := vConf.ReadConfig(bytes.NewBuffer(json))
	if err != nil {
		log.Error().Err(err).Msg("read config file")
	} else {
		err = vConf.Unmarshal(config.viper)
		if err != nil {
			log.Error().Err(err).Msg("unmarshal config")
		}
	}

	configSource := "JSON string"

	log.Debug().Str("source", configSource).Msg("config loaded")

	config.finalise()
	config.rebuildSnapshot()

	return config
}

// SetDefault resets the configuration to the default values.
func (c *Config) SetDefault() {
	c.mu.Lock()

	// Try to load default config from <baseDir>/etc/default.conf
	baseDir := c.viper.App.BaseDir
	if baseDir == "" {
		baseDir = "."
	}

	defaultConfigPath := filepath.Join(baseDir, "etc", "default.conf")

	// Check if the file exists
	_, err := os.Stat(defaultConfigPath)
	if err == nil {
		// File exists, try to load it
		data, err := os.ReadFile(defaultConfigPath)
		if err == nil {
			// Create a new config structure
			newConfig := defaultConfig()

			err = json.Unmarshal(data, newConfig)
			if err == nil {
				c.viper = newConfig
				c.mu.Unlock()
				c.finalise()
				c.rebuildSnapshot()

				return
			}
			// If unmarshal failed, fall through to default
			log.Warn().Err(err).Str("file", defaultConfigPath).Msg("failed to unmarshal default config, using built-in defaults")
		} else {
			log.Warn().Err(err).Str("file", defaultConfigPath).Msg("failed to read default config, using built-in defaults")
		}
	}

	// Fall back to built-in defaults
	c.viper = defaultConfig()

	c.mu.Unlock()

	c.finalise()
	c.rebuildSnapshot()
}

// SetI18n sets the i18n instance for the Config which is required for interaction with language settings.
func (c *Config) SetI18n(i18n *i18n.I18n) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.i18n = i18n
}

// GetI18n returns the i18n instance.
func (c *Config) GetI18n() *i18n.I18n {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.i18n
}

// Status returns the current configuration status.
// The status includes Unix timestamp of the last change and whether a restart is required to apply the config.
func (c *Config) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.status
}

// IsUpToDate returns true if the configuration has changed since the given Unix timestamp.
func (c *Config) IsUpToDate(timestamp int64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return timestamp < c.status.LastUpdate
}

// RestartRequired returns true if a restart is required for configuration changes to take effect.
func (c *Config) RestartRequired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.status.RestartRequired
}

// MarkRestartRequired marks that a restart is required for configuration changes to take effect.
func (c *Config) MarkRestartRequired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status.RestartRequired = true
	c.status.LastUpdate = time.Now().Unix()
}

// ****************************************************************************
// App section methods.
// ****************************************************************************

// GetAppAccent returns the configured accent.
// If not set, it defaults to "us".
func (c *Config) GetAppAccent() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.viper.App.Accent == "" {
		return "us"
	}

	return c.viper.App.Accent
}

// SetAppAccent sets the application accent.
func (c *Config) SetAppAccent(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.App.Accent = value

	c.registerUpdate(false)
}

// GetAppBaseDir returns the configured base directory.
// If not set, it defaults to the current directory (".").
func (c *Config) GetAppBaseDir() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.viper.App.BaseDir == "" {
		return "."
	}

	return c.viper.App.BaseDir
}

// SetAppBaseDir sets the application base directory.
func (c *Config) SetAppBaseDir(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.App.BaseDir = value

	c.registerUpdate(true)
}

// GetAppVehicleDBFile returns the configured vehicle database file path.
func (c *Config) GetAppVehicleDBFile() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.App.VehicleDBFile
}

// SetAppVehicleDBFile sets the vehicle database file path.
func (c *Config) SetAppVehicleDBFile(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.App.VehicleDBFile = value

	c.registerUpdate(true)
}

// GetAppLanguage returns the configured application language.
// If not set, it defaults to "en".
func (c *Config) GetAppLanguage() *string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.viper.App.Language == "" {
		c.viper.App.Language = "en"
	}

	return &c.viper.App.Language
}

// SetAppLanguage sets the application language.
func (c *Config) SetAppLanguage(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// TODO: validate language code

	c.viper.App.Language = value

	c.registerUpdate(false)
}

// NextAppLanguage cycles to the next available language.
// It returns the language code of the selected language.
func (c *Config) NextAppLanguage() string {
	if c.i18n == nil {
		log.Warn().Msg("i18n instance not set in config")

		return c.viper.App.Language
	}

	languageCodes := c.i18n.LanguageCodes()

	var language string

	for i, lang := range languageCodes {
		if lang == c.viper.App.Language {
			nextIndex := (i + 1) % len(languageCodes)
			language = languageCodes[nextIndex]

			break
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if language != "" {
		c.viper.App.Language = language
	} else {
		c.viper.App.Language = "en"
	}

	c.registerUpdate(false)

	return c.viper.App.Language
}

// PreviousAppLanguage cycles to the previous available language.
// It returns the language code of the selected language.
func (c *Config) PreviousAppLanguage() string {
	if c.i18n == nil {
		log.Warn().Msg("i18n instance not set in config")

		return c.viper.App.Language
	}

	languageCodes := c.i18n.LanguageCodes()

	var language string

	for i, lang := range languageCodes {
		if lang == c.viper.App.Language {
			prevIndex := (i - 1 + len(languageCodes)) % len(languageCodes)
			language = languageCodes[prevIndex]

			break
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if language != "" {
		c.viper.App.Language = language
	} else {
		c.viper.App.Language = "en"
	}

	c.registerUpdate(false)

	return c.viper.App.Language
}

// GetAppLogLevel returns the configured log level.
// If not set, it defaults to "info".
func (c *Config) GetAppLogLevel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.viper.App.LogLevel == "" {
		return "info"
	}

	return c.viper.App.LogLevel
}

// SetAppLogLevel sets the application log level.
func (c *Config) SetAppLogLevel(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// TODO: validate log level

	c.viper.App.LogLevel = value

	c.registerUpdate(true)
}

// GetAppWebUIEnabled returns true if the web UI is enabled.
func (c *Config) GetAppWebUIEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.App.EnabledWebUI
}

// GetAppWebUIPort returns the configured web UI port.
// If not set, it defaults to 8080.
func (c *Config) GetAppWebUIPort() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.viper.App.WebUIPort == 0 {
		return 8080
	}

	return c.viper.App.WebUIPort
}

// GetAppUpdateAutoCheck returns whether automatic update checking is enabled.
func (c *Config) GetAppUpdateAutoCheck() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.App.Update.AutoCheck
}

// SetAppUpdateAutoCheck sets whether automatic update checking is enabled.
func (c *Config) SetAppUpdateAutoCheck(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.App.Update.AutoCheck = enabled

	c.registerUpdate(false)
}

// GetAppUpdateAutoInstall returns whether updates should be automatically installed.
func (c *Config) GetAppUpdateAutoInstall() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.App.Update.AutoInstall
}

// SetAppUpdateAutoInstall sets whether updates should be automatically installed.
func (c *Config) SetAppUpdateAutoInstall(autoInstall bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.App.Update.AutoInstall = autoInstall

	c.registerUpdate(false)
}

// GetAppUpdateCheckIntervalMinutes returns the update check interval in minutes.
func (c *Config) GetAppUpdateCheckIntervalMinutes() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.App.Update.CheckIntervalMinutes
}

// SetAppUpdateCheckIntervalMinutes sets the update check interval in minutes.
func (c *Config) SetAppUpdateCheckIntervalMinutes(minutes int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.App.Update.CheckIntervalMinutes = minutes

	c.registerUpdate(false)
}

// GetAppUpdateBaseURL returns the URL of the update manifest.
func (c *Config) GetAppUpdateBaseURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.App.Update.BaseURL
}

// GetAppUpdateChannel returns the update channel (e.g., "stable", "beta").
func (c *Config) GetAppUpdateChannel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.App.Update.Channel
}

// SetAppUpdateChannel sets the update channel (e.g., "stable", "beta", "dev").
func (c *Config) SetAppUpdateChannel(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.App.Update.Channel = channel

	c.registerUpdate(false)
}

// GetDevToolsEnabled returns true if developer tools are enabled.
func (c *Config) GetDevToolsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.App.EnableDevTools
}

// SetDevToolsEnabled sets whether developer tools are enabled.
func (c *Config) SetDevToolsEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.App.EnableDevTools = enabled

	c.registerUpdate(false)
}

// ****************************************************************************
// Hardware section methods.
// ****************************************************************************

// GetHardwareModel returns the configured hardware model.
func (c *Config) GetHardwareModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.Hardware.Model
}

// SetHardwareModel sets the hardware model.
func (c *Config) SetHardwareModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.Hardware.Model = model

	c.registerUpdate(true)
}

// GetDisplayOrientation returns the configured display orientation in degrees.
// Valid orientaitions are 0, 90, 180, and 270 degrees.
// Uses lock-free atomic read from snapshot.
func (c *Config) GetDisplayOrientation() int {
	return c.snapshot.Load().DisplayOrientation
}

// SetDisplayOrientation sets the display orientation in degrees.
// Valid values are 0, 90, 180, and 270 degrees.
func (c *Config) SetDisplayOrientation(orientation int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Normalize the orientation to valid values (0, 90, 180, 270)
	orientation %= 360
	if orientation < 0 {
		orientation += 360
	}

	// Round to nearest 90-degree increment
	orientation = (orientation + 45) / 90 * 90
	if orientation == 360 {
		orientation = 0
	}

	c.viper.Hardware.DisplayOrientation = orientation
	c.rebuildSnapshot()
	c.registerUpdate(false)
}

// ****************************************************************************
// Haptics section methods.
// ****************************************************************************

// GethapticsDynamicTransFeedbackEnabled returns true if dynamic transmission feedback is enabled.
func (c *Config) GethapticsDynamicTransFeedbackEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.Haptics.DynamicTransmissionFeedback
}

// SetHapticsDynamicTransFeedbackEnabled sets whether dynamic transmission feedback is enabled.
func (c *Config) SetHapticsDynamicTransFeedbackEnabled(value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.Haptics.DynamicTransmissionFeedback = value

	c.registerUpdate(false)
}

// GethapticsJerkCurve returns the jerk curve value.
// Values closer to 0 produce a more linear response.
// Values closer to 1 produce a more exponential response.
func (c *Config) GethapticsJerkCurve() float64 {
	return c.snapshot.Load().JerkCurve
}

// SetHapticsJerkCurve sets the jerk curve value.
// Values closer to 0 produce a more linear response.
// Values closer to 1 produce a more exponential response.
func (c *Config) SetHapticsJerkCurve(value int) {
	value = min(value, 955)
	value = max(value, 5)

	c.mu.Lock()
	c.viper.Haptics.JerkCurve = value
	c.mu.Unlock()
	c.updateJerkScale()
}

// IncreaseHapticsJerkCurve increases the jerk curve value in increments of 5.
func (c *Config) IncreaseHapticsJerkCurve() int {
	c.mu.Lock()
	c.viper.Haptics.JerkCurve = min(955, c.viper.Haptics.JerkCurve+5)
	result := c.viper.Haptics.JerkCurve
	c.mu.Unlock()
	c.updateJerkScale()

	return result
}

// DecreaseHapticsJerkCurve decreases the jerk curve value in increments of 5.
func (c *Config) DecreaseHapticsJerkCurve() int {
	c.mu.Lock()
	c.viper.Haptics.JerkCurve = max(5, c.viper.Haptics.JerkCurve-5)
	result := c.viper.Haptics.JerkCurve
	c.mu.Unlock()
	c.updateJerkScale()

	return result
}

// GetHapticsJerkScale returns the current jerk scale factor.
func (c *Config) GetHapticsJerkScale() float64 {
	return c.snapshot.Load().JerkScale
}

// GetHapticsJerkMax returns the maximum jerk value.
// The jerk curve is applied over the range from 0 to this maximum value.
// Any jerk vakues above this value are clamped to this maximum.
func (c *Config) GetHapticsJerkMax() int {
	return c.snapshot.Load().JerkMax
}

// SetHapticsJerkMax sets the maximum jerk value.
// The jerk curve is applied over the range from 0 to this maximum value.
// Any jerk vakues above this value are clamped to this maximum.
func (c *Config) SetHapticsJerkMax(value int) {
	value = min(value, 200)
	value = max(value, 1)

	c.mu.Lock()
	c.viper.Haptics.JerkMax = value
	c.mu.Unlock()
	c.updateJerkScale()
}

// IncreaseHapticsJerkMax increases the maximum jerk value in increments of 1.
func (c *Config) IncreaseHapticsJerkMax() int {
	c.mu.Lock()
	c.viper.Haptics.JerkMax = min(100, c.viper.Haptics.JerkMax+1)
	result := c.viper.Haptics.JerkMax
	c.mu.Unlock()
	c.updateJerkScale()

	return result
}

// DecreaseHapticsJerkMax decreases the maximum jerk value in increments of 1.
func (c *Config) DecreaseHapticsJerkMax() int {
	c.mu.Lock()
	c.viper.Haptics.JerkMax = max(1, c.viper.Haptics.JerkMax-1)
	result := c.viper.Haptics.JerkMax
	c.mu.Unlock()
	c.updateJerkScale()

	return result
}

// GetHapticsReplayEnabled returns true if replay mode is enabled.
func (c *Config) GetHapticsReplayEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.Haptics.EnableReplay
}

// SetHapticsEnableReplay sets whether haptics are generated for replays.
func (c *Config) SetHapticsEnableReplay(value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.Haptics.EnableReplay = value

	c.registerUpdate(true)
}

// GetHapticsSnapCurve returns the snap curve value.
func (c *Config) GetHapticsSnapCurve() float64 {
	return c.snapshot.Load().SnapCurve
}

// SetHapticsSnapCurve sets the snap curve value.
// Values closer to 0 produce a more linear response.
// Values closer to 1 produce a more exponential response.
func (c *Config) SetHapticsSnapCurve(value int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	value = min(value, 955)
	value = max(value, 5)

	c.viper.Haptics.SnapCurve = value

	c.registerUpdate(false)
}

// IncreaseHapticsSnapCurve increases the snap curve value in increments of 5.
func (c *Config) IncreaseHapticsSnapCurve() int {
	c.mu.Lock()

	c.viper.Haptics.SnapCurve = min(
		955,
		c.viper.Haptics.SnapCurve+5,
	)

	c.mu.Unlock()

	c.updateSnapScale()

	return c.viper.Haptics.SnapCurve
}

// DecreaseHapticsSnapCurve decreases the snap curve value in increments of 5.
func (c *Config) DecreaseHapticsSnapCurve() int {
	c.mu.Lock()

	if c.viper.Haptics.SnapCurve >= 10 {
		c.viper.Haptics.SnapCurve -= 5
	} else {
		c.viper.Haptics.SnapCurve = 5
	}

	c.mu.Unlock()

	c.updateSnapScale()

	return c.viper.Haptics.SnapCurve
}

// GetHapticsSnapScale returns the current snap scale factor.
func (c *Config) GetHapticsSnapScale() float64 {
	return c.snapshot.Load().SnapScale
}

// GetHapticsSnapMax returns the maximum snap value.
func (c *Config) GetHapticsSnapMax() int {
	return c.snapshot.Load().SnapMax
}

// SetHapticsSnapMax sets the maximum snap value.
// The snap curve is applied over the range from 0 to this maximum value.
// Any snap values above this value are clamped to this maximum.
// Allowed range is 1 to 200.
func (c *Config) SetHapticsSnapMax(value int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	value = min(value, 200)
	value = max(value, 1)

	c.viper.Haptics.SnapMax = value

	c.registerUpdate(false)
}

// IncreaseHapticsSnapMax increases the maximum snap value in increments of 1.
func (c *Config) IncreaseHapticsSnapMax() int {
	c.mu.Lock()

	c.viper.Haptics.SnapMax = min(
		100,
		c.viper.Haptics.SnapMax+1,
	)

	c.mu.Unlock()

	c.updateSnapScale()

	return c.viper.Haptics.SnapMax
}

// DecreaseHapticsSnapMax decreases the maximum snap value in increments of 1.
func (c *Config) DecreaseHapticsSnapMax() int {
	c.mu.Lock()

	c.viper.Haptics.SnapMax = max(
		1,
		c.viper.Haptics.SnapMax-1,
	)

	c.mu.Unlock()

	c.updateSnapScale()

	return c.viper.Haptics.SnapMax
}

// GetHapticsTransmissionCurve returns the transmission curve value.
// This curve is applied to the dynamic transmission feedback bsaed on the longitudinal vehicle g-force.
func (c *Config) GetHapticsTransmissionCurve() float64 {
	return float64(c.snapshot.Load().DynamicTransmissionCurve)
}

// SetHapticsTransmissionCurve sets the transmission curve value.
// This curve is applied to the dynamic transmission feedback bsaed on the longitudinal vehicle g-force.
func (c *Config) SetHapticsTransmissionCurve(value int) {
	c.mu.Lock()

	value = min(value, 955)
	value = max(value, 5)
	c.viper.Haptics.DynamicTransmissionCurve = value
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// IncreaseHapticsTransmissionCurve increases the transmission curve value in increments of 5.
func (c *Config) IncreaseHapticsTransmissionCurve() int {
	c.mu.Lock()
	c.viper.Haptics.DynamicTransmissionCurve = min(955, c.viper.Haptics.DynamicTransmissionCurve+5)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Haptics.DynamicTransmissionCurve
	c.mu.Unlock()

	return result
}

// DecreaseHapticsTransmissionCurve decreases the transmission curve value in increments of 5.
func (c *Config) DecreaseHapticsTransmissionCurve() int {
	c.mu.Lock()
	c.viper.Haptics.DynamicTransmissionCurve = max(5, c.viper.Haptics.DynamicTransmissionCurve-5)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Haptics.DynamicTransmissionCurve
	c.mu.Unlock()

	return result
}

// GetHapticsTransmissionGforceMax returns the maximum g-force for dynamic transmission feedback.
// Any longitudinal g-force values above this are clamped to this maximum.
func (c *Config) GetHapticsTransmissionGforceMax() float64 {
	return c.snapshot.Load().DynamicTransmissionGforceMax
}

// SetHapticsTransmissionGforceMax sets the maximum transmission G-force value.
// Any longitudinal g-force values above this are clamped to this maximum.
func (c *Config) SetHapticsTransmissionGforceMax(value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	value = math.Min(10, value)
	value = math.Max(0, value)

	c.viper.Haptics.DynamicTransmissionGforceMax = value

	c.registerUpdate(false)
}

// IncreaseHapticsTransmissionGforceMax increases the maximum g-force for dynamic transmission feedback in increments of 0.1g.
func (c *Config) IncreaseHapticsTransmissionGforceMax() float64 {
	c.mu.Lock()

	c.viper.Haptics.DynamicTransmissionGforceMax = min(
		5.0,
		c.viper.Haptics.DynamicTransmissionGforceMax+0.1,
	)

	c.mu.Unlock()

	c.registerUpdate(false)

	return c.viper.Haptics.DynamicTransmissionGforceMax
}

// DecreasehapticsTransmissionGforceMax decreases the maximum g-force for dynamic transmission feedback in increments of 0.1g.
func (c *Config) DecreasehapticsTransmissionGforceMax() float64 {
	c.mu.Lock()

	c.viper.Haptics.DynamicTransmissionGforceMax = max(
		0.1,
		c.viper.Haptics.DynamicTransmissionGforceMax-0.1,
	)

	c.mu.Unlock()

	c.registerUpdate(false)

	return c.viper.Haptics.DynamicTransmissionGforceMax
}

// GetHapticsPulseMinHz returns the configured minimum pulse frequency in Hz.
// This is the minimum frequency output for chassis bump haptics.
func (c *Config) GetHapticsPulseMinHz() float64 {
	return c.snapshot.Load().PulseMinFrequencyHz
}

// GetHapticsEngineProfile returns the currently selected engine profile.
// If no profile is selected, it returns nil.
func (c *Config) GetHapticsEngineProfile(name string) *appHaptics.EngineProfile {
	c.mu.Lock()
	defer c.mu.Unlock()

	name = strings.ToLower(name)
	if profile, ok := c.viper.Haptics.EngineProfiles[name]; ok {
		c.viper.Haptics._engineProfile = &profile
		c.viper.Haptics._engineProfileName = name
	} else {
		c.viper.Haptics._engineProfile = nil
		c.viper.Haptics._engineProfileName = ""
	}

	return c.viper.Haptics._engineProfile
}

// GetHapticesEnginePrimaryBalance returns the current engine primary balance.
// If no profile is selected, it returns 1.0 (perfect balance).
func (c *Config) GetHapticesEnginePrimaryBalance() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.viper.Haptics._engineProfile == nil {
		return 1.0
	}

	return c.viper.Haptics._engineProfile.PrimaryBalance
}

// IncreaseHapticsEnginePrimaryBalance increases the current engoine primary balancee in increments of 0.01.
// If no profile is selected, it returns 1.0 (perfect balance).
func (c *Config) IncreaseHapticsEnginePrimaryBalance() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.viper.Haptics._engineProfile == nil {
		return 1.0
	}

	c.viper.Haptics._engineProfile.PrimaryBalance = min(
		1.0,
		c.viper.Haptics._engineProfile.PrimaryBalance+0.01,
	)

	c.syncEngineProfileToMap()
	c.registerUpdate(false)

	return c.viper.Haptics._engineProfile.PrimaryBalance
}

// DecreaseHapticsEnginePrimaryBalance decreases the current engine primary balance in increments of 0.01.
// If no profile is selected, it returns 1.0 (perfect balance).
func (c *Config) DecreaseHapticsEnginePrimaryBalance() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.viper.Haptics._engineProfile == nil {
		return 1.0
	}

	c.viper.Haptics._engineProfile.PrimaryBalance = max(
		0.0,
		c.viper.Haptics._engineProfile.PrimaryBalance-0.01,
	)

	c.syncEngineProfileToMap()
	c.registerUpdate(false)

	return c.viper.Haptics._engineProfile.PrimaryBalance
}

// GetHapticsEngineSecondaryBalance returns the current engine secondary balance.
// If no profile is selected, it returns 1.0 (perfect balance).
func (c *Config) GetHapticsEngineSecondaryBalance() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.viper.Haptics._engineProfile == nil {
		return 1.0
	}

	return c.viper.Haptics._engineProfile.SecondaryBalance
}

// IncreaseHapticsEngineSecondaryBalance increases the current engine secondary balance in increments of 0.01.
// If no profile is selected, it returns 1.0 (perfect balance).
func (c *Config) IncreaseHapticsEngineSecondaryBalance() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.viper.Haptics._engineProfile == nil {
		return 1.0
	}

	c.viper.Haptics._engineProfile.SecondaryBalance = min(
		1.0,
		c.viper.Haptics._engineProfile.SecondaryBalance+0.01,
	)

	c.syncEngineProfileToMap()
	c.registerUpdate(false)

	return c.viper.Haptics._engineProfile.SecondaryBalance
}

// DecreaseHapticsEngineSecondaryBalance decreases the current engine secondary balance in increments of 0.01.
// If no profile is selected, it returns 1.0 (perfect balance).
func (c *Config) DecreaseHapticsEngineSecondaryBalance() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.viper.Haptics._engineProfile == nil {
		return 1.0
	}

	c.viper.Haptics._engineProfile.SecondaryBalance = max(
		0.0,
		c.viper.Haptics._engineProfile.SecondaryBalance-0.01,
	)

	c.syncEngineProfileToMap()
	c.registerUpdate(false)

	return c.viper.Haptics._engineProfile.SecondaryBalance
}

// GetHapticsEnginePulseGain returns the current engine pulse gain (i.e. engine haptic volume).
// If no profile is selected, it returns a gain level that silences engine haptics.
func (c *Config) GetHapticsEnginePulseGain() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.viper.Haptics._engineProfile == nil {
		return MinimumGain
	}

	return c.viper.Haptics._engineProfile.Gain
}

// IncreaseHapticsEnginePulseGain increases the current engine pulse gain by the configured increment.
// If no profile is selected, it returns a gain level that silences engine haptics.
func (c *Config) IncreaseHapticsEnginePulseGain() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.viper.Haptics._engineProfile == nil {
		return MinimumGain
	}

	c.viper.Haptics._engineProfile.Gain = min(
		MaximumGain,
		c.viper.Haptics._engineProfile.Gain+c.viper.Synthesizer.GainIncrement,
	)

	c.syncEngineProfileToMap()
	c.registerUpdate(false)

	return c.viper.Haptics._engineProfile.Gain
}

// DecreaseHapticsEnginePulseGain decreases the current engine pulse gain by the configured increment.
// If no profile is selected, it returns a gain level that silences engine haptics.
func (c *Config) DecreaseHapticsEnginePulseGain() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.viper.Haptics._engineProfile == nil {
		return MinimumGain
	}

	c.viper.Haptics._engineProfile.Gain = max(
		MinimumGain,
		c.viper.Haptics._engineProfile.Gain-c.viper.Synthesizer.GainIncrement,
	)

	c.syncEngineProfileToMap()
	c.registerUpdate(false)

	return c.viper.Haptics._engineProfile.Gain
}

// GetHapticsEnginePulseScale returns the current engine pulse scale factor.
// If no profile is selected, it returns a scale factor of 1.0 (no scaling).
func (c *Config) GetHapticsEnginePulseScale() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.viper.Haptics._engineProfile == nil {
		return 1.0
	}

	return c.viper.Haptics._engineProfile.PulseScale
}

// IncreaseHapticsEnginePulseScale increases the current engine pulse scale factor in increments of 0.01.
// If no profile is selected, it returns a scale factor of 1.0 (no scaling).
func (c *Config) IncreaseHapticsEnginePulseScale() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.viper.Haptics._engineProfile == nil {
		return 1.0
	}

	c.viper.Haptics._engineProfile.PulseScale = min(
		1.0,
		c.viper.Haptics._engineProfile.PulseScale+0.01,
	)

	c.syncEngineProfileToMap()
	c.registerUpdate(false)

	return c.viper.Haptics._engineProfile.PulseScale
}

// DecreasehapticsEnginePulseScale decreases the current engine pulse scale factor in increments of 0.01.
// If no profile is selected, it returns a scale factor of 1.0 (no scaling).
func (c *Config) DecreasehapticsEnginePulseScale() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.viper.Haptics._engineProfile == nil {
		return 1.0
	}

	c.viper.Haptics._engineProfile.PulseScale = max(
		0.0,
		c.viper.Haptics._engineProfile.PulseScale-0.01,
	)

	c.syncEngineProfileToMap()
	c.registerUpdate(false)

	return c.viper.Haptics._engineProfile.PulseScale
}

// IncreaseHapticsPulseMinHz increases the minimum pulse frequency in 1 Hz increments.
// This is the minimum frequency output for chassis bump haptics and is clamped to a maximum of 25Hz.
func (c *Config) IncreaseHapticsPulseMinHz() int {
	c.mu.Lock()
	c.viper.Haptics.PulseMinFrequencyHz = min(25, c.viper.Haptics.PulseMinFrequencyHz+1)
	result := int(c.viper.Haptics.PulseMinFrequencyHz)
	c.mu.Unlock()
	c.updatePulseWidthExtents()

	return result
}

// DecreaseHapticsPulseMinHz decreases the minimum pulse frequency in 1 Hz increments.
// This is the minimum frequency output for chassis bump haptics and is clamped to a minimum of 5Hz.
func (c *Config) DecreaseHapticsPulseMinHz() int {
	c.mu.Lock()
	c.viper.Haptics.PulseMinFrequencyHz = max(5, c.viper.Haptics.PulseMinFrequencyHz-1)
	result := int(c.viper.Haptics.PulseMinFrequencyHz)
	c.mu.Unlock()
	c.updatePulseWidthExtents()

	return result
}

// GetHapticsPulseMaxHz returns the configured maximum pulse frequency in Hz.
// This is the maximum frequency output for chassis bump haptics.
func (c *Config) GetHapticsPulseMaxHz() float64 {
	return c.snapshot.Load().PulseMaxFrequencyHz
}

// IncreaseHapticsPulseMaxHz increases the maximum pulse frequency in 1 Hz increments.
// This is the maximum frequency output for chassis bump haptics and is clamped to a maximum of 100Hz.
func (c *Config) IncreaseHapticsPulseMaxHz() int {
	c.mu.Lock()
	c.viper.Haptics.PulseMaxFrequencyHz = min(100, c.viper.Haptics.PulseMaxFrequencyHz+1)
	result := int(c.viper.Haptics.PulseMaxFrequencyHz)
	c.mu.Unlock()
	c.updatePulseWidthExtents()

	return result
}

// DecreaseHapticsPulseMaxHz decreases the maximum pulse frequency in 1 Hz increments.
// This is the maximum frequency output for chassis bump haptics and is clamped to a minimum of 26Hz.
func (c *Config) DecreaseHapticsPulseMaxHz() int {
	c.mu.Lock()
	c.viper.Haptics.PulseMaxFrequencyHz = max(26, c.viper.Haptics.PulseMaxFrequencyHz-1)
	result := int(c.viper.Haptics.PulseMaxFrequencyHz)
	c.mu.Unlock()
	c.updatePulseWidthExtents()

	return result
}

// GetHapticePulseFrequencyHzRange returns the range between the configured minimum and maximum pulse frequencies in Hz.
// This is the frequency range output for chassis bump haptics.
func (c *Config) GetHapticePulseFrequencyHzRange() float64 {
	snap := c.snapshot.Load()

	return snap.PulseMaxFrequencyHz - snap.PulseMinFrequencyHz
}

// GetHapticsPulseWidthMin returns the minimum pulse width in samples based on the current max frequency.
func (c *Config) GetHapticsPulseWidthMin() float64 {
	return c.snapshot.Load().PulseWidthMin
}

// GetHapticsPulseWidthMax returns the maximum pulse width in samples based on the current min and max frequencies.
func (c *Config) GetHapticsPulseWidthMax() float64 {
	return c.snapshot.Load().PulseWidthMax
}

// GetHapticsPulseMaxAmplitude returns the maximum pulse amplitude for chassis bump haptics.
func (c *Config) GetHapticsPulseMaxAmplitude() float64 {
	return c.snapshot.Load().PulseMaxAmplitude
}

// SetHapticsPulseMaxAmplitude sets the maximum pulse amplitude for chassis bump haptics.
func (c *Config) SetHapticsPulseMaxAmplitude(value float64) {
	c.mu.Lock()
	c.viper.Haptics.PulseMaxAmplitude = value
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// IncreaseHapticsPulseMaxAmplitude increases the maximum pulse amplitude for chassis bump haptics in increments of 0.01.
func (c *Config) IncreaseHapticsPulseMaxAmplitude() float64 {
	c.mu.Lock()
	c.viper.Haptics.PulseMaxAmplitude = min(1.0, c.viper.Haptics.PulseMaxAmplitude+0.01)
	result := c.viper.Haptics.PulseMaxAmplitude
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()

	return result
}

// DecreaseHapticsPulseMaxAmplitude decreases the maximum pulse amplitude for chassis bump haptics in increments of 0.01.
func (c *Config) DecreaseHapticsPulseMaxAmplitude() float64 {
	c.mu.Lock()
	c.viper.Haptics.PulseMaxAmplitude = max(0.0, c.viper.Haptics.PulseMaxAmplitude-0.01)
	result := c.viper.Haptics.PulseMaxAmplitude
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()

	return result
}

// GetHapticsPulseMaxFrequencyHz returns the maximum pulse frequency in Hz for chassis bump haptics.
func (c *Config) GetHapticsPulseMaxFrequencyHz() float64 {
	return c.snapshot.Load().PulseMaxFrequencyHz
}

// SetHapticsPulseMaxFrequencyHz sets the maximum pulse frequency in Hz for chassis bump haptics.
func (c *Config) SetHapticsPulseMaxFrequencyHz(value float64) {
	c.mu.Lock()
	c.viper.Haptics.PulseMaxFrequencyHz = value
	c.updatePulseWidthExtents()
	c.mu.Unlock()
}

// SetHapticsPulseMinFrequencyHz sets the minimum pulse frequency in Hz for chassis bump haptics.
func (c *Config) SetHapticsPulseMinFrequencyHz(value float64) {
	c.mu.Lock()
	c.viper.Haptics.PulseMinFrequencyHz = value
	c.updatePulseWidthExtents()
	c.mu.Unlock()
}

// ****************************************************************************
// Pit Radio section methods.
// ****************************************************************************

// PitRadioEnabled returns true if pit radio integration is enabled.
func (c *Config) PitRadioEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Enabled
}

// SetPitRadioEnabled sets whether pit radio integration is enabled.
func (c *Config) SetPitRadioEnabled(value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Enabled = value

	c.registerUpdate(true)
}

func (c *Config) GetPitRadioOutput() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.viper.PitRadio.Output == "" {
		c.viper.PitRadio.Output = "log"
	}

	return c.viper.PitRadio.Output
}

// SetPitRadioOutput sets the pit radio output device.
func (c *Config) SetPitRadioOutput(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch value {
	case "discord":
		c.viper.PitRadio.Output = value
	default:
		c.viper.PitRadio.Output = "log"
	}

	c.registerUpdate(true)
}

// GetPitRadioMessageSendIntervalMs returns the interval in milliseconds between sending of pit radio messages.
func (c *Config) GetPitRadioMessageSendIntervalMs() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.MessageSendIntervalMs
}

// SetPitRadioMessageSendIntervalMs sets the interval in milliseconds between sending of pit radio messages.
func (c *Config) SetPitRadioMessageSendIntervalMs(value int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.MessageSendIntervalMs = value

	c.registerUpdate(false)
}

// GetPitRadioNotifyRaceProgressEnabled returns whether race progress notifications are enabled.
func (c *Config) GetPitRadioNotifyRaceProgressEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Notifications.EnableRaceProgress
}

// SetPitRadioNotifyRaceProgressEnabled sets whether race progress notifications are enabled.
func (c *Config) SetPitRadioNotifyRaceProgressEnabled(value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.EnableRaceProgress = value

	c.registerUpdate(false)
}

// GetPitRadioNotifyRaceProgressMinLaps returns the minimum number of laps before race progress notifications begin.
func (c *Config) GetPitRadioNotifyRaceProgressMinLaps() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Notifications.RaceProgressMinLaps
}

// SetPitRadioNotifyRaceProgressMinLaps sets the minimum number of laps before race progress notifications begin.
func (c *Config) SetPitRadioNotifyRaceProgressMinLaps(value int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceProgressMinLaps = value

	c.registerUpdate(false)
}

// IncreasePitRadioNotifyRaceProgressMinLaps increases the minimum laps by 1 (max 50).
func (c *Config) IncreasePitRadioNotifyRaceProgressMinLaps() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceProgressMinLaps = min(50, c.viper.PitRadio.Notifications.RaceProgressMinLaps+1)
	c.registerUpdate(false)

	return c.viper.PitRadio.Notifications.RaceProgressMinLaps
}

// DecreasePitRadioNotifyRaceProgressMinLaps decreases the minimum laps by 1 (min 1).
func (c *Config) DecreasePitRadioNotifyRaceProgressMinLaps() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceProgressMinLaps = max(1, c.viper.PitRadio.Notifications.RaceProgressMinLaps-1)
	c.registerUpdate(false)

	return c.viper.PitRadio.Notifications.RaceProgressMinLaps
}

// GetPitRadioNotifyRaceProgressIntervalPc returns the race progress notification interval percentage.
func (c *Config) GetPitRadioNotifyRaceProgressIntervalPc() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Notifications.RaceProgressIntervalPc
}

// SetPitRadioNotifyRaceProgressIntervalPc sets the race progress notification interval percentage.
func (c *Config) SetPitRadioNotifyRaceProgressIntervalPc(value int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceProgressIntervalPc = min(50, max(5, value))

	c.registerUpdate(false)
}

// IncreasePitRadioNotifyRaceProgressIntervalPc increases the interval by 5% (max 50).
func (c *Config) IncreasePitRadioNotifyRaceProgressIntervalPc() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceProgressIntervalPc = min(50, c.viper.PitRadio.Notifications.RaceProgressIntervalPc+5)
	c.registerUpdate(false)

	return c.viper.PitRadio.Notifications.RaceProgressIntervalPc
}

// DecreasePitRadioNotifyRaceProgressIntervalPc decreases the interval by 5% (min 5).
func (c *Config) DecreasePitRadioNotifyRaceProgressIntervalPc() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceProgressIntervalPc = max(5, c.viper.PitRadio.Notifications.RaceProgressIntervalPc-5)
	c.registerUpdate(false)

	return c.viper.PitRadio.Notifications.RaceProgressIntervalPc
}

// GetPitRadioNotifyRaceLapsEnabled returns whether race lap notifications are enabled.
func (c *Config) GetPitRadioNotifyRaceLapsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Notifications.EnableRaceLaps
}

// SetPitRadioNotifyRaceLapsEnabled sets whether race lap notifications are enabled.
func (c *Config) SetPitRadioNotifyRaceLapsEnabled(value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.EnableRaceLaps = value

	c.registerUpdate(false)
}

// GetPitRadioNotifyRaceLapsIntervalLaps returns the interval in laps for race lap notifications.
func (c *Config) GetPitRadioNotifyRaceLapsIntervalLaps() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Notifications.RaceLapsIntervalLaps
}

// SetPitRadioNotifyRaceLapsIntervalLaps sets the interval in laps for race lap notifications.
func (c *Config) SetPitRadioNotifyRaceLapsIntervalLaps(value int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceLapsIntervalLaps = min(50, max(1, value))

	c.registerUpdate(false)
}

// IncreasePitRadioNotifyRaceLapsIntervalLaps increases the interval by 1 lap (max 50).
func (c *Config) IncreasePitRadioNotifyRaceLapsIntervalLaps() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceLapsIntervalLaps = min(50, c.viper.PitRadio.Notifications.RaceLapsIntervalLaps+1)
	c.registerUpdate(false)

	return c.viper.PitRadio.Notifications.RaceLapsIntervalLaps
}

// DecreasePitRadioNotifyRaceLapsIntervalLaps decreases the interval by 1 lap (min 1).
func (c *Config) DecreasePitRadioNotifyRaceLapsIntervalLaps() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceLapsIntervalLaps = max(1, c.viper.PitRadio.Notifications.RaceLapsIntervalLaps-1)
	c.registerUpdate(false)

	return c.viper.PitRadio.Notifications.RaceLapsIntervalLaps
}

// GetPitRadioNotifyRaceLapsCountdownLaps returns the number of laps for countdown notifications.
func (c *Config) GetPitRadioNotifyRaceLapsCountdownLaps() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Notifications.RaceLapsCountdownLaps
}

// SetPitRadioNotifyRaceLapsCountdownLaps sets the number of laps for countdown notifications.
func (c *Config) SetPitRadioNotifyRaceLapsCountdownLaps(value int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceLapsCountdownLaps = min(25, max(1, value))

	c.registerUpdate(false)
}

// IncreasePitRadioNotifyRaceLapsCountdownLaps increases the countdown laps by 1 (max 25).
func (c *Config) IncreasePitRadioNotifyRaceLapsCountdownLaps() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceLapsCountdownLaps = min(25, c.viper.PitRadio.Notifications.RaceLapsCountdownLaps+1)
	c.registerUpdate(false)

	return c.viper.PitRadio.Notifications.RaceLapsCountdownLaps
}

// DecreasePitRadioNotifyRaceLapsCountdownLaps decreases the countdown laps by 1 (min 1).
func (c *Config) DecreasePitRadioNotifyRaceLapsCountdownLaps() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.RaceLapsCountdownLaps = max(1, c.viper.PitRadio.Notifications.RaceLapsCountdownLaps-1)
	c.registerUpdate(false)

	return c.viper.PitRadio.Notifications.RaceLapsCountdownLaps
}

// GetPitRadioNotifyLapTimesEnabled returns whether lap time notifications are enabled.
func (c *Config) GetPitRadioNotifyLapTimesEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Notifications.EnableLapTimes
}

// SetPitRadioNotifyLapTimesEnabled sets whether lap time notifications are enabled.
func (c *Config) SetPitRadioNotifyLapTimesEnabled(value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.EnableLapTimes = value

	c.registerUpdate(false)
}

// GetPitRadioNotifyLapTimesMaxDeltaSeconds returns the maximum delta in seconds for lap time notifications.
func (c *Config) GetPitRadioNotifyLapTimesMaxDeltaSeconds() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Notifications.LapTimesMaxDeltaSeconds
}

// SetPitRadioNotifyLapTimesMaxDeltaSeconds sets the maximum delta in seconds for lap time notifications.
func (c *Config) SetPitRadioNotifyLapTimesMaxDeltaSeconds(value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.LapTimesMaxDeltaSeconds = min(30.0, max(0.1, value))

	c.registerUpdate(false)
}

// IncreasePitRadioNotifyLapTimesMaxDeltaSeconds increases the maximum delta by 0.1 seconds (max 30.0).
func (c *Config) IncreasePitRadioNotifyLapTimesMaxDeltaSeconds() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.LapTimesMaxDeltaSeconds = min(30.0, c.viper.PitRadio.Notifications.LapTimesMaxDeltaSeconds+0.1)
	c.registerUpdate(false)

	return c.viper.PitRadio.Notifications.LapTimesMaxDeltaSeconds
}

// DecreasePitRadioNotifyLapTimesMaxDeltaSeconds decreases the maximum delta by 0.1 seconds (min 0.1).
func (c *Config) DecreasePitRadioNotifyLapTimesMaxDeltaSeconds() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.LapTimesMaxDeltaSeconds = max(0.1, c.viper.PitRadio.Notifications.LapTimesMaxDeltaSeconds-0.1)
	c.registerUpdate(false)

	return c.viper.PitRadio.Notifications.LapTimesMaxDeltaSeconds
}

// GetPitRadioNotifyCircuitMatchingEnabled returns whether circuit change notifications are enabled.
func (c *Config) GetPitRadioNotifyCircuitMatchingEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Notifications.EnableCircuitMatching
}

// SetPitRadioNotifyCircuitMatchingEnabled sets whether circuit change notifications are enabled.
func (c *Config) SetPitRadioNotifyCircuitMatchingEnabled(value bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Notifications.EnableCircuitMatching = value

	c.registerUpdate(false)
}

// GetPitRadioFuelMonitoringEnabled returns true if fuel monitoring is enabled.
func (c *Config) GetPitRadioFuelMonitoringEnabled() bool {
	return c.snapshot.Load().FuelMonitoringEnabled
}

// SetPitRadioFuelMonitoringEnabled sets whether fuel monitoring is enabled.
func (c *Config) SetPitRadioFuelMonitoringEnabled(value bool) {
	c.mu.Lock()
	c.viper.PitRadio.FuelMonitoring.Enabled = value
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// GetPitRadioFuelPreWarnNotifyLaps returns the number of laps remaining before a fuel pre-warning is triggered.
func (c *Config) GetPitRadioFuelPreWarnNotifyLaps() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.FuelMonitoring.PreWarnNotifyLaps
}

// SetPitRadioFuelPreWarnNotifyLaps sets the number of laps remaining before a fuel pre-warning is triggered.
func (c *Config) SetPitRadioFuelPreWarnNotifyLaps(value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.PreWarnNotifyLaps = min(10.0, max(0.0, value))

	c.registerUpdate(false)
}

// IncreasePitRadioFuelPreWarnNotifyLaps increases the pre-warn laps by 0.1 (max 10.0).
func (c *Config) IncreasePitRadioFuelPreWarnNotifyLaps() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.PreWarnNotifyLaps = min(10.0, c.viper.PitRadio.FuelMonitoring.PreWarnNotifyLaps+0.1)
	c.registerUpdate(false)

	return c.viper.PitRadio.FuelMonitoring.PreWarnNotifyLaps
}

// DecreasePitRadioFuelPreWarnNotifyLaps decreases the pre-warn laps by 0.1 (min 0.0).
func (c *Config) DecreasePitRadioFuelPreWarnNotifyLaps() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.PreWarnNotifyLaps = max(0.0, c.viper.PitRadio.FuelMonitoring.PreWarnNotifyLaps-0.1)
	c.registerUpdate(false)

	return c.viper.PitRadio.FuelMonitoring.PreWarnNotifyLaps
}

// GetPitRadioFuelStrategyNotifyLaps returns the number of laps remaining before a fuel strategy notification is triggered.
func (c *Config) GetPitRadioFuelStrategyNotifyLaps() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.FuelMonitoring.StrategyNotifyLaps
}

// SetPitRadioFuelStrategyNotifyLaps sets the number of laps remaining before a fuel strategy notification is triggered.
func (c *Config) SetPitRadioFuelStrategyNotifyLaps(value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.StrategyNotifyLaps = min(20.0, max(0.0, value))

	c.registerUpdate(false)
}

// IncreasePitRadioFuelStrategyNotifyLaps increases the strategy notify laps by 0.1 (max 20.0).
func (c *Config) IncreasePitRadioFuelStrategyNotifyLaps() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.StrategyNotifyLaps = min(20.0, c.viper.PitRadio.FuelMonitoring.StrategyNotifyLaps+0.1)
	c.registerUpdate(false)

	return c.viper.PitRadio.FuelMonitoring.StrategyNotifyLaps
}

// DecreasePitRadioFuelStrategyNotifyLaps decreases the strategy notify laps by 0.1 (min 0.0).
func (c *Config) DecreasePitRadioFuelStrategyNotifyLaps() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.StrategyNotifyLaps = max(0.0, c.viper.PitRadio.FuelMonitoring.StrategyNotifyLaps-0.1)
	c.registerUpdate(false)

	return c.viper.PitRadio.FuelMonitoring.StrategyNotifyLaps
}

// GetPitRadioFuelRangeSafetyMarginLaps returns the safety margin in laps to apply when calculating fuel range.
func (c *Config) GetPitRadioFuelRangeSafetyMarginLaps() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginLaps
}

// SetPitRadioFuelRangeSafetyMarginLaps sets the safety margin in laps to apply when calculating fuel range.
func (c *Config) SetPitRadioFuelRangeSafetyMarginLaps(value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginLaps = min(2.0, max(0.0, value))

	c.registerUpdate(false)
}

// IncreasePitRadioFuelRangeSafetyMarginLaps increases the safety margin by 0.05 laps (max 2.0).
func (c *Config) IncreasePitRadioFuelRangeSafetyMarginLaps() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginLaps = min(2.0, c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginLaps+0.05)
	c.registerUpdate(false)

	return c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginLaps
}

// DecreasePitRadioFuelRangeSafetyMarginLaps decreases the safety margin by 0.05 laps (min 0.0).
func (c *Config) DecreasePitRadioFuelRangeSafetyMarginLaps() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginLaps = max(0.0, c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginLaps-0.05)
	c.registerUpdate(false)

	return c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginLaps
}

// GetPitRadioFuelRangeSafetyMarginMetres returns the safety margin in metres to apply when calculating fuel range.
func (c *Config) GetPitRadioFuelRangeSafetyMarginMetres() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginMetres
}

// SetPitRadioFuelRangeSafetyMarginMetres sets the safety margin in metres to apply when calculating fuel range.
func (c *Config) SetPitRadioFuelRangeSafetyMarginMetres(value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginMetres = min(2000.0, max(0.0, value))

	c.registerUpdate(false)
}

// IncreasePitRadioFuelRangeSafetyMarginMetres increases the safety margin by 50 metres (max 2000).
func (c *Config) IncreasePitRadioFuelRangeSafetyMarginMetres() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginMetres = min(2000.0, c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginMetres+50)
	c.registerUpdate(false)

	return c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginMetres
}

// DecreasePitRadioFuelRangeSafetyMarginMetres decreases the safety margin by 50 metres (min 0).
func (c *Config) DecreasePitRadioFuelRangeSafetyMarginMetres() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginMetres = max(0.0, c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginMetres-50)
	c.registerUpdate(false)

	return c.viper.PitRadio.FuelMonitoring.RangeSafetyMarginMetres
}

// GetPitRadioTyreMonitoringEnabled returns whether tyre monitoring is enabled.
func (c *Config) GetPitRadioTyreMonitoringEnabled() bool {
	return c.snapshot.Load().TyreMonitoringEnabled
}

// SetPitRadioTyreMonitoringEnabled sets whether tyre monitoring is enabled.
func (c *Config) SetPitRadioTyreMonitoringEnabled(value bool) {
	c.mu.Lock()
	c.viper.PitRadio.TyreMonitoring.Enabled = value
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// GetPitRadioTyreTemperatureOptimalCelsius returns the optimal (center) tyre temperature in Celsius.
func (c *Config) GetPitRadioTyreTemperatureOptimalCelsius() float32 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.TyreMonitoring.TemperatureOptimalCelsius
}

// SetPitRadioTyreTemperatureOptimalCelsius sets the optimal (center) tyre temperature in Celsius.
func (c *Config) SetPitRadioTyreTemperatureOptimalCelsius(value float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.TyreMonitoring.TemperatureOptimalCelsius = min(120.0, max(60.0, value))

	c.registerUpdate(false)
}

// IncreasePitRadioTyreTemperatureOptimalCelsius increases the optimal temperature by 1°C (max 120).
func (c *Config) IncreasePitRadioTyreTemperatureOptimalCelsius() float32 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.TyreMonitoring.TemperatureOptimalCelsius = min(120.0, c.viper.PitRadio.TyreMonitoring.TemperatureOptimalCelsius+1)
	c.registerUpdate(false)

	return c.viper.PitRadio.TyreMonitoring.TemperatureOptimalCelsius
}

// DecreasePitRadioTyreTemperatureOptimalCelsius decreases the optimal temperature by 1°C (min 60).
func (c *Config) DecreasePitRadioTyreTemperatureOptimalCelsius() float32 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.TyreMonitoring.TemperatureOptimalCelsius = max(60.0, c.viper.PitRadio.TyreMonitoring.TemperatureOptimalCelsius-1)
	c.registerUpdate(false)

	return c.viper.PitRadio.TyreMonitoring.TemperatureOptimalCelsius
}

// GetPitRadioTyreTemperatureOperatingWindow returns the total operating window width around optimal temperature in Celsius.
// The ideal temperature range is calculated as optimal ± (window/2).
func (c *Config) GetPitRadioTyreTemperatureOperatingWindow() float32 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.TyreMonitoring.TemperatureOperatingWindow
}

// SetPitRadioTyreTemperatureOperatingWindow sets the total operating window width around optimal temperature in Celsius.
func (c *Config) SetPitRadioTyreTemperatureOperatingWindow(value float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.TyreMonitoring.TemperatureOperatingWindow = min(20.0, max(0.5, value))

	c.registerUpdate(false)
}

// IncreasePitRadioTyreTemperatureOperatingWindow increases the operating window by 0.5°C (max 20.0).
func (c *Config) IncreasePitRadioTyreTemperatureOperatingWindow() float32 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.TyreMonitoring.TemperatureOperatingWindow = min(20.0, c.viper.PitRadio.TyreMonitoring.TemperatureOperatingWindow+0.5)
	c.registerUpdate(false)

	return c.viper.PitRadio.TyreMonitoring.TemperatureOperatingWindow
}

// DecreasePitRadioTyreTemperatureOperatingWindow decreases the operating window by 0.5°C (min 0.5).
func (c *Config) DecreasePitRadioTyreTemperatureOperatingWindow() float32 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.TyreMonitoring.TemperatureOperatingWindow = max(0.5, c.viper.PitRadio.TyreMonitoring.TemperatureOperatingWindow-0.5)
	c.registerUpdate(false)

	return c.viper.PitRadio.TyreMonitoring.TemperatureOperatingWindow
}

// GetPitRadioTyreTemperatureMarginCelsius returns the margin beyond operating window for hot/cold thresholds in Celsius.
func (c *Config) GetPitRadioTyreTemperatureMarginCelsius() float32 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.TyreMonitoring.TemperatureMarginCelsius
}

// SetPitRadioTyreTemperatureMarginCelsius sets the margin beyond operating window for hot/cold thresholds in Celsius.
func (c *Config) SetPitRadioTyreTemperatureMarginCelsius(value float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.TyreMonitoring.TemperatureMarginCelsius = min(10.0, max(0.5, value))

	c.registerUpdate(false)
}

// IncreasePitRadioTyreTemperatureMarginCelsius increases the temperature margin by 0.5°C (max 10.0).
func (c *Config) IncreasePitRadioTyreTemperatureMarginCelsius() float32 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.TyreMonitoring.TemperatureMarginCelsius = min(10.0, c.viper.PitRadio.TyreMonitoring.TemperatureMarginCelsius+0.5)
	c.registerUpdate(false)

	return c.viper.PitRadio.TyreMonitoring.TemperatureMarginCelsius
}

// DecreasePitRadioTyreTemperatureMarginCelsius decreases the temperature margin by 0.5°C (min 0.5).
func (c *Config) DecreasePitRadioTyreTemperatureMarginCelsius() float32 {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.TyreMonitoring.TemperatureMarginCelsius = max(0.5, c.viper.PitRadio.TyreMonitoring.TemperatureMarginCelsius-0.5)
	c.registerUpdate(false)

	return c.viper.PitRadio.TyreMonitoring.TemperatureMarginCelsius
}

// ****************************************************************************
// Discord pit radio sub-section methods.
// ****************************************************************************

// GetDiscordToken returns the Discord API token.
func (c *Config) GetDiscordToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Discord.Token
}

// SetDiscordToken sets the Discord API token.
func (c *Config) SetDiscordToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Discord.Token = token

	c.registerUpdate(true)
}

// GetDiscordGuildID returns the Discord guild (server) ID.
func (c *Config) GetDiscordGuildID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Discord.GuildID
}

// SetDiscordGuildID sets the Discord guild (server) ID.
func (c *Config) SetDiscordGuildID(guildID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Discord.GuildID = guildID

	c.registerUpdate(true)
}

// GetDiscordChannelID returns the Discord text channel ID.
func (c *Config) GetDiscordChannelID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Discord.ChannelID
}

// SetDiscordChannelID sets the Discord text channel ID.
func (c *Config) SetDiscordChannelID(channelID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Discord.ChannelID = channelID

	c.registerUpdate(true)
}

// GetDiscordVoiceChannelID returns the Discord voice channel ID.
func (c *Config) GetDiscordVoiceChannelID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.PitRadio.Discord.VoiceChannelID
}

// SetDiscordVoiceChannelID sets the Discord voice channel ID.
func (c *Config) SetDiscordVoiceChannelID(voiceChannelID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.viper.PitRadio.Discord.VoiceChannelID = voiceChannelID

	c.registerUpdate(true)
}

// ****************************************************************************
// Synthesizer methods.
// ****************************************************************************

// GetSynthesizer returns the synthesizer configuration.
func (c *Config) GetSynthesizer() *Synthesizer {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.Synthesizer
}

// GetSynthInternalSampleRateHz returns the internal sample rate of the synthesizer in Hz.
// This is the sample rate at which the synthesizer processes audio.
// Lower values reduce CPU load and 8000 Hz should be more than sufficient for the haptic frequency range.
func (c *Config) GetSynthInternalSampleRateHz() int {
	return c.snapshot.Load().InternalSampleRateHz
}

// SetSynthInternalSampleRateHz sets the internal sample rate of the synthesizer in Hz.
func (c *Config) SetSynthInternalSampleRateHz(value int) {
	c.mu.Lock()
	c.viper.Synthesizer.InternalSampleRateHz = value
	c.updatePulseWidthExtents()
	c.rebuildSnapshot()
	c.registerUpdate(true)
	c.mu.Unlock()
}

// GetSynthOutputSampleRateHz returns the output sample rate of the synthesizer in Hz.
// This is the sample rate at which audio is output to the audio device or file.
// 32000 Hz is suitable for most common hardware but some may work at lower rates.
func (c *Config) GetSynthOutputSampleRateHz() int {
	return c.snapshot.Load().OutputSampleRateHz
}

// SetSynthOutputSampleRateHz sets the output sample rate of the synthesizer in Hz.
func (c *Config) SetSynthOutputSampleRateHz(value int) {
	c.mu.Lock()
	c.viper.Synthesizer.OutputSampleRateHz = value
	c.rebuildSnapshot()
	c.registerUpdate(true)
	c.mu.Unlock()
}

// GetSynthGainIncrement returns the gain increment value.
func (c *Config) GetSynthGainIncrement() float64 {
	return c.snapshot.Load().GainIncrement
}

// SetSynthGainIncrement sets the gain increment value.
func (c *Config) SetSynthGainIncrement(value float64) {
	c.mu.Lock()

	value = max(0.01, value)
	value = min(10, value)
	c.viper.Synthesizer.GainIncrement = value
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// GetSynthMasterGain returns the master gain of the synthesizer (i.e. the overall volume level).
// This is a global gain applied to all haptic feedback.
// 0.0 is maximum gain and -60.0 will mute haptic output.
func (c *Config) GetSynthMasterGain() float64 {
	return c.snapshot.Load().MasterGain
}

// SetSynthMasterGain sets the master gain of the synthesizer.
// This is a global gain applied to all haptic feedback.
// 0.0 is maximum gain and -60.0 will mute haptic output.
func (c *Config) SetSynthMasterGain(value float64) {
	c.mu.Lock()
	c.viper.Synthesizer.MasterGain = max(MinimumGain, min(MaximumGain, value))
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// GetSynthMasterMute returns whether the master gain is muted.
func (c *Config) GetSynthMasterMute() bool {
	return c.snapshot.Load().MasterMute
}

// SetSynthMasterMute sets whether the master gain is muted.
func (c *Config) SetSynthMasterMute(mute bool) {
	c.mu.Lock()
	c.viper.Synthesizer.MasterMute = mute
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// IncreaseSynthMasterGain increases the master gain by the configured gain increment.
func (c *Config) IncreaseSynthMasterGain() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.MasterGain = min(
		MaximumGain,
		c.viper.Synthesizer.MasterGain+c.viper.Synthesizer.GainIncrement,
	)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.MasterGain
	c.mu.Unlock()

	return result
}

// DecreaseSynthMasterGain decreases the master gain by the configured gain increment.
func (c *Config) DecreaseSynthMasterGain() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.MasterGain = max(
		MinimumGain,
		c.viper.Synthesizer.MasterGain-c.viper.Synthesizer.GainIncrement,
	)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.MasterGain
	c.mu.Unlock()

	return result
}

// GetSynthChannelGain returns the gain for a specific channel of the synthesizer
// 0.0 is maximum gain and -60.0 will mute haptic output.
func (c *Config) GetSynthChannelGain(channel int) float64 {
	snap := c.snapshot.Load()
	if channel < 0 || channel >= len(snap.ChannelGain) {
		return 0.0
	}

	return snap.ChannelGain[channel]
}

// SetSynthChannelGain sets the gain for a specific channel of the synthesizer
// 0.0 is maximum gain and -60.0 will mute haptic output.
func (c *Config) SetSynthChannelGain(channel int, value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if channel < 0 || channel >= len(c.viper.Synthesizer.ChannelGain) {
		return
	}

	c.viper.Synthesizer.ChannelGain[channel] = max(MinimumGain, min(MaximumGain, value))

	c.rebuildSnapshot()
	c.registerUpdate(false)
}

// GetSynthChannelMute returns whether a specific channel is muted.
func (c *Config) GetSynthChannelMute(channel int) bool {
	snap := c.snapshot.Load()
	if channel >= 0 && channel < len(snap.ChannelMute) {
		return snap.ChannelMute[channel]
	}

	return false
}

// SetSynthChannelMute sets whether a specific channel is muted.
func (c *Config) SetSynthChannelMute(channel int, mute bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if channel < 0 || channel >= len(c.viper.Synthesizer.ChannelMute) {
		return
	}

	c.viper.Synthesizer.ChannelMute[channel] = mute

	c.rebuildSnapshot()
	c.registerUpdate(false)
}

// IncreaseSynthChannelGain increases the gain for a specific channel by the configured gain increment.
func (c *Config) IncreaseSynthChannelGain(channel int) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if channel < 0 || channel >= len(c.viper.Synthesizer.ChannelGain) {
		return 0.0
	}

	currentGain := c.viper.Synthesizer.ChannelGain[channel]
	c.viper.Synthesizer.ChannelGain[channel] = min(
		MaximumGain,
		currentGain+c.viper.Synthesizer.GainIncrement,
	)
	result := c.viper.Synthesizer.ChannelGain[channel]

	c.rebuildSnapshot()
	c.registerUpdate(false)

	return result
}

// DecreaseSynthChannelGain decreases the gain for a specific channel by the configured gain increment.
func (c *Config) DecreaseSynthChannelGain(channel int) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if channel < 0 || channel >= len(c.viper.Synthesizer.ChannelGain) {
		return 0.0
	}

	currentGain := c.viper.Synthesizer.ChannelGain[channel]
	c.viper.Synthesizer.ChannelGain[channel] = max(
		MinimumGain,
		currentGain-c.viper.Synthesizer.GainIncrement,
	)
	result := c.viper.Synthesizer.ChannelGain[channel]

	c.rebuildSnapshot()
	c.registerUpdate(false)

	return result
}

// GetSynthChassisGain returns the chassis gain of the synthesizer (i.e. the volume level for chassis bump haptics).
// 0.0 is maximum gain and -60.0 will mute chassis bump haptic output.
func (c *Config) GetSynthChassisGain() float64 {
	return c.snapshot.Load().ChassisGain
}

// GetSynthChassisMute returns whether the chassis gain is muted.
func (c *Config) GetSynthChassisMute() bool {
	return c.snapshot.Load().ChassisMute
}

// SetSynthChassisMute sets whether the chassis gain is muted.
func (c *Config) SetSynthChassisMute(mute bool) {
	c.mu.Lock()
	c.viper.Synthesizer.ChassisMute = mute
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// SetSynthChassisGain sets the chassis gain of the synthesizer.
// 0.0 is maximum gain and -60.0 will mute chassis bump haptic output.
func (c *Config) SetSynthChassisGain(value float64) {
	c.mu.Lock()
	c.viper.Synthesizer.ChassisGain = max(MinimumGain, min(MaximumGain, value))
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// IncreaseSynthChassisGain increases the chassis gain by the configured gain increment.
func (c *Config) IncreaseSynthChassisGain() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.ChassisGain = min(
		MaximumGain,
		c.viper.Synthesizer.ChassisGain+c.viper.Synthesizer.GainIncrement,
	)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.ChassisGain
	c.mu.Unlock()

	return result
}

// DecreaseSynthChassisGain decreases the chassis gain by the configured gain increment.
func (c *Config) DecreaseSynthChassisGain() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.ChassisGain = max(
		MinimumGain,
		c.viper.Synthesizer.ChassisGain-c.viper.Synthesizer.GainIncrement,
	)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.ChassisGain
	c.mu.Unlock()

	return result
}

// SetSynthTransmissionGainMinRace sets the minimum transmission gain for race transmissions.
func (c *Config) SetSynthTransmissionGainMinRace(value float64) {
	c.mu.Lock()
	c.viper.Synthesizer.TransmissionGainMinRace = max(MinimumGain, min(MaximumGain, value))
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// IncreaseSynthTransmissionGainMinRace increases the minimum race transmission gain by 0.25 (max 0).
func (c *Config) IncreaseSynthTransmissionGainMinRace() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.TransmissionGainMinRace = min(MaximumGain, c.viper.Synthesizer.TransmissionGainMinRace+0.25)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.TransmissionGainMinRace
	c.mu.Unlock()

	return result
}

// DecreaseSynthTransmissionGainMinRace decreases the minimum race transmission gain by 0.25 (min -60).
func (c *Config) DecreaseSynthTransmissionGainMinRace() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.TransmissionGainMinRace = max(MinimumGain, c.viper.Synthesizer.TransmissionGainMinRace-0.25)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.TransmissionGainMinRace
	c.mu.Unlock()

	return result
}

// SetSynthTransmissionGainMinStreet sets the minimum transmission gain for street transmissions.
func (c *Config) SetSynthTransmissionGainMinStreet(value float64) {
	c.mu.Lock()
	c.viper.Synthesizer.TransmissionGainMinStreet = max(MinimumGain, min(MaximumGain, value))
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// IncreaseSynthTransmissionGainMinStreet increases the minimum street transmission gain by 0.25 (max 0).
func (c *Config) IncreaseSynthTransmissionGainMinStreet() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.TransmissionGainMinStreet = min(MaximumGain, c.viper.Synthesizer.TransmissionGainMinStreet+0.25)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.TransmissionGainMinStreet
	c.mu.Unlock()

	return result
}

// DecreaseSynthTransmissionGainMinStreet decreases the minimum street transmission gain by 0.25 (min -60).
func (c *Config) DecreaseSynthTransmissionGainMinStreet() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.TransmissionGainMinStreet = max(MinimumGain, c.viper.Synthesizer.TransmissionGainMinStreet-0.25)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.TransmissionGainMinStreet
	c.mu.Unlock()

	return result
}

// GetSynthTransmissionGain returns the transmission gain of the synthesizer (i.e. the volume level for transmission
// haptics).
// 0.0 is maximum gain and -60.0 will mute transmission haptic output.
func (c *Config) GetSynthTransmissionGain() float64 {
	return c.snapshot.Load().TransmissionGain
}

// GetSynthTransmissionMute returns whether the transmission gain is muted.
func (c *Config) GetSynthTransmissionMute() bool {
	return c.snapshot.Load().TransmissionMute
}

// SetSynthTransmissionMute sets whether the transmission gain is muted.
func (c *Config) SetSynthTransmissionMute(mute bool) {
	c.mu.Lock()
	c.viper.Synthesizer.TransmissionMute = mute
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// SetSynthTransmissionGain sets the transmission gain of the synthesizer.
// 0.0 is maximum gain and -60.0 will mute transmission haptic output.
func (c *Config) SetSynthTransmissionGain(value float64) {
	c.mu.Lock()
	c.viper.Synthesizer.TransmissionGain = max(MinimumGain, min(MaximumGain, value))
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// IncreaseSynthTransmissionGain increases the transmission gain by the configured gain increment.
func (c *Config) IncreaseSynthTransmissionGain() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.TransmissionGain = min(
		MaximumGain,
		c.viper.Synthesizer.TransmissionGain+c.viper.Synthesizer.GainIncrement,
	)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.TransmissionGain
	c.mu.Unlock()

	return result
}

// DecreaseSynthTransmissionGain decreases the transmission gain by the configured gain increment.
func (c *Config) DecreaseSynthTransmissionGain() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.TransmissionGain = max(
		MinimumGain,
		c.viper.Synthesizer.TransmissionGain-c.viper.Synthesizer.GainIncrement,
	)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.TransmissionGain
	c.mu.Unlock()

	return result
}

// GetSynthTransmissionGainMinRace returns the minimum transmission gain for race vehicle types.
// This is the transnmission haptic feebdack volume applied when the vehicle is stationary.
func (c *Config) GetSynthTransmissionGainMinRace() float64 {
	return c.snapshot.Load().TransmissionGainMinRace
}

// GetSynthTransmissionGainMinStreet returns the minimum transmission gain for street vehicle types.
// This is the transmission haptic feedback volume applied when the vehicle is stationary.
func (c *Config) GetSynthTransmissionGainMinStreet() float64 {
	return c.snapshot.Load().TransmissionGainMinStreet
}

// GetSynthEngineGain returns the gain fir the currently selected engine (i.e. the volume level for engine haptics).
// 0.0 is maximum gain and -60.0 will mute engine haptic output.
func (c *Config) GetSynthEngineGain() float64 {
	return c.snapshot.Load().EngineGain
}

// GetSynthEngineMute returns whether the engine gain is muted.
func (c *Config) GetSynthEngineMute() bool {
	return c.snapshot.Load().EngineMute
}

// SetSynthEngineMute sets whether the engine gain is muted.
func (c *Config) SetSynthEngineMute(mute bool) {
	c.mu.Lock()
	c.viper.Synthesizer.EngineMute = mute
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// SetSynthEngineGain sets the engine gain of the synthesizer.
// 0.0 is maximum gain and -60.0 will mute engine haptic output.
func (c *Config) SetSynthEngineGain(gain float64) {
	c.mu.Lock()
	c.viper.Synthesizer.EngineGain = max(MinimumGain, min(MaximumGain, gain))
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// IncreaseSynthEngineGain increases the gain of the currently selected engine by the configured gain increment.
func (c *Config) IncreaseSynthEngineGain() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.EngineGain = min(
		MaximumGain,
		c.viper.Synthesizer.EngineGain+c.viper.Synthesizer.GainIncrement,
	)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.EngineGain
	c.mu.Unlock()

	return result
}

// DecreaseSynthEngineGain decreases the gain of the currently selected engine by the configured gain increment.
func (c *Config) DecreaseSynthEngineGain() float64 {
	c.mu.Lock()
	c.viper.Synthesizer.EngineGain = max(
		MinimumGain,
		c.viper.Synthesizer.EngineGain-c.viper.Synthesizer.GainIncrement,
	)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	result := c.viper.Synthesizer.EngineGain
	c.mu.Unlock()

	return result
}

// GetSynthOutputFile returns the synthesizer output file path.
func (c *Config) GetSynthOutputFile() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.Synthesizer.OutputFile
}

// GetSynthEngineProfiles returns all engine profiles as a map.
func (c *Config) GetSynthEngineProfiles() map[string]appHaptics.EngineProfile {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.Haptics.EngineProfiles
}

// SetSynthEngineProfile updates or creates an engine profile.
func (c *Config) SetSynthEngineProfile(name string, profile appHaptics.EngineProfile) {
	c.mu.Lock()
	defer c.mu.Unlock()

	name = strings.ToLower(name)
	c.viper.Haptics.EngineProfiles[name] = profile

	c.registerUpdate(false)
}

// GetSynthChannelEqEnabled returns whether the equalizer is enabled for a specific channel.
func (c *Config) GetSynthChannelEqEnabled(channel int) bool {
	snap := c.snapshot.Load()
	if channel >= 0 && channel < len(snap.EqEnabled) {
		return snap.EqEnabled[channel]
	}

	return false
}

// SetSynthChannelEqEnabled sets whether the equalizer is enabled for a specific channel.
func (c *Config) SetSynthChannelEqEnabled(channel int, enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if channel < 0 || channel >= len(c.viper.Synthesizer.EnableEq) {
		return
	}

	c.viper.Synthesizer.EnableEq[channel] = enabled
	c.rebuildSnapshot()
	c.registerUpdate(false)
}

// GetSynthChannelEq returns the equalizer bands for a specific channel.
func (c *Config) GetSynthChannelEq(channel int) []EQBand {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if channel < 0 || channel >= len(c.viper.Synthesizer.EqBands) {
		return nil
	}

	return c.viper.Synthesizer.EqBands[channel]
}

// SetSynthChannelEq sets the equalizer bands for a specific channel and recomputes the curve.
func (c *Config) SetSynthChannelEq(channel int, bands []EQBand) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if channel < 0 || channel >= len(c.viper.Synthesizer.EqBands) {
		log.Warn().Int("channel", channel).Int("eqBandsLen", len(c.viper.Synthesizer.EqBands)).Msg("SetSynthChannelEq: channel out of range")

		return
	}

	if len(bands) == 8 {
		c.viper.Synthesizer.EqBands[channel] = bands
		c.computeEqCurve(channel)
		c.registerUpdate(false)
	}
}

// GetSynthChannelEqCurve returns the computed EQ curve for fast lookup for a specific channel.
// Returns the curve, minimum frequency, and resolution (Hz per bucket).
func (c *Config) GetSynthChannelEqCurve(channel int) ([]float64, float64, float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if channel < 0 || channel >= len(c.viper.Synthesizer._eqCurve) {
		log.Warn().Int("channel", channel).Int("eqCurveLen", len(c.viper.Synthesizer._eqCurve)).Msg("GetSynthChannelEqCurve: channel out of range")

		return nil, 0, 0
	}

	return c.viper.Synthesizer._eqCurve[channel],
		c.viper.Synthesizer._eqMinFreq,
		c.viper.Synthesizer._eqResolution
}

// GetSynthChannelsEqEnabled returns the EQ enabled state for all channels.
func (c *Config) GetSynthChannelsEqEnabled() []bool {
	snap := c.snapshot.Load()
	result := make([]bool, len(snap.EqEnabled))
	copy(result, snap.EqEnabled)

	return result
}

// GetSynthChannelsEq returns the EQ bands for all channels.
func (c *Config) GetSynthChannelsEq() [][]EQBand {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([][]EQBand, len(c.viper.Synthesizer.EqBands))
	for ch, bands := range c.viper.Synthesizer.EqBands {
		result[ch] = make([]EQBand, len(bands))
		copy(result[ch], bands)
	}

	return result
}

// GetSynthChannelsEqCurve returns the computed EQ curves for all channels.
// Returns the curves, minimum frequency, and resolution (Hz per bucket).
func (c *Config) GetSynthChannelsEqCurve() ([][]float64, float64, float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([][]float64, len(c.viper.Synthesizer._eqCurve))
	for ch, curve := range c.viper.Synthesizer._eqCurve {
		result[ch] = make([]float64, len(curve))
		copy(result[ch], curve)
	}

	return result, c.viper.Synthesizer._eqMinFreq, c.viper.Synthesizer._eqResolution
}

// ****************************************************************************
// Telemetry methods.
// ****************************************************************************

// GetTelemetrySource returns the configured telemetry source.
func (c *Config) GetTelemetrySource() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.viper.Telemetry.Source
}

// SetTelemetrySource sets the telemetry source.
func (c *Config) SetTelemetrySource(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// TODO: validate value

	c.viper.Telemetry.Source = value

	c.registerUpdate(true)
}

// ****************************************************************************
// Configuration file management methods.
// ****************************************************************************

// BackupConfigFile creates a backup of the current configuration file.
// Returns the backup filename and any error encountered.
func (c *Config) BackupConfigFile() (string, error) {
	// If the file doesn't exist, nothing to back up
	_, err := os.Stat(c.configFile)
	if err != nil {
		return "", fmt.Errorf("configuration file %s not found", c.configFile)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s.backup.%s", c.configFile, timestamp)

	source, err := os.Open(c.configFile)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close()

	destination, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	return backupPath, nil
}

// SaveConfigToFile saves the current configuration to the configuration file.
// Skips writing if the config hasn't changed to reduce disk wear.
func (c *Config) SaveConfigToFile() error {
	// If no config file was specified, we can't save
	if c.configFile == "" {
		return errors.New("no config file specified")
	}

	// Lock for the entire save operation to serialize concurrent saves
	c.mu.Lock()
	defer c.mu.Unlock()

	// Marshal the configuration to JSON
	jsonData, err := json.MarshalIndent(c.viper, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration to JSON: %w", err)
	}

	// Skip write if config hasn't changed (reduces SD card wear)
	if bytes.Equal(jsonData, c.lastSavedConfig) {
		return nil
	}

	// Ensure the directory exists
	configDir := filepath.Dir(c.configFile)

	err = os.MkdirAll(configDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", configDir, err)
	}

	// Write directly to config file with fsync to ensure durability
	file, err := os.OpenFile(c.configFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open config file %s: %w", c.configFile, err)
	}

	_, err = file.Write(jsonData)
	if err != nil {
		file.Close()

		return fmt.Errorf("failed to write config file %s: %w", c.configFile, err)
	}

	// Force sync to disk before closing
	err = file.Sync()
	if err != nil {
		file.Close()

		return fmt.Errorf("failed to sync config file %s: %w", c.configFile, err)
	}

	err = file.Close()
	if err != nil {
		return fmt.Errorf("failed to close config file %s: %w", c.configFile, err)
	}

	// Update last saved config
	c.lastSavedConfig = jsonData

	return nil
}

// ****************************************************************************
// Public Helper methods.
// ****************************************************************************

// GetConfigFilePath returns the path to the configuration file.
func (c *Config) GetConfigFilePath() string {
	return c.configFile
}

// ****************************************************************************
// Private Helper methods.
// ****************************************************************************

// finalise performs validation of the config and updates any derived configuration values.
func (c *Config) finalise() {
	c.mu.Lock()

	// Ensure we have exactly 2 channels of EQ bands
	numChannels := 2
	if len(c.viper.Synthesizer.EqBands) != numChannels {
		log.Warn().Int("length", len(c.viper.Synthesizer.EqBands)).Msg("invalid synthesizer EQ bands length, initializing defaults")

		// Initialize with default bands for each channel
		defaultBands := []EQBand{
			{Frequency: 12, Gain: 0.0, Q: 2.0},
			{Frequency: 16, Gain: 0.0, Q: 2.0},
			{Frequency: 20, Gain: 0.0, Q: 2.0},
			{Frequency: 25, Gain: 0.0, Q: 2.0},
			{Frequency: 30, Gain: 0.0, Q: 2.0},
			{Frequency: 38, Gain: 0.0, Q: 2.0},
			{Frequency: 48, Gain: 0.0, Q: 2.0},
			{Frequency: 58, Gain: 0.0, Q: 2.0},
		}

		c.viper.Synthesizer.EqBands = make([][]EQBand, numChannels)
		for ch := range numChannels {
			c.viper.Synthesizer.EqBands[ch] = make([]EQBand, len(defaultBands))
			copy(c.viper.Synthesizer.EqBands[ch], defaultBands)
		}
	}

	// Validate each channel has 8 bands
	for ch := range c.viper.Synthesizer.EqBands {
		if len(c.viper.Synthesizer.EqBands[ch]) != 8 {
			log.Warn().Int("channel", ch).Int("length", len(c.viper.Synthesizer.EqBands[ch])).Msg("invalid EQ bands length for channel, initializing defaults")

			c.viper.Synthesizer.EqBands[ch] = []EQBand{
				{Frequency: 12, Gain: 0.0, Q: 2.0},
				{Frequency: 16, Gain: 0.0, Q: 2.0},
				{Frequency: 20, Gain: 0.0, Q: 2.0},
				{Frequency: 25, Gain: 0.0, Q: 2.0},
				{Frequency: 30, Gain: 0.0, Q: 2.0},
				{Frequency: 38, Gain: 0.0, Q: 2.0},
				{Frequency: 48, Gain: 0.0, Q: 2.0},
				{Frequency: 58, Gain: 0.0, Q: 2.0},
			}
		}
	}

	// Ensure we have 2 channels of EQ enabled flags
	if len(c.viper.Synthesizer.EnableEq) < numChannels {
		c.viper.Synthesizer.EnableEq = make([]bool, numChannels)
	}

	// Compute the EQ curve for each channel
	for ch := range c.viper.Synthesizer.EqBands {
		c.computeEqCurve(ch)
	}

	// Update pulse width extents (inline since we hold the lock)
	c.viper.Haptics._pulseWidthMin = float64(c.viper.Synthesizer.InternalSampleRateHz) /
		(2 * c.viper.Haptics.PulseMaxFrequencyHz)
	c.viper.Haptics._pulseWidthMax = float64(c.viper.Synthesizer.InternalSampleRateHz) /
		(2 * c.viper.Haptics.PulseMinFrequencyHz)

	c.mu.Unlock()

	c.updateJerkScale()
	c.updateSnapScale()
}

// updateJerkScale recalculates the jerk scale factor based on the current jerk curve, scale and maximum.
func (c *Config) updateJerkScale() {
	c.mu.Lock()
	exponent := float64(c.viper.Haptics.JerkCurve) / 1000.0
	jerkMax := 100 * float64(c.viper.Haptics.JerkMax)
	c.viper.Haptics._jerkScale = 1 / math.Pow(jerkMax, exponent)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// updateSnapScale recalculates the snap scale factor based on the current snap curve, scale and maximum.
func (c *Config) updateSnapScale() {
	c.mu.Lock()
	exponent := float64(c.viper.Haptics.SnapCurve) / 1000.0
	snapMax := 1000 * float64(c.viper.Haptics.SnapMax)
	c.viper.Haptics._snapScale = 1 / math.Pow(snapMax, exponent)
	c.rebuildSnapshot()
	c.registerUpdate(false)
	c.mu.Unlock()
}

// computeEqCurve computes the EQ curve for a specific channel based on its bands.
// Uses 8-band parametric EQ with bell filters.
func (c *Config) computeEqCurve(channel int) {
	const (
		minFreqHz    = 5.0
		maxFreqHz    = 160.0
		resolutionHz = 0.5
	)

	if channel < 0 || channel >= len(c.viper.Synthesizer.EqBands) {
		return
	}

	numBuckets := int((maxFreqHz-minFreqHz)/resolutionHz) + 1
	curve := make([]float64, numBuckets)

	// For each frequency bucket, compute the EQ response using bell filter
	for bucketNum := range numBuckets {
		freq := minFreqHz + float64(bucketNum)*resolutionHz

		// Start with unity gain (1.0 in linear, 0.0 in dB)
		amplitudeRatio := 1.0

		// Apply each band's bell filter by multiplication in linear space
		for _, band := range c.viper.Synthesizer.EqBands[channel] {
			// Calculate bell filter response at this frequency
			// Using per-band Q factor for bandwidth control
			if band.Gain != 0.0 {
				// Use band's qFactor value, default to 2.0 if not set
				qFactor := band.Q
				if qFactor <= 0 {
					qFactor = 2.0
				}

				freqRatio := freq / band.Frequency
				if freqRatio > 0 {
					// Bell filter magnitude response in dB
					// H(f) = G / sqrt(1 + Q^2 * (f/fc - fc/f)^2)
					// At center frequency (f = fc), delta = 0, denom = 1, so gain = G (exact)
					delta := freqRatio - 1.0/freqRatio
					denom := math.Sqrt(1.0 + qFactor*qFactor*delta*delta)

					if denom > 0 {
						// Calculate this band's gain at this frequency in dB
						bandGainDB := band.Gain / denom
						// Convert to amplitude ratio and multiply
						amplitudeRatio *= math.Pow(10, bandGainDB/20)
					}
				}
			}
		}

		// Store the final amplitude ratio for efficient multiplication
		curve[bucketNum] = amplitudeRatio
	}

	// Ensure the curves slice is large enough
	for len(c.viper.Synthesizer._eqCurve) <= channel {
		c.viper.Synthesizer._eqCurve = append(c.viper.Synthesizer._eqCurve, nil)
	}

	c.viper.Synthesizer._eqCurve[channel] = curve
	c.viper.Synthesizer._eqMinFreq = minFreqHz
	c.viper.Synthesizer._eqMaxFreq = maxFreqHz
	c.viper.Synthesizer._eqResolution = resolutionHz
}

// registerUpdate records the time of the last configuration update.
// Assumes that the caller holds the write lock.
func (c *Config) registerUpdate(restartRequired bool) {
	c.status.LastUpdate = time.Now().Unix()

	if restartRequired {
		c.status.RestartRequired = restartRequired
	}
}

// rebuildSnapshot creates a new immutable snapshot for lock-free reads.
// Assumes that the caller holds the write lock.
func (c *Config) rebuildSnapshot() {
	// Copy channel arrays
	channelMute := make([]bool, len(c.viper.Synthesizer.ChannelMute))
	copy(channelMute, c.viper.Synthesizer.ChannelMute)

	channelGain := make([]float64, len(c.viper.Synthesizer.ChannelGain))
	copy(channelGain, c.viper.Synthesizer.ChannelGain)

	newSnap := &Snapshot{
		MasterMute:                c.viper.Synthesizer.MasterMute,
		MasterGain:                c.viper.Synthesizer.MasterGain,
		ChannelMute:               channelMute,
		ChannelGain:               channelGain,
		ChassisMute:               c.viper.Synthesizer.ChassisMute,
		ChassisGain:               c.viper.Synthesizer.ChassisGain,
		TransmissionMute:          c.viper.Synthesizer.TransmissionMute,
		TransmissionGain:          c.viper.Synthesizer.TransmissionGain,
		TransmissionGainMinRace:   c.viper.Synthesizer.TransmissionGainMinRace,
		TransmissionGainMinStreet: c.viper.Synthesizer.TransmissionGainMinStreet,
		EngineMute:                c.viper.Synthesizer.EngineMute,
		EngineGain:                c.viper.Synthesizer.EngineGain,
		GainIncrement:             c.viper.Synthesizer.GainIncrement,
		InternalSampleRateHz:      c.viper.Synthesizer.InternalSampleRateHz,
		OutputSampleRateHz:        c.viper.Synthesizer.OutputSampleRateHz,

		JerkCurve: float64(c.viper.Haptics.JerkCurve),
		JerkMax:   c.viper.Haptics.JerkMax,
		JerkScale: c.viper.Haptics._jerkScale,

		SnapCurve: float64(c.viper.Haptics.SnapCurve),
		SnapMax:   c.viper.Haptics.SnapMax,
		SnapScale: c.viper.Haptics._snapScale,

		PulseMaxAmplitude:   c.viper.Haptics.PulseMaxAmplitude,
		PulseMaxFrequencyHz: c.viper.Haptics.PulseMaxFrequencyHz,
		PulseMinFrequencyHz: c.viper.Haptics.PulseMinFrequencyHz,
		PulseWidthMin:       c.viper.Haptics._pulseWidthMin,
		PulseWidthMax:       c.viper.Haptics._pulseWidthMax,

		DynamicTransmissionFeedback:  c.viper.Haptics.DynamicTransmissionFeedback,
		DynamicTransmissionCurve:     c.viper.Haptics.DynamicTransmissionCurve,
		DynamicTransmissionGforceMax: c.viper.Haptics.DynamicTransmissionGforceMax,

		EqEnabled: func() []bool {
			eqEnabled := make([]bool, len(c.viper.Synthesizer.EnableEq))
			copy(eqEnabled, c.viper.Synthesizer.EnableEq)

			return eqEnabled
		}(),

		FuelMonitoringEnabled: c.viper.PitRadio.FuelMonitoring.Enabled,
		TyreMonitoringEnabled: c.viper.PitRadio.TyreMonitoring.Enabled,

		DisplayOrientation: c.viper.Hardware.DisplayOrientation,
	}
	c.snapshot.Store(newSnap)
}

// syncEngineProfileToMap copies the current engine profile pointer back to the map.
// This must be called after modifying _engineProfile to ensure changes are persisted.
// Caller must hold the mutex.
func (c *Config) syncEngineProfileToMap() {
	if c.viper.Haptics._engineProfile != nil && c.viper.Haptics._engineProfileName != "" {
		c.viper.Haptics.EngineProfiles[c.viper.Haptics._engineProfileName] = *c.viper.Haptics._engineProfile
	}
}

// updatePulseWidthExtents recalculates the minimum and maximum pulse widths in samples.
// Assumes the caller does NOT hold the lock.
func (c *Config) updatePulseWidthExtents() {
	// Assumes caller holds c.mu.Lock()
	c.viper.Haptics._pulseWidthMin = float64(c.viper.Synthesizer.InternalSampleRateHz) /
		(2 * c.viper.Haptics.PulseMaxFrequencyHz)

	c.viper.Haptics._pulseWidthMax = float64(c.viper.Synthesizer.InternalSampleRateHz) /
		(2 * c.viper.Haptics.PulseMinFrequencyHz)

	c.rebuildSnapshot()
	c.registerUpdate(false)
}

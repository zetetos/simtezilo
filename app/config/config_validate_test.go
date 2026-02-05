package config //nolint:testpackage // white-box testing for internal validation functions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationResult_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   ValidationResult
		expected string
	}{
		{
			name:     "valid result returns empty string",
			result:   ValidationResult{Valid: true, Errors: nil},
			expected: "",
		},
		{
			name: "single error",
			result: ValidationResult{
				Valid: false,
				Errors: []ValidationError{
					{Field: "app.language", Message: "language code is required"},
				},
			},
			expected: "app.language: language code is required",
		},
		{
			name: "multiple errors",
			result: ValidationResult{
				Valid: false,
				Errors: []ValidationError{
					{Field: "app.language", Message: "language code is required"},
					{Field: "app.webUIPort", Message: "port out of range"},
				},
			},
			expected: "app.language: language code is required; app.webUIPort: port out of range",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.expected, testCase.result.Error())
		})
	}
}

func TestValidateJSONSchema_ValidConfig(t *testing.T) {
	t.Parallel()

	validJSON := `{
		"schemaVersion": "1.0.0",
		"app": {
			"language": "en",
			"logLevel": "info",
			"webUIPort": 8080
		},
		"hardware": {
			"model": "console",
			"displayOrientation": 90
		},
		"telemetry": {
			"source": "gt7"
		}
	}`

	result := validateJSONSchema([]byte(validJSON))
	assert.True(t, result.Valid, "expected valid config, got errors: %v", result.Errors)
}

func TestValidateJSONSchema_InvalidSchemaVersion(t *testing.T) {
	t.Parallel()

	invalidJSON := `{
		"schemaVersion": "2.0.0"
	}`

	result := validateJSONSchema([]byte(invalidJSON))
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateJSONSchema_InvalidLogLevel(t *testing.T) {
	t.Parallel()

	invalidJSON := `{
		"schemaVersion": "1.0.0",
		"app": {
			"language": "en",
			"logLevel": "invalid_level"
		}
	}`

	result := validateJSONSchema([]byte(invalidJSON))
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
	// Check that the error mentions the field
	hasLogLevelError := false

	for _, err := range result.Errors {
		if err.Field == "app.logLevel" {
			hasLogLevelError = true

			break
		}
	}

	assert.True(t, hasLogLevelError, "expected error for app.logLevel, got: %v", result.Errors)
}

func TestValidateJSONSchema_InvalidWebUIPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		port int
	}{
		{"port too low", 0},
		{"port too high", 65536},
		{"port negative", -1},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			invalidJSON := `{"schemaVersion": "1.0.0", "app": {"webUIPort": ` + intToString(testCase.port) + `}}`
			result := validateJSONSchema([]byte(invalidJSON))
			assert.False(t, result.Valid, "expected invalid for port %d", testCase.port)
		})
	}
}

func TestValidateJSONSchema_InvalidHardwareModel(t *testing.T) {
	t.Parallel()

	invalidJSON := `{
		"schemaVersion": "1.0.0",
		"hardware": {
			"model": "invalid_model"
		}
	}`

	result := validateJSONSchema([]byte(invalidJSON))
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateJSONSchema_InvalidDisplayOrientation(t *testing.T) {
	t.Parallel()

	invalidJSON := `{
		"schemaVersion": "1.0.0",
		"hardware": {
			"displayOrientation": 45
		}
	}`

	result := validateJSONSchema([]byte(invalidJSON))
	assert.False(t, result.Valid)
}

func TestValidateJSONSchema_SynthesizerGainOutOfRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		value float64
	}{
		{"masterGain too low", "masterGain", -70.0},
		{"masterGain too high", "masterGain", 1.0},
		{"chassisGain too low", "chassisGain", -65.0},
		{"engineGain too high", "engineGain", 5.0},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			invalidJSON := `{
				"schemaVersion": "1.0.0",
				"synthesizer": {
					"` + testCase.field + `": ` + floatToString(testCase.value) + `
				}
			}`
			result := validateJSONSchema([]byte(invalidJSON))
			assert.False(t, result.Valid, "expected invalid for %s=%f", testCase.field, testCase.value)
		})
	}
}

func TestValidateJSONSchema_SampleRateOutOfRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		value int
	}{
		{"internalSampleRate too low", "internalSampleRateHz", 4000},
		{"internalSampleRate too high", "internalSampleRateHz", 200000},
		{"outputSampleRate too low", "outputSampleRateHz", 1000},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			invalidJSON := `{
				"schemaVersion": "1.0.0",
				"synthesizer": {
					"` + testCase.field + `": ` + intToString(testCase.value) + `
				}
			}`
			result := validateJSONSchema([]byte(invalidJSON))
			assert.False(t, result.Valid, "expected invalid for %s=%d", testCase.field, testCase.value)
		})
	}
}

func TestValidateJSONSchema_HapticsOutOfRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		json  string
	}{
		{"jerkCurve too low", "jerkCurve", `"jerkCurve": 2`},
		{"jerkCurve too high", "jerkCurve", `"jerkCurve": 1000`},
		{"jerkMax too low", "jerkMax", `"jerkMax": 0`},
		{"jerkMax too high", "jerkMax", `"jerkMax": 250`},
		{"snapCurve too low", "snapCurve", `"snapCurve": 1`},
		{"snapMax too high", "snapMax", `"snapMax": 201`},
		{"pulseMaxAmplitude too high", "pulseMaxAmplitude", `"pulseMaxAmplitude": 1.5`},
		{"pulseMaxAmplitude negative", "pulseMaxAmplitude", `"pulseMaxAmplitude": -0.1`},
		{"dynamicTransmissionCurve too low", "dynamicTransmissionCurve", `"dynamicTransmissionCurve": 3`},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			invalidJSON := `{
				"schemaVersion": "1.0.0",
				"haptics": {
					` + testCase.json + `
				}
			}`
			result := validateJSONSchema([]byte(invalidJSON))
			assert.False(t, result.Valid, "expected invalid for %s", testCase.name)
		})
	}
}

func TestValidateJSONSchema_FuelMonitoringNonNegative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
	}{
		{"preWarnNotifyLaps", "preWarnNotifyLaps"},
		{"strategyNotifyLaps", "strategyNotifyLaps"},
		{"rangeSafetyMarginLaps", "rangeSafetyMarginLaps"},
		{"rangeSafetyMarginMetres", "rangeSafetyMarginMetres"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			invalidJSON := `{
				"schemaVersion": "1.0.0",
				"pitRadio": {
					"fuelMonitoring": {
						"` + testCase.field + `": -1.0
					}
				}
			}`
			result := validateJSONSchema([]byte(invalidJSON))
			assert.False(t, result.Valid, "expected invalid for negative %s", testCase.field)
		})
	}
}

func TestValidateJSONSchema_TyreMonitoringRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
	}{
		{"temperatureOptimalCelsius too high", `"temperatureOptimalCelsius": 250`},
		{"temperatureOptimalCelsius negative", `"temperatureOptimalCelsius": -10`},
		{"temperatureOperatingWindow too high", `"temperatureOperatingWindow": 150`},
		{"temperatureMarginCelsius too high", `"temperatureMarginCelsius": 60`},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			invalidJSON := `{
				"schemaVersion": "1.0.0",
				"pitRadio": {
					"tyreMonitoring": {
						` + testCase.json + `
					}
				}
			}`
			result := validateJSONSchema([]byte(invalidJSON))
			assert.False(t, result.Valid, "expected invalid for %s", testCase.name)
		})
	}
}

func TestValidateJSONSchema_EQBandsArrayLength(t *testing.T) {
	t.Parallel()

	// Test wrong number of channels
	invalidJSON := `{
		"schemaVersion": "1.0.0",
		"synthesizer": {
			"eqBands": [
				[{"frequency": 20, "gain": 0, "q": 1}]
			]
		}
	}`

	result := validateJSONSchema([]byte(invalidJSON))
	assert.False(t, result.Valid, "expected invalid for single channel eqBands")
}

func TestValidateJSONSchema_EQBandFrequencyRange(t *testing.T) {
	t.Parallel()

	invalidJSON := `{
		"schemaVersion": "1.0.0",
		"synthesizer": {
			"eqBands": [
				[
					{"frequency": 5, "gain": 0, "q": 1},
					{"frequency": 20, "gain": 0, "q": 1},
					{"frequency": 30, "gain": 0, "q": 1},
					{"frequency": 40, "gain": 0, "q": 1},
					{"frequency": 50, "gain": 0, "q": 1},
					{"frequency": 60, "gain": 0, "q": 1},
					{"frequency": 65, "gain": 0, "q": 1},
					{"frequency": 200, "gain": 0, "q": 1}
				],
				[
					{"frequency": 20, "gain": 0, "q": 1},
					{"frequency": 25, "gain": 0, "q": 1},
					{"frequency": 30, "gain": 0, "q": 1},
					{"frequency": 35, "gain": 0, "q": 1},
					{"frequency": 40, "gain": 0, "q": 1},
					{"frequency": 45, "gain": 0, "q": 1},
					{"frequency": 50, "gain": 0, "q": 1},
					{"frequency": 55, "gain": 0, "q": 1}
				]
			]
		}
	}`

	result := validateJSONSchema([]byte(invalidJSON))
	assert.False(t, result.Valid, "expected invalid for out-of-range EQ frequency")
}

func TestValidateJSONSchema_InvalidJSON(t *testing.T) {
	t.Parallel()

	invalidJSON := `{not valid json}`

	result := validateJSONSchema([]byte(invalidJSON))
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateAppCustom_BaseDirExists(t *testing.T) {
	t.Parallel()

	// Create a temp directory
	tempDir := t.TempDir()

	result := &ValidationResult{Valid: true, Errors: []ValidationError{}}
	appCfg := &app{BaseDir: tempDir}
	validateAppCustom(appCfg, result)

	assert.Empty(t, result.Errors, "expected no errors for existing directory")
}

func TestValidateAppCustom_BaseDirNotExists(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{Valid: true, Errors: []ValidationError{}}
	appCfg := &app{BaseDir: "/nonexistent/path/that/does/not/exist"}
	validateAppCustom(appCfg, result)

	assert.NotEmpty(t, result.Errors, "expected error for non-existent directory")
	assert.Equal(t, "app.baseDir", result.Errors[0].Field)
}

func TestValidateAppCustom_EmptyBaseDir(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{Valid: true, Errors: []ValidationError{}}
	appCfg := &app{BaseDir: ""}
	validateAppCustom(appCfg, result)

	assert.Empty(t, result.Errors, "expected no errors for empty baseDir")
}

func TestValidateHapticsCustom_FrequencyRelationship(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		minFreq     float64
		maxFreq     float64
		expectError bool
	}{
		{"valid: min < max", 10.0, 60.0, false},
		{"invalid: min == max", 50.0, 50.0, true},
		{"invalid: min > max", 60.0, 10.0, true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := &ValidationResult{Valid: true, Errors: []ValidationError{}}
			hap := &haptics{
				PulseMinFrequencyHz: testCase.minFreq,
				PulseMaxFrequencyHz: testCase.maxFreq,
			}
			validateHapticsCustom(hap, result)

			if testCase.expectError {
				assert.NotEmpty(t, result.Errors)
			} else {
				assert.Empty(t, result.Errors)
			}
		})
	}
}

func TestValidateDiscordCustom_GuildIDRequired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		token       string
		guildID     string
		expectError bool
	}{
		{"no token, no guildID", "", "", false},
		{"token set, guildID set", "token123", "guild456", false},
		{"token set, no guildID", "token123", "", true},
		{"no token, guildID set", "", "guild456", false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := &ValidationResult{Valid: true, Errors: []ValidationError{}}
			d := &discord{
				Token:   testCase.token,
				GuildID: testCase.guildID,
			}
			validateDiscordCustom(d, result)

			if testCase.expectError {
				assert.NotEmpty(t, result.Errors)
				assert.Equal(t, "pitRadio.discord.guildID", result.Errors[0].Field)
			} else {
				assert.Empty(t, result.Errors)
			}
		})
	}
}

func TestGetCompiledSchema_Caching(t *testing.T) { //nolint:paralleltest // intentionally sequential to test singleton
	// First call should compile the schema
	schema1, err1 := getCompiledSchema()
	require.NoError(t, err1)
	require.NotNil(t, schema1)

	// Second call should return the same cached instance
	schema2, err2 := getCompiledSchema()
	require.NoError(t, err2)
	require.NotNil(t, schema2)

	// They should be the same pointer (cached)
	assert.Same(t, schema1, schema2)
}

func TestExtractValidationErrors(t *testing.T) {
	t.Parallel()

	validJSON := `{
		"schemaVersion": "1.0.0",
		"app": {
			"logLevel": "invalid"
		}
	}`

	// Test that nested errors are properly extracted
	result := validateJSONSchema([]byte(validJSON))
	assert.False(t, result.Valid)

	// Should have extracted the error with proper field path
	foundError := false

	for _, err := range result.Errors {
		if err.Field == "app.logLevel" {
			foundError = true

			break
		}
	}

	assert.True(t, foundError, "expected to find app.logLevel error")
}

func TestValidateConfig_ValidJSON(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	validJSON := `{
		"schemaVersion": "1.0.0",
		"app": {
			"language": "en",
			"logLevel": "info",
			"webUIPort": 8080
		},
		"hardware": {
			"model": "console",
			"displayOrientation": 90
		},
		"telemetry": {
			"source": "gt7"
		},
		"haptics": {
			"pulseMinFrequencyHz": 10,
			"pulseMaxFrequencyHz": 60
		}
	}`

	result := cfg.ValidateConfig([]byte(validJSON))
	assert.True(t, result.Valid, "expected valid config, got errors: %v", result.Errors)
}

func TestValidateConfig_InvalidJSON(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	invalidJSON := `{not valid json}`

	result := cfg.ValidateConfig([]byte(invalidJSON))
	assert.False(t, result.Valid)
}

func TestValidateConfig_SchemaViolation(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	invalidJSON := `{
		"schemaVersion": "1.0.0",
		"app": {
			"webUIPort": 100000
		}
	}`

	result := cfg.ValidateConfig([]byte(invalidJSON))
	assert.False(t, result.Valid)
}

func TestConfig_Validate_Integration(t *testing.T) {
	t.Parallel()

	// Create a temp directory for baseDir
	tempDir := t.TempDir()

	cfg := &Config{
		viper: &viperConfig{
			SchemaVersion: "1.0.0",
			App: &app{
				Language:  "en",
				LogLevel:  "info",
				BaseDir:   tempDir,
				WebUIPort: 8080,
			},
			Hardware: &hardware{
				Model:              "console",
				DisplayOrientation: 90,
			},
			Telemetry: &Telemetry{
				Source: "gt7",
			},
			Haptics: &haptics{
				JerkCurve:                    190,
				JerkMax:                      37,
				SnapCurve:                    310,
				SnapMax:                      90,
				PulseMinFrequencyHz:          10,
				PulseMaxFrequencyHz:          60,
				PulseMaxAmplitude:            1.0,
				DynamicTransmissionCurve:     150,
				DynamicTransmissionGforceMax: 2.0,
			},
			Synthesizer: &Synthesizer{
				InternalSampleRateHz:      8000,
				OutputSampleRateHz:        32000,
				MasterGain:                -7.0,
				ChassisGain:               0.0,
				EngineGain:                -3.5,
				TransmissionGain:          0.0,
				TransmissionGainMinRace:   -4.5,
				TransmissionGainMinStreet: -6.0,
				GainIncrement:             0.25,
				ChannelGain:               []float64{-5.5, -5.5},
				ChannelMute:               []bool{false, false},
				EnableEq:                  []bool{false, false},
			},
		},
	}

	result := cfg.Validate()
	assert.True(t, result.Valid, "expected valid config, got errors: %v", result.Errors)
}

func TestValidateJSONSchema_ValidHardwareModels(t *testing.T) {
	t.Parallel()

	validModels := []string{"none", "console", "pirateaudio", "spotpear", "waveshare"}

	for _, model := range validModels {
		t.Run(model, func(t *testing.T) {
			t.Parallel()

			validJSON := `{
				"schemaVersion": "1.0.0",
				"hardware": {
					"model": "` + model + `"
				}
			}`
			result := validateJSONSchema([]byte(validJSON))
			assert.True(t, result.Valid, "expected valid for model %s, got errors: %v", model, result.Errors)
		})
	}
}

func TestValidateJSONSchema_ValidLogLevels(t *testing.T) {
	t.Parallel()

	validLevels := []string{"trace", "debug", "info", "warn", "error", "fatal", "panic"}

	for _, level := range validLevels {
		t.Run(level, func(t *testing.T) {
			t.Parallel()

			validJSON := `{
				"schemaVersion": "1.0.0",
				"app": {
					"language": "en",
					"logLevel": "` + level + `"
				}
			}`
			result := validateJSONSchema([]byte(validJSON))
			assert.True(t, result.Valid, "expected valid for logLevel %s, got errors: %v", level, result.Errors)
		})
	}
}

func TestValidateJSONSchema_ValidDisplayOrientations(t *testing.T) {
	t.Parallel()

	validOrientations := []int{0, 90, 180, 270}

	for _, orientation := range validOrientations {
		t.Run(intToString(orientation), func(t *testing.T) {
			t.Parallel()

			validJSON := `{
				"schemaVersion": "1.0.0",
				"hardware": {
					"displayOrientation": ` + intToString(orientation) + `
				}
			}`
			result := validateJSONSchema([]byte(validJSON))
			assert.True(t, result.Valid, "expected valid for displayOrientation %d, got errors: %v", orientation, result.Errors)
		})
	}
}

func TestValidateJSONSchema_PitRadioMessageInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		value       int
		expectValid bool
	}{
		{"zero is valid", 0, true},
		{"positive is valid", 5000, true},
		{"max is valid", 60000, true},
		{"negative is invalid", -1, false},
		{"above max is invalid", 60001, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			jsonStr := `{
				"schemaVersion": "1.0.0",
				"pitRadio": {
					"messageSendIntervalMs": ` + intToString(testCase.value) + `
				}
			}`
			result := validateJSONSchema([]byte(jsonStr))
			assert.Equal(t, testCase.expectValid, result.Valid, "for value %d, got errors: %v", testCase.value, result.Errors)
		})
	}
}

func TestValidateJSONSchema_MissingRequiredField(t *testing.T) {
	t.Parallel()

	// Missing schemaVersion which is required
	invalidJSON := `{
		"app": {
			"language": "en"
		}
	}`

	result := validateJSONSchema([]byte(invalidJSON))
	assert.False(t, result.Valid)
}

func TestValidateJSONSchema_TelemetrySourceRequired(t *testing.T) {
	t.Parallel()

	// telemetry.source is required per schema
	invalidJSON := `{
		"schemaVersion": "1.0.0",
		"telemetry": {}
	}`

	result := validateJSONSchema([]byte(invalidJSON))
	assert.False(t, result.Valid)
}

func TestCreateTempConfigFile(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for config
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	validConfig := `{
		"schemaVersion": "1.0.0",
		"app": {
			"language": "en",
			"logLevel": "info",
			"baseDir": "` + tempDir + `",
			"webUIPort": 8080
		}
	}`

	err := os.WriteFile(configPath, []byte(validConfig), 0o600)
	require.NoError(t, err)

	// Read back and validate
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	cfg := &Config{}
	result := cfg.ValidateConfig(data)
	assert.True(t, result.Valid, "expected valid config from file, got errors: %v", result.Errors)
}

// Helper function: int to string conversion for tests.
func intToString(intValue int) string {
	if intValue == 0 {
		return "0"
	}

	if intValue < 0 {
		return "-" + intToString(-intValue)
	}

	result := ""

	for intValue > 0 {
		result = string(rune('0'+intValue%10)) + result
		intValue /= 10
	}

	return result
}

// Helper function: float to string conversion for tests.
func floatToString(floatValue float64) string {
	if floatValue == 0 {
		return "0"
	}

	// Handle negative
	prefix := ""

	if floatValue < 0 {
		prefix = "-"
		floatValue = -floatValue
	}

	// Integer part
	intPart := int(floatValue)
	fracPart := floatValue - float64(intPart)

	// Simple conversion
	result := intToString(intPart)

	// Add decimal part if non-zero
	if fracPart > 0.001 {
		result += "."
		// Just one decimal place for our tests
		fracDigit := int(fracPart * 10)
		result += intToString(fracDigit)
	} else {
		result += ".0"
	}

	return prefix + result
}

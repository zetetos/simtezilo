package config //nolint:testpackage // white-box testing for internal config methods

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appHaptics "github.com/zetetos/simtezilo/app/haptics"
)

// newTestConfig creates a new Config instance with default values for testing.
func newTestConfig() *Config {
	config := &Config{
		viper:      defaultConfig(),
		configFile: "",
		status: Status{
			RestartRequired: false,
			LastUpdate:      0,
		},
	}
	config.finalise()
	config.rebuildSnapshot()

	return config
}

// newTestConfigFromJSON creates a new Config instance from JSON for testing.
func newTestConfigFromJSON(json string) *Config {
	return NewFromJSON([]byte(json), zerolog.Nop())
}

// ****************************************************************************
// App section tests
// ****************************************************************************

func testAppAccentGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default value
	assert.Equal(t, "us", cfg.GetAppAccent(), 0.001)

	// Act - set new value
	cfg.SetAppAccent("uk")

	// Assert
	assert.Equal(t, "uk", cfg.GetAppAccent(), 0.001)
}

func testAppAccentDefaultWhenEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.viper.App.Accent = ""

	// Act & Assert
	assert.Equal(t, "us", cfg.GetAppAccent(), 0.001)
}

func testAppBaseDirGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default value
	assert.Equal(t, ".", cfg.GetAppBaseDir(), 0.001)

	// Act - set new value
	cfg.SetAppBaseDir("/opt/simtezilo")

	// Assert
	assert.Equal(t, "/opt/simtezilo", cfg.GetAppBaseDir(), 0.001)
}

func testAppBaseDirDefaultWhenEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.viper.App.BaseDir = ""

	// Act & Assert
	assert.Equal(t, ".", cfg.GetAppBaseDir(), 0.001)
}

func testAppVehicleDBFileGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set value
	cfg.SetAppVehicleDBFile("/data/vehicles.db")

	// Assert
	assert.Equal(t, "/data/vehicles.db", cfg.GetAppVehicleDBFile(), 0.001)
}

func testAppLanguageGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default value
	assert.Equal(t, "en", *cfg.GetAppLanguage(), 0.001)

	// Act - set new value
	cfg.SetAppLanguage("de")

	// Assert
	assert.Equal(t, "de", *cfg.GetAppLanguage(), 0.001)
}

func testAppLanguageDefaultWhenEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.viper.App.Language = ""

	// Act & Assert
	assert.Equal(t, "en", *cfg.GetAppLanguage(), 0.001)
}

func testAppLogLevelGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default value
	assert.Equal(t, "info", cfg.GetAppLogLevel(), 0.001)

	// Act - set new value
	cfg.SetAppLogLevel("debug")

	// Assert
	assert.Equal(t, "debug", cfg.GetAppLogLevel(), 0.001)
}

func testAppLogLevelDefaultWhenEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.viper.App.LogLevel = ""

	// Act & Assert
	assert.Equal(t, "info", cfg.GetAppLogLevel(), 0.001)
}

func testAppWebUIEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default value from defaultConfig is true
	assert.True(t, cfg.GetAppWebUIEnabled(), 0.001)
}

func testAppWebUIPortGetDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default from defaultConfig is 80
	assert.InDelta(t, 80, cfg.GetAppWebUIPort(), 0.001)
}

func testAppWebUIPortDefaultWhenZero(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.viper.App.WebUIPort = 0

	// Act & Assert
	assert.InDelta(t, 8080, cfg.GetAppWebUIPort(), 0.001)
}

func testAppUpdateAutoCheckGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.GetAppUpdateAutoCheck(), 0.001)

	// Act - set true
	cfg.SetAppUpdateAutoCheck(true)

	// Assert
	assert.True(t, cfg.GetAppUpdateAutoCheck(), 0.001)
}

func testAppUpdateAutoInstallGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.GetAppUpdateAutoInstall(), 0.001)

	// Act - set true
	cfg.SetAppUpdateAutoInstall(true)

	// Assert
	assert.True(t, cfg.GetAppUpdateAutoInstall(), 0.001)
}

func testAppUpdateCheckIntervalMinutesGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 60
	assert.InDelta(t, 60, cfg.GetAppUpdateCheckIntervalMinutes(), 0.001)

	// Act - set new value
	cfg.SetAppUpdateCheckIntervalMinutes(120)

	// Assert
	assert.InDelta(t, 120, cfg.GetAppUpdateCheckIntervalMinutes(), 0.001)
}

func testAppUpdateBaseURL(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert
	assert.Equal(t, "https://simtezilo.com/releases", cfg.GetAppUpdateBaseURL(), 0.001)
}

func testAppUpdateChannelGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is "stable"
	assert.Equal(t, "stable", cfg.GetAppUpdateChannel(), 0.001)

	// Act - set new value
	cfg.SetAppUpdateChannel("beta")

	// Assert
	assert.Equal(t, "beta", cfg.GetAppUpdateChannel(), 0.001)
}

func testDevToolsEnabledGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.GetDevToolsEnabled(), 0.001)

	// Act - set true
	cfg.SetDevToolsEnabled(true)

	// Assert
	assert.True(t, cfg.GetDevToolsEnabled(), 0.001)
}

func testStatusAndRestartRequired(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - initially no restart required
	assert.False(t, cfg.RestartRequired(), 0.001)

	// Act - mark restart required
	cfg.MarkRestartRequired()

	// Assert
	assert.True(t, cfg.RestartRequired(), 0.001)
	assert.True(t, cfg.Status().RestartRequired)
	assert.Positive(t, cfg.Status().LastUpdate)
}

func testIsUpToDate(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	// Use a timestamp from the past to ensure the comparison works
	oldTimestamp := cfg.Status().LastUpdate - 1

	// Act - make a change
	cfg.SetAppAccent("uk")

	// Assert - should have newer timestamp (oldTimestamp < newTimestamp)
	assert.True(t, cfg.IsUpToDate(oldTimestamp))
}

func TestAppSection(t *testing.T) { //nolint:dupl // Test runners have similar structure by design
	t.Parallel()

	t.Run("testAppAccentGetSet", testAppAccentGetSet)
	t.Run("testAppAccentDefaultWhenEmpty", testAppAccentDefaultWhenEmpty)
	t.Run("testAppBaseDirGetSet", testAppBaseDirGetSet)
	t.Run("testAppBaseDirDefaultWhenEmpty", testAppBaseDirDefaultWhenEmpty)
	t.Run("testAppVehicleDBFileGetSet", testAppVehicleDBFileGetSet)
	t.Run("testAppLanguageGetSet", testAppLanguageGetSet)
	t.Run("testAppLanguageDefaultWhenEmpty", testAppLanguageDefaultWhenEmpty)
	t.Run("testAppLogLevelGetSet", testAppLogLevelGetSet)
	t.Run("testAppLogLevelDefaultWhenEmpty", testAppLogLevelDefaultWhenEmpty)
	t.Run("testAppWebUIEnabled", testAppWebUIEnabled)
	t.Run("testAppWebUIPortGetDefault", testAppWebUIPortGetDefault)
	t.Run("testAppWebUIPortDefaultWhenZero", testAppWebUIPortDefaultWhenZero)
	t.Run("testAppUpdateAutoCheckGetSet", testAppUpdateAutoCheckGetSet)
	t.Run("testAppUpdateAutoInstallGetSet", testAppUpdateAutoInstallGetSet)
	t.Run("testAppUpdateCheckIntervalMinutesGetSet", testAppUpdateCheckIntervalMinutesGetSet)
	t.Run("testAppUpdateBaseURL", testAppUpdateBaseURL)
	t.Run("testAppUpdateChannelGetSet", testAppUpdateChannelGetSet)
	t.Run("testDevToolsEnabledGetSet", testDevToolsEnabledGetSet)
	t.Run("testStatusAndRestartRequired", testStatusAndRestartRequired)
	t.Run("testIsUpToDate", testIsUpToDate)
}

// ****************************************************************************
// Hardware section tests
// ****************************************************************************

func testHardwareModelGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is "none"
	assert.Equal(t, "none", cfg.GetHardwareModel(), 0.001)

	// Act - set new value
	cfg.SetHardwareModel("console")

	// Assert
	assert.Equal(t, "console", cfg.GetHardwareModel(), 0.001)
}

func testDisplayOrientationGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 0
	assert.InDelta(t, 0, cfg.GetDisplayOrientation(), 0.001)

	// Act - set new value
	cfg.SetDisplayOrientation(90)

	// Assert
	assert.InDelta(t, 90, cfg.GetDisplayOrientation(), 0.001)
}

func testDisplayOrientationNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"0 degrees", 0, 0},
		{"90 degrees", 90, 90},
		{"180 degrees", 180, 180},
		{"270 degrees", 270, 270},
		{"360 becomes 0", 360, 0},
		{"450 becomes 90", 450, 90},
		{"-90 becomes 270", -90, 270},
		{"45 rounds to 90", 45, 90},
		{"44 rounds to 0", 44, 0},
		{"135 rounds to 180", 135, 180},
		{"225 rounds to 270", 225, 270},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := newTestConfig()

			// Act
			cfg.SetDisplayOrientation(testCase.input)

			// Assert
			assert.Equal(t, testCase.expected, cfg.GetDisplayOrientation(), 0.001)
		})
	}
}

func TestHardwareSection(t *testing.T) {
	t.Parallel()

	t.Run("testHardwareModelGetSet", testHardwareModelGetSet)
	t.Run("testDisplayOrientationGetSet", testDisplayOrientationGetSet)
	t.Run("testDisplayOrientationNormalization", testDisplayOrientationNormalization)
}

// ****************************************************************************
// Config constructor tests
// ****************************************************************************

func testNewFromJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	json := `{
		"schemaVersion": "1.0.0",
		"app": {
			"language": "de",
			"accent": "uk",
			"logLevel": "debug"
		}
	}`

	// Act
	cfg := newTestConfigFromJSON(json)

	// Assert
	require.NotNil(t, cfg)
	assert.Equal(t, "de", *cfg.GetAppLanguage(), 0.001)
	assert.Equal(t, "uk", cfg.GetAppAccent(), 0.001)
	assert.Equal(t, "debug", cfg.GetAppLogLevel(), 0.001)
}

func testGetConfigFilePath(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.configFile = "/test/config.json"

	// Act & Assert
	assert.Equal(t, "/test/config.json", cfg.GetConfigFilePath(), 0.001)
}

func TestConfigConstructors(t *testing.T) {
	t.Parallel()

	t.Run("testNewFromJSON", testNewFromJSON)
	t.Run("testGetConfigFilePath", testGetConfigFilePath)
}

// ****************************************************************************
// Haptics core section tests (Dynamic Transmission, Jerk, Snap)
// ****************************************************************************

func testHapticsDynamicTransFeedbackEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is true
	assert.True(t, cfg.GethapticsDynamicTransFeedbackEnabled(), 0.001)

	// Act - set false
	cfg.SetHapticsDynamicTransFeedbackEnabled(false)

	// Assert
	assert.False(t, cfg.GethapticsDynamicTransFeedbackEnabled(), 0.001)
}

func testHapticsJerkCurveGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 190
	assert.InDelta(t, 190, cfg.GethapticsJerkCurve(), 0.001)

	// Act - set new value
	cfg.SetHapticsJerkCurve(300)

	// Assert
	assert.InDelta(t, 300, cfg.GethapticsJerkCurve(), 0.001)
}

func testHapticsJerkCurveClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set value below minimum (5)
	cfg.SetHapticsJerkCurve(1)

	// Assert - should be clamped to 5
	assert.InDelta(t, 5, cfg.GethapticsJerkCurve(), 0.001)

	// Act - set value above maximum (955)
	cfg.SetHapticsJerkCurve(1000)

	// Assert - should be clamped to 955
	assert.InDelta(t, 955, cfg.GethapticsJerkCurve(), 0.001)
}

func testHapticsJerkCurveIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetHapticsJerkCurve(100)

	// Act - increase
	result := cfg.IncreaseHapticsJerkCurve()

	// Assert
	assert.InDelta(t, 105, result, 0.001)
	assert.InDelta(t, 105, cfg.GethapticsJerkCurve(), 0.001)

	// Act - decrease
	result = cfg.DecreaseHapticsJerkCurve()

	// Assert
	assert.InDelta(t, 100, result, 0.001)
	assert.InDelta(t, 100, cfg.GethapticsJerkCurve(), 0.001)
}

func testHapticsJerkMaxGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 37
	assert.InDelta(t, 37, cfg.GetHapticsJerkMax(), 0.001)

	// Act - set new value
	cfg.SetHapticsJerkMax(50)

	// Assert
	assert.InDelta(t, 50, cfg.GetHapticsJerkMax(), 0.001)
}

func testHapticsJerkMaxClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set value below minimum (1)
	cfg.SetHapticsJerkMax(0)

	// Assert - should be clamped to 1
	assert.InDelta(t, 1, cfg.GetHapticsJerkMax(), 0.001)

	// Act - set value above maximum (200)
	cfg.SetHapticsJerkMax(300)

	// Assert - should be clamped to 200
	assert.InDelta(t, 200, cfg.GetHapticsJerkMax(), 0.001)
}

func testHapticsJerkMaxIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetHapticsJerkMax(50)

	// Act - increase
	result := cfg.IncreaseHapticsJerkMax()

	// Assert
	assert.InDelta(t, 51, result, 0.001)
	assert.InDelta(t, 51, cfg.GetHapticsJerkMax(), 0.001)

	// Act - decrease
	result = cfg.DecreaseHapticsJerkMax()

	// Assert
	assert.InDelta(t, 50, result, 0.001)
	assert.InDelta(t, 50, cfg.GetHapticsJerkMax(), 0.001)
}

func testHapticsJerkScale(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - should have a scale value
	scale := cfg.GetHapticsJerkScale()
	assert.Greater(t, scale, 0.0)
}

func testHapticsReplayEnabledGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.GetHapticsReplayEnabled(), 0.001)

	// Act - set true
	cfg.SetHapticsEnableReplay(true)

	// Assert
	assert.True(t, cfg.GetHapticsReplayEnabled(), 0.001)
}

func testHapticsSnapCurveGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 310
	assert.InDelta(t, 310, cfg.GetHapticsSnapCurve(), 0.001)

	// Act - set new value
	cfg.SetHapticsSnapCurve(400)
	cfg.rebuildSnapshot() // Required as SetHapticsSnapCurve doesn't rebuild snapshot

	// Assert
	assert.InDelta(t, 400, cfg.GetHapticsSnapCurve(), 0.001)
}

func testHapticsSnapCurveClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set value below minimum (5)
	cfg.SetHapticsSnapCurve(1)
	cfg.rebuildSnapshot() // Required as SetHapticsSnapCurve doesn't rebuild snapshot

	// Assert - should be clamped to 5
	assert.InDelta(t, 5, cfg.GetHapticsSnapCurve(), 0.001)

	// Act - set value above maximum (955)
	cfg.SetHapticsSnapCurve(1000)
	cfg.rebuildSnapshot()

	// Assert - should be clamped to 955
	assert.InDelta(t, 955, cfg.GetHapticsSnapCurve(), 0.001)
}

func testHapticsSnapCurveIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetHapticsSnapCurve(200)

	// Act - increase
	result := cfg.IncreaseHapticsSnapCurve()

	// Assert
	assert.InDelta(t, 205, result, 0.001)
	assert.InDelta(t, 205, cfg.GetHapticsSnapCurve(), 0.001)

	// Act - decrease
	result = cfg.DecreaseHapticsSnapCurve()

	// Assert
	assert.InDelta(t, 200, result, 0.001)
	assert.InDelta(t, 200, cfg.GetHapticsSnapCurve(), 0.001)
}

func testHapticsSnapMaxGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 90
	assert.InDelta(t, 90, cfg.GetHapticsSnapMax(), 0.001)

	// Act - set new value
	cfg.SetHapticsSnapMax(100)
	cfg.rebuildSnapshot() // Required as SetHapticsSnapMax doesn't rebuild snapshot

	// Assert
	assert.InDelta(t, 100, cfg.GetHapticsSnapMax(), 0.001)
}

func testHapticsSnapMaxClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set value below minimum (1)
	cfg.SetHapticsSnapMax(0)
	cfg.rebuildSnapshot() // Required as SetHapticsSnapMax doesn't rebuild snapshot

	// Assert - should be clamped to 1
	assert.InDelta(t, 1, cfg.GetHapticsSnapMax(), 0.001)

	// Act - set value above maximum (200)
	cfg.SetHapticsSnapMax(300)
	cfg.rebuildSnapshot()

	// Assert - should be clamped to 200
	assert.InDelta(t, 200, cfg.GetHapticsSnapMax(), 0.001)
}

func testHapticsSnapMaxIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetHapticsSnapMax(50)

	// Act - increase
	result := cfg.IncreaseHapticsSnapMax()

	// Assert
	assert.InDelta(t, 51, result, 0.001)
	assert.InDelta(t, 51, cfg.GetHapticsSnapMax(), 0.001)

	// Act - decrease
	result = cfg.DecreaseHapticsSnapMax()

	// Assert
	assert.InDelta(t, 50, result, 0.001)
	assert.InDelta(t, 50, cfg.GetHapticsSnapMax(), 0.001)
}

func testHapticsSnapScale(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - should have a scale value
	scale := cfg.GetHapticsSnapScale()
	assert.Greater(t, scale, 0.0)
}

func testHapticsTransmissionCurveGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 150
	assert.InDelta(t, 150, cfg.GetHapticsTransmissionCurve(), 0.001)

	// Act - set new value
	cfg.SetHapticsTransmissionCurve(200)

	// Assert
	assert.InDelta(t, 200, cfg.GetHapticsTransmissionCurve(), 0.001)
}

func testHapticsTransmissionCurveClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set value below minimum (5)
	cfg.SetHapticsTransmissionCurve(1)

	// Assert - should be clamped to 5
	assert.InDelta(t, 5, cfg.GetHapticsTransmissionCurve(), 0.001)

	// Act - set value above maximum (955)
	cfg.SetHapticsTransmissionCurve(1000)

	// Assert - should be clamped to 955
	assert.InDelta(t, 955, cfg.GetHapticsTransmissionCurve(), 0.001)
}

func testHapticsTransmissionCurveIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetHapticsTransmissionCurve(200)

	// Act - increase
	result := cfg.IncreaseHapticsTransmissionCurve()

	// Assert
	assert.InDelta(t, 205, result, 0.001)
	assert.InDelta(t, 205, cfg.GetHapticsTransmissionCurve(), 0.001)

	// Act - decrease
	result = cfg.DecreaseHapticsTransmissionCurve()

	// Assert
	assert.InDelta(t, 200, result, 0.001)
	assert.InDelta(t, 200, cfg.GetHapticsTransmissionCurve(), 0.001)
}

func testHapticsTransmissionGforceMaxGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 2.0
	assert.InDelta(t, 2.0, cfg.GetHapticsTransmissionGforceMax(), 0.001)

	// Act - set new value
	cfg.SetHapticsTransmissionGforceMax(3.5)
	cfg.rebuildSnapshot() // Required as SetHapticsTransmissionGforceMax doesn't rebuild snapshot

	// Assert
	assert.InDelta(t, 3.5, cfg.GetHapticsTransmissionGforceMax(), 0.001)
}

func testHapticsTransmissionGforceMaxClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set value below minimum (0)
	cfg.SetHapticsTransmissionGforceMax(-1.0)
	cfg.rebuildSnapshot() // Required as SetHapticsTransmissionGforceMax doesn't rebuild snapshot

	// Assert - should be clamped to 0
	assert.InDelta(t, 0.0, cfg.GetHapticsTransmissionGforceMax(), 0.001)

	// Act - set value above maximum (10)
	cfg.SetHapticsTransmissionGforceMax(15.0)
	cfg.rebuildSnapshot()

	// Assert - should be clamped to 10
	assert.InDelta(t, 10.0, cfg.GetHapticsTransmissionGforceMax(), 0.001)
}

func testHapticsTransmissionGforceMaxIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetHapticsTransmissionGforceMax(2.0)
	cfg.rebuildSnapshot()

	// Act - increase
	result := cfg.IncreaseHapticsTransmissionGforceMax()

	// Assert
	assert.InDelta(t, 2.1, result, 0.001)

	// Act - decrease
	result = cfg.DecreasehapticsTransmissionGforceMax()

	// Assert
	assert.InDelta(t, 2.0, result, 0.001)
}

func TestHapticsCoreSection(t *testing.T) {
	t.Parallel()

	t.Run("testHapticsDynamicTransFeedbackEnabled", testHapticsDynamicTransFeedbackEnabled)
	t.Run("testHapticsJerkCurveGetSet", testHapticsJerkCurveGetSet)
	t.Run("testHapticsJerkCurveClamping", testHapticsJerkCurveClamping)
	t.Run("testHapticsJerkCurveIncreaseDecrease", testHapticsJerkCurveIncreaseDecrease)
	t.Run("testHapticsJerkMaxGetSet", testHapticsJerkMaxGetSet)
	t.Run("testHapticsJerkMaxClamping", testHapticsJerkMaxClamping)
	t.Run("testHapticsJerkMaxIncreaseDecrease", testHapticsJerkMaxIncreaseDecrease)
	t.Run("testHapticsJerkScale", testHapticsJerkScale)
	t.Run("testHapticsReplayEnabledGetSet", testHapticsReplayEnabledGetSet)
	t.Run("testHapticsSnapCurveGetSet", testHapticsSnapCurveGetSet)
	t.Run("testHapticsSnapCurveClamping", testHapticsSnapCurveClamping)
	t.Run("testHapticsSnapCurveIncreaseDecrease", testHapticsSnapCurveIncreaseDecrease)
	t.Run("testHapticsSnapMaxGetSet", testHapticsSnapMaxGetSet)
	t.Run("testHapticsSnapMaxClamping", testHapticsSnapMaxClamping)
	t.Run("testHapticsSnapMaxIncreaseDecrease", testHapticsSnapMaxIncreaseDecrease)
	t.Run("testHapticsSnapScale", testHapticsSnapScale)
	t.Run("testHapticsTransmissionCurveGetSet", testHapticsTransmissionCurveGetSet)
	t.Run("testHapticsTransmissionCurveClamping", testHapticsTransmissionCurveClamping)
	t.Run("testHapticsTransmissionCurveIncreaseDecrease", testHapticsTransmissionCurveIncreaseDecrease)
	t.Run("testHapticsTransmissionGforceMaxGetSet", testHapticsTransmissionGforceMaxGetSet)
	t.Run("testHapticsTransmissionGforceMaxClamping", testHapticsTransmissionGforceMaxClamping)
	t.Run("testHapticsTransmissionGforceMaxIncreaseDecrease", testHapticsTransmissionGforceMaxIncreaseDecrease)
}

// ****************************************************************************
// Haptics pulse and engine profile tests
// ****************************************************************************

func testHapticsPulseMinHz(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 16
	assert.InDelta(t, 16, cfg.GetHapticsPulseMinHz(), 0.001)
}

func testHapticsPulseMinHzIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetHapticsPulseMinFrequencyHz(15)

	// Act - increase
	result := cfg.IncreaseHapticsPulseMinHz()

	// Assert
	assert.InDelta(t, 16, result, 0.001)

	// Act - decrease
	result = cfg.DecreaseHapticsPulseMinHz()

	// Assert
	assert.InDelta(t, 15, result, 0.001)
}

func testHapticsPulseMinHzClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - decrease below minimum (5)
	cfg.SetHapticsPulseMinFrequencyHz(5)
	result := cfg.DecreaseHapticsPulseMinHz()

	// Assert - should stay at 5
	assert.InDelta(t, 5, result, 0.001)

	// Act - increase above maximum (25)
	cfg.SetHapticsPulseMinFrequencyHz(25)
	result = cfg.IncreaseHapticsPulseMinHz()

	// Assert - should stay at 25
	assert.InDelta(t, 25, result, 0.001)
}

func testHapticsPulseMaxHz(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 60
	assert.InDelta(t, 60, cfg.GetHapticsPulseMaxHz(), 0.001)
}

func testHapticsPulseMaxHzIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetHapticsPulseMaxFrequencyHz(50)

	// Act - increase
	result := cfg.IncreaseHapticsPulseMaxHz()

	// Assert
	assert.InDelta(t, 51, result, 0.001)

	// Act - decrease
	result = cfg.DecreaseHapticsPulseMaxHz()

	// Assert
	assert.InDelta(t, 50, result, 0.001)
}

func testHapticsPulseMaxHzClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - decrease below minimum (26)
	cfg.SetHapticsPulseMaxFrequencyHz(26)
	result := cfg.DecreaseHapticsPulseMaxHz()

	// Assert - should stay at 26
	assert.InDelta(t, 26, result, 0.001)

	// Act - increase above maximum (100)
	cfg.SetHapticsPulseMaxFrequencyHz(100)
	result = cfg.IncreaseHapticsPulseMaxHz()

	// Assert - should stay at 100
	assert.InDelta(t, 100, result, 0.001)
}

func testHapticsPulseFrequencyRange(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetHapticsPulseMinFrequencyHz(20)
	cfg.SetHapticsPulseMaxFrequencyHz(80)

	// Act & Assert
	assert.InDelta(t, 60, cfg.GetHapticePulseFrequencyHzRange(), 0.001)
}

func testHapticsPulseMaxAmplitudeGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 1
	assert.InDelta(t, 1.0, cfg.GetHapticsPulseMaxAmplitude(), 0.001)

	// Act - set new value
	cfg.SetHapticsPulseMaxAmplitude(0.8)

	// Assert
	assert.InDelta(t, 0.8, cfg.GetHapticsPulseMaxAmplitude(), 0.001)
}

func testHapticsPulseMaxAmplitudeIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetHapticsPulseMaxAmplitude(0.5)

	// Act - increase
	result := cfg.IncreaseHapticsPulseMaxAmplitude()

	// Assert
	assert.InDelta(t, 0.51, result, 0.001)

	// Act - decrease
	result = cfg.DecreaseHapticsPulseMaxAmplitude()

	// Assert
	assert.InDelta(t, 0.5, result, 0.001)
}

func testHapticsPulseMaxAmplitudeClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - decrease below minimum (0)
	cfg.SetHapticsPulseMaxAmplitude(0.0)
	result := cfg.DecreaseHapticsPulseMaxAmplitude()

	// Assert - should stay at 0
	assert.InDelta(t, 0.0, result, 0.001)

	// Act - increase above maximum (1.0)
	cfg.SetHapticsPulseMaxAmplitude(1.0)
	result = cfg.IncreaseHapticsPulseMaxAmplitude()

	// Assert - should stay at 1.0
	assert.InDelta(t, 1.0, result, 0.001)
}

func testHapticsPulseWidthMinMax(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - should have calculated pulse widths
	assert.Greater(t, cfg.GetHapticsPulseWidthMin(), 0.0)
	assert.Greater(t, cfg.GetHapticsPulseWidthMax(), cfg.GetHapticsPulseWidthMin(), 0.001)
}

func testHapticsEngineProfile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - get existing profile
	profile := cfg.GetHapticsEngineProfile("i4")

	// Assert
	require.NotNil(t, profile)
	assert.InDelta(t, 0.76, profile.PrimaryBalance, 0.001)
	assert.InDelta(t, 0.80, profile.SecondaryBalance, 0.001)
}

func testHapticsEngineProfileNotFound(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - get non-existent profile
	profile := cfg.GetHapticsEngineProfile("nonexistent")

	// Assert
	assert.Nil(t, profile)
}

func testHapticsEnginePrimaryBalanceNoProfile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - no profile selected, should return 1.0
	assert.InDelta(t, 1.0, cfg.GetHapticesEnginePrimaryBalance(), 0.001)
}

func testHapticsEnginePrimaryBalanceWithProfile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.GetHapticsEngineProfile("i4") // Select profile

	// Act & Assert
	assert.InDelta(t, 0.76, cfg.GetHapticesEnginePrimaryBalance(), 0.001)
}

func testHapticsEnginePrimaryBalanceIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.GetHapticsEngineProfile("i4") // Select profile with balance 0.76

	// Act - increase
	result := cfg.IncreaseHapticsEnginePrimaryBalance()

	// Assert
	assert.InDelta(t, 0.77, result, 0.001)

	// Act - decrease
	result = cfg.DecreaseHapticsEnginePrimaryBalance()

	// Assert
	assert.InDelta(t, 0.76, result, 0.001)
}

func testHapticsEngineSecondaryBalanceNoProfile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - no profile selected, should return 1.0
	assert.InDelta(t, 1.0, cfg.GetHapticsEngineSecondaryBalance(), 0.001)
}

func testHapticsEngineSecondaryBalanceWithProfile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.GetHapticsEngineProfile("i4") // Select profile

	// Act & Assert
	assert.InDelta(t, 0.80, cfg.GetHapticsEngineSecondaryBalance(), 0.001)
}

func testHapticsEngineSecondaryBalanceIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.GetHapticsEngineProfile("i4") // Select profile with secondary balance 0.80

	// Act - increase
	result := cfg.IncreaseHapticsEngineSecondaryBalance()

	// Assert
	assert.InDelta(t, 0.81, result, 0.001)

	// Act - decrease
	result = cfg.DecreaseHapticsEngineSecondaryBalance()

	// Assert
	assert.InDelta(t, 0.80, result, 0.001)
}

func testHapticsEnginePulseGainNoProfile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - no profile selected, should return MinimumGain
	assert.InDelta(t, MinimumGain, cfg.GetHapticsEnginePulseGain(), 0.001)
}

func testHapticsEnginePulseGainWithProfile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.GetHapticsEngineProfile("i4") // Select profile with gain -4.25

	// Act & Assert
	assert.InDelta(t, -4.25, cfg.GetHapticsEnginePulseGain(), 0.001)
}

func testHapticsEnginePulseScaleNoProfile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - no profile selected, should return 1.0
	assert.InDelta(t, 1.0, cfg.GetHapticsEnginePulseScale(), 0.001)
}

func testHapticsEnginePulseScaleWithProfile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.GetHapticsEngineProfile("i4") // Select profile with pulse scale 0.75

	// Act & Assert
	assert.InDelta(t, 0.75, cfg.GetHapticsEnginePulseScale(), 0.001)
}

func testHapticsEnginePulseScaleIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.GetHapticsEngineProfile("i4") // Select profile with pulse scale 0.75

	// Act - increase
	result := cfg.IncreaseHapticsEnginePulseScale()

	// Assert
	assert.InDelta(t, 0.76, result, 0.001)

	// Act - decrease
	result = cfg.DecreasehapticsEnginePulseScale()

	// Assert
	assert.InDelta(t, 0.75, result, 0.001)
}

func TestHapticsPulseAndEngineProfile(t *testing.T) { //nolint:dupl // Test runners have similar structure by design
	t.Parallel()

	t.Run("testHapticsPulseMinHz", testHapticsPulseMinHz)
	t.Run("testHapticsPulseMinHzIncreaseDecrease", testHapticsPulseMinHzIncreaseDecrease)
	t.Run("testHapticsPulseMinHzClamping", testHapticsPulseMinHzClamping)
	t.Run("testHapticsPulseMaxHz", testHapticsPulseMaxHz)
	t.Run("testHapticsPulseMaxHzIncreaseDecrease", testHapticsPulseMaxHzIncreaseDecrease)
	t.Run("testHapticsPulseMaxHzClamping", testHapticsPulseMaxHzClamping)
	t.Run("testHapticsPulseFrequencyRange", testHapticsPulseFrequencyRange)
	t.Run("testHapticsPulseMaxAmplitudeGetSet", testHapticsPulseMaxAmplitudeGetSet)
	t.Run("testHapticsPulseMaxAmplitudeIncreaseDecrease", testHapticsPulseMaxAmplitudeIncreaseDecrease)
	t.Run("testHapticsPulseMaxAmplitudeClamping", testHapticsPulseMaxAmplitudeClamping)
	t.Run("testHapticsPulseWidthMinMax", testHapticsPulseWidthMinMax)
	t.Run("testHapticsEngineProfile", testHapticsEngineProfile)
	t.Run("testHapticsEngineProfileNotFound", testHapticsEngineProfileNotFound)
	t.Run("testHapticsEnginePrimaryBalanceNoProfile", testHapticsEnginePrimaryBalanceNoProfile)
	t.Run("testHapticsEnginePrimaryBalanceWithProfile", testHapticsEnginePrimaryBalanceWithProfile)
	t.Run("testHapticsEnginePrimaryBalanceIncreaseDecrease", testHapticsEnginePrimaryBalanceIncreaseDecrease)
	t.Run("testHapticsEngineSecondaryBalanceNoProfile", testHapticsEngineSecondaryBalanceNoProfile)
	t.Run("testHapticsEngineSecondaryBalanceWithProfile", testHapticsEngineSecondaryBalanceWithProfile)
	t.Run("testHapticsEngineSecondaryBalanceIncreaseDecrease", testHapticsEngineSecondaryBalanceIncreaseDecrease)
	t.Run("testHapticsEnginePulseGainNoProfile", testHapticsEnginePulseGainNoProfile)
	t.Run("testHapticsEnginePulseGainWithProfile", testHapticsEnginePulseGainWithProfile)
	t.Run("testHapticsEnginePulseScaleNoProfile", testHapticsEnginePulseScaleNoProfile)
	t.Run("testHapticsEnginePulseScaleWithProfile", testHapticsEnginePulseScaleWithProfile)
	t.Run("testHapticsEnginePulseScaleIncreaseDecrease", testHapticsEnginePulseScaleIncreaseDecrease)
}

// ****************************************************************************
// Pit Radio notification tests
// ****************************************************************************

func testPitRadioEnabledGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.PitRadioEnabled(), 0.001)

	// Act - set true
	cfg.SetPitRadioEnabled(true)

	// Assert
	assert.True(t, cfg.PitRadioEnabled(), 0.001)
}

func testPitRadioOutputGetSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is "log"
	assert.Equal(t, "log", cfg.GetPitRadioOutput(), 0.001)

	// Act - set to discord
	cfg.SetPitRadioOutput("discord")

	// Assert
	assert.Equal(t, "discord", cfg.GetPitRadioOutput(), 0.001)

	// Act - set invalid value, should default to "log"
	cfg.SetPitRadioOutput("invalid")

	// Assert
	assert.Equal(t, "log", cfg.GetPitRadioOutput(), 0.001)
}

func testPitRadioMessageSendIntervalMs(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 2000
	assert.InDelta(t, 2000, cfg.GetPitRadioMessageSendIntervalMs(), 0.001)

	// Act - set new value
	cfg.SetPitRadioMessageSendIntervalMs(3000)

	// Assert
	assert.InDelta(t, 3000, cfg.GetPitRadioMessageSendIntervalMs(), 0.001)
}

func testPitRadioNotifyRaceProgressEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is true
	assert.True(t, cfg.GetPitRadioNotifyRaceProgressEnabled(), 0.001)

	// Act - set false
	cfg.SetPitRadioNotifyRaceProgressEnabled(false)

	// Assert
	assert.False(t, cfg.GetPitRadioNotifyRaceProgressEnabled(), 0.001)
}

func testPitRadioNotifyRaceProgressMinLaps(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 10
	assert.InDelta(t, 10, cfg.GetPitRadioNotifyRaceProgressMinLaps(), 0.001)

	// Act - set new value
	cfg.SetPitRadioNotifyRaceProgressMinLaps(15)

	// Assert
	assert.InDelta(t, 15, cfg.GetPitRadioNotifyRaceProgressMinLaps(), 0.001)
}

func testPitRadioNotifyRaceProgressMinLapsIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioNotifyRaceProgressMinLaps(10)

	// Act - increase
	result := cfg.IncreasePitRadioNotifyRaceProgressMinLaps()

	// Assert
	assert.InDelta(t, 11, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioNotifyRaceProgressMinLaps()

	// Assert
	assert.InDelta(t, 10, result, 0.001)
}

func testPitRadioNotifyRaceProgressMinLapsClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - decrease below minimum (1)
	cfg.SetPitRadioNotifyRaceProgressMinLaps(1)
	result := cfg.DecreasePitRadioNotifyRaceProgressMinLaps()

	// Assert - should stay at 1
	assert.InDelta(t, 1, result, 0.001)

	// Act - increase above maximum (50)
	cfg.SetPitRadioNotifyRaceProgressMinLaps(50)
	result = cfg.IncreasePitRadioNotifyRaceProgressMinLaps()

	// Assert - should stay at 50
	assert.InDelta(t, 50, result, 0.001)
}

func testPitRadioNotifyRaceProgressIntervalPc(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 25
	assert.InDelta(t, 25, cfg.GetPitRadioNotifyRaceProgressIntervalPc(), 0.001)

	// Act - set new value
	cfg.SetPitRadioNotifyRaceProgressIntervalPc(30)

	// Assert
	assert.InDelta(t, 30, cfg.GetPitRadioNotifyRaceProgressIntervalPc(), 0.001)
}

func testPitRadioNotifyRaceProgressIntervalPcClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum (5)
	cfg.SetPitRadioNotifyRaceProgressIntervalPc(1)

	// Assert - should be clamped to 5
	assert.InDelta(t, 5, cfg.GetPitRadioNotifyRaceProgressIntervalPc(), 0.001)

	// Act - set above maximum (50)
	cfg.SetPitRadioNotifyRaceProgressIntervalPc(60)

	// Assert - should be clamped to 50
	assert.InDelta(t, 50, cfg.GetPitRadioNotifyRaceProgressIntervalPc(), 0.001)
}

func testPitRadioNotifyRaceProgressIntervalPcIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioNotifyRaceProgressIntervalPc(25)

	// Act - increase
	result := cfg.IncreasePitRadioNotifyRaceProgressIntervalPc()

	// Assert
	assert.InDelta(t, 30, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioNotifyRaceProgressIntervalPc()

	// Assert
	assert.InDelta(t, 25, result, 0.001)
}

func testPitRadioNotifyRaceLapsEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is true
	assert.True(t, cfg.GetPitRadioNotifyRaceLapsEnabled(), 0.001)

	// Act - set false
	cfg.SetPitRadioNotifyRaceLapsEnabled(false)

	// Assert
	assert.False(t, cfg.GetPitRadioNotifyRaceLapsEnabled(), 0.001)
}

func testPitRadioNotifyRaceLapsIntervalLaps(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 1
	assert.InDelta(t, 1, cfg.GetPitRadioNotifyRaceLapsIntervalLaps(), 0.001)

	// Act - set new value
	cfg.SetPitRadioNotifyRaceLapsIntervalLaps(5)

	// Assert
	assert.InDelta(t, 5, cfg.GetPitRadioNotifyRaceLapsIntervalLaps(), 0.001)
}

func testPitRadioNotifyRaceLapsIntervalLapsIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioNotifyRaceLapsIntervalLaps(5)

	// Act - increase
	result := cfg.IncreasePitRadioNotifyRaceLapsIntervalLaps()

	// Assert
	assert.InDelta(t, 6, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioNotifyRaceLapsIntervalLaps()

	// Assert
	assert.InDelta(t, 5, result, 0.001)
}

func testPitRadioNotifyRaceLapsCountdownLaps(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 3
	assert.InDelta(t, 3, cfg.GetPitRadioNotifyRaceLapsCountdownLaps(), 0.001)

	// Act - set new value
	cfg.SetPitRadioNotifyRaceLapsCountdownLaps(5)

	// Assert
	assert.InDelta(t, 5, cfg.GetPitRadioNotifyRaceLapsCountdownLaps(), 0.001)
}

func testPitRadioNotifyRaceLapsCountdownLapsIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioNotifyRaceLapsCountdownLaps(5)

	// Act - increase
	result := cfg.IncreasePitRadioNotifyRaceLapsCountdownLaps()

	// Assert
	assert.InDelta(t, 6, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioNotifyRaceLapsCountdownLaps()

	// Assert
	assert.InDelta(t, 5, result, 0.001)
}

func testPitRadioNotifyLapTimesEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is true
	assert.True(t, cfg.GetPitRadioNotifyLapTimesEnabled(), 0.001)

	// Act - set false
	cfg.SetPitRadioNotifyLapTimesEnabled(false)

	// Assert
	assert.False(t, cfg.GetPitRadioNotifyLapTimesEnabled(), 0.001)
}

func testPitRadioNotifyLapTimesMaxDeltaSeconds(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 2.0
	assert.InDelta(t, 2.0, cfg.GetPitRadioNotifyLapTimesMaxDeltaSeconds(), 0.001)

	// Act - set new value
	cfg.SetPitRadioNotifyLapTimesMaxDeltaSeconds(5.0)

	// Assert
	assert.InDelta(t, 5.0, cfg.GetPitRadioNotifyLapTimesMaxDeltaSeconds(), 0.001)
}

func testPitRadioNotifyLapTimesMaxDeltaSecondsClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum (0.1)
	cfg.SetPitRadioNotifyLapTimesMaxDeltaSeconds(0.05)

	// Assert - should be clamped to 0.1
	assert.InDelta(t, 0.1, cfg.GetPitRadioNotifyLapTimesMaxDeltaSeconds(), 0.001)

	// Act - set above maximum (30.0)
	cfg.SetPitRadioNotifyLapTimesMaxDeltaSeconds(35.0)

	// Assert - should be clamped to 30.0
	assert.InDelta(t, 30.0, cfg.GetPitRadioNotifyLapTimesMaxDeltaSeconds(), 0.001)
}

func testPitRadioNotifyLapTimesMaxDeltaSecondsIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioNotifyLapTimesMaxDeltaSeconds(5.0)

	// Act - increase
	result := cfg.IncreasePitRadioNotifyLapTimesMaxDeltaSeconds()

	// Assert
	assert.InDelta(t, 5.1, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioNotifyLapTimesMaxDeltaSeconds()

	// Assert
	assert.InDelta(t, 5.0, result, 0.001)
}

func testPitRadioNotifyCircuitMatchingEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.GetPitRadioNotifyCircuitMatchingEnabled(), 0.001)

	// Act - set true
	cfg.SetPitRadioNotifyCircuitMatchingEnabled(true)

	// Assert
	assert.True(t, cfg.GetPitRadioNotifyCircuitMatchingEnabled(), 0.001)
}

func TestPitRadioNotifications(t *testing.T) { //nolint:dupl // Test runners have similar structure by design
	t.Parallel()

	t.Run("testPitRadioEnabledGetSet", testPitRadioEnabledGetSet)
	t.Run("testPitRadioOutputGetSet", testPitRadioOutputGetSet)
	t.Run("testPitRadioMessageSendIntervalMs", testPitRadioMessageSendIntervalMs)
	t.Run("testPitRadioNotifyRaceProgressEnabled", testPitRadioNotifyRaceProgressEnabled)
	t.Run("testPitRadioNotifyRaceProgressMinLaps", testPitRadioNotifyRaceProgressMinLaps)
	t.Run("testPitRadioNotifyRaceProgressMinLapsIncreaseDecrease", testPitRadioNotifyRaceProgressMinLapsIncreaseDecrease)
	t.Run("testPitRadioNotifyRaceProgressMinLapsClamping", testPitRadioNotifyRaceProgressMinLapsClamping)
	t.Run("testPitRadioNotifyRaceProgressIntervalPc", testPitRadioNotifyRaceProgressIntervalPc)
	t.Run("testPitRadioNotifyRaceProgressIntervalPcClamping", testPitRadioNotifyRaceProgressIntervalPcClamping)
	t.Run("testPitRadioNotifyRaceProgressIntervalPcIncreaseDecrease", testPitRadioNotifyRaceProgressIntervalPcIncreaseDecrease)
	t.Run("testPitRadioNotifyRaceLapsEnabled", testPitRadioNotifyRaceLapsEnabled)
	t.Run("testPitRadioNotifyRaceLapsIntervalLaps", testPitRadioNotifyRaceLapsIntervalLaps)
	t.Run("testPitRadioNotifyRaceLapsIntervalLapsIncreaseDecrease", testPitRadioNotifyRaceLapsIntervalLapsIncreaseDecrease)
	t.Run("testPitRadioNotifyRaceLapsCountdownLaps", testPitRadioNotifyRaceLapsCountdownLaps)
	t.Run("testPitRadioNotifyRaceLapsCountdownLapsIncreaseDecrease", testPitRadioNotifyRaceLapsCountdownLapsIncreaseDecrease)
	t.Run("testPitRadioNotifyLapTimesEnabled", testPitRadioNotifyLapTimesEnabled)
	t.Run("testPitRadioNotifyLapTimesMaxDeltaSeconds", testPitRadioNotifyLapTimesMaxDeltaSeconds)
	t.Run("testPitRadioNotifyLapTimesMaxDeltaSecondsClamping", testPitRadioNotifyLapTimesMaxDeltaSecondsClamping)
	t.Run("testPitRadioNotifyLapTimesMaxDeltaSecondsIncreaseDecrease", testPitRadioNotifyLapTimesMaxDeltaSecondsIncreaseDecrease)
	t.Run("testPitRadioNotifyCircuitMatchingEnabled", testPitRadioNotifyCircuitMatchingEnabled)
}

// ****************************************************************************
// Pit Radio fuel and tyre monitoring tests
// ****************************************************************************

func testPitRadioFuelMonitoringEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is true
	assert.True(t, cfg.GetPitRadioFuelMonitoringEnabled(), 0.001)

	// Act - set false
	cfg.SetPitRadioFuelMonitoringEnabled(false)

	// Assert
	assert.False(t, cfg.GetPitRadioFuelMonitoringEnabled(), 0.001)
}

func testPitRadioFuelPreWarnNotifyLaps(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 2.0
	assert.InDelta(t, 2.0, cfg.GetPitRadioFuelPreWarnNotifyLaps(), 0.001)

	// Act - set new value
	cfg.SetPitRadioFuelPreWarnNotifyLaps(3.5)

	// Assert
	assert.InDelta(t, 3.5, cfg.GetPitRadioFuelPreWarnNotifyLaps(), 0.001)
}

func testPitRadioFuelPreWarnNotifyLapsClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum (0.0)
	cfg.SetPitRadioFuelPreWarnNotifyLaps(-1.0)

	// Assert - should be clamped to 0.0
	assert.InDelta(t, 0.0, cfg.GetPitRadioFuelPreWarnNotifyLaps(), 0.001)

	// Act - set above maximum (10.0)
	cfg.SetPitRadioFuelPreWarnNotifyLaps(15.0)

	// Assert - should be clamped to 10.0
	assert.InDelta(t, 10.0, cfg.GetPitRadioFuelPreWarnNotifyLaps(), 0.001)
}

func testPitRadioFuelPreWarnNotifyLapsIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioFuelPreWarnNotifyLaps(2.0)

	// Act - increase
	result := cfg.IncreasePitRadioFuelPreWarnNotifyLaps()

	// Assert
	assert.InDelta(t, 2.1, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioFuelPreWarnNotifyLaps()

	// Assert
	assert.InDelta(t, 2.0, result, 0.001)
}

func testPitRadioFuelStrategyNotifyLaps(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 5.0
	assert.InDelta(t, 5.0, cfg.GetPitRadioFuelStrategyNotifyLaps(), 0.001)

	// Act - set new value
	cfg.SetPitRadioFuelStrategyNotifyLaps(7.5)

	// Assert
	assert.InDelta(t, 7.5, cfg.GetPitRadioFuelStrategyNotifyLaps(), 0.001)
}

func testPitRadioFuelStrategyNotifyLapsClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum (0.0)
	cfg.SetPitRadioFuelStrategyNotifyLaps(-1.0)

	// Assert - should be clamped to 0.0
	assert.InDelta(t, 0.0, cfg.GetPitRadioFuelStrategyNotifyLaps(), 0.001)

	// Act - set above maximum (20.0)
	cfg.SetPitRadioFuelStrategyNotifyLaps(25.0)

	// Assert - should be clamped to 20.0
	assert.InDelta(t, 20.0, cfg.GetPitRadioFuelStrategyNotifyLaps(), 0.001)
}

func testPitRadioFuelStrategyNotifyLapsIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioFuelStrategyNotifyLaps(5.0)

	// Act - increase
	result := cfg.IncreasePitRadioFuelStrategyNotifyLaps()

	// Assert
	assert.InDelta(t, 5.1, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioFuelStrategyNotifyLaps()

	// Assert
	assert.InDelta(t, 5.0, result, 0.001)
}

func testPitRadioFuelRangeSafetyMarginLaps(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 0.2
	assert.InDelta(t, 0.2, cfg.GetPitRadioFuelRangeSafetyMarginLaps(), 0.001)

	// Act - set new value
	cfg.SetPitRadioFuelRangeSafetyMarginLaps(0.5)

	// Assert
	assert.InDelta(t, 0.5, cfg.GetPitRadioFuelRangeSafetyMarginLaps(), 0.001)
}

func testPitRadioFuelRangeSafetyMarginLapsClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum (0.0)
	cfg.SetPitRadioFuelRangeSafetyMarginLaps(-1.0)

	// Assert - should be clamped to 0.0
	assert.InDelta(t, 0.0, cfg.GetPitRadioFuelRangeSafetyMarginLaps(), 0.001)

	// Act - set above maximum (2.0)
	cfg.SetPitRadioFuelRangeSafetyMarginLaps(5.0)

	// Assert - should be clamped to 2.0
	assert.InDelta(t, 2.0, cfg.GetPitRadioFuelRangeSafetyMarginLaps(), 0.001)
}

func testPitRadioFuelRangeSafetyMarginLapsIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioFuelRangeSafetyMarginLaps(0.5)

	// Act - increase
	result := cfg.IncreasePitRadioFuelRangeSafetyMarginLaps()

	// Assert
	assert.InDelta(t, 0.55, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioFuelRangeSafetyMarginLaps()

	// Assert
	assert.InDelta(t, 0.5, result, 0.001)
}

func testPitRadioFuelRangeSafetyMarginMetres(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 750
	assert.InDelta(t, 750.0, cfg.GetPitRadioFuelRangeSafetyMarginMetres(), 0.001)

	// Act - set new value
	cfg.SetPitRadioFuelRangeSafetyMarginMetres(1000)

	// Assert
	assert.InDelta(t, 1000.0, cfg.GetPitRadioFuelRangeSafetyMarginMetres(), 0.001)
}

func testPitRadioFuelRangeSafetyMarginMetresClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum (0)
	cfg.SetPitRadioFuelRangeSafetyMarginMetres(-100)

	// Assert - should be clamped to 0
	assert.InDelta(t, 0.0, cfg.GetPitRadioFuelRangeSafetyMarginMetres(), 0.001)

	// Act - set above maximum (2000)
	cfg.SetPitRadioFuelRangeSafetyMarginMetres(3000)

	// Assert - should be clamped to 2000
	assert.InDelta(t, 2000.0, cfg.GetPitRadioFuelRangeSafetyMarginMetres(), 0.001)
}

func testPitRadioFuelRangeSafetyMarginMetresIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioFuelRangeSafetyMarginMetres(750)

	// Act - increase
	result := cfg.IncreasePitRadioFuelRangeSafetyMarginMetres()

	// Assert
	assert.InDelta(t, 800.0, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioFuelRangeSafetyMarginMetres()

	// Assert
	assert.InDelta(t, 750.0, result, 0.001)
}

func testPitRadioTyreMonitoringEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is true
	assert.True(t, cfg.GetPitRadioTyreMonitoringEnabled(), 0.001)

	// Act - set false
	cfg.SetPitRadioTyreMonitoringEnabled(false)

	// Assert
	assert.False(t, cfg.GetPitRadioTyreMonitoringEnabled(), 0.001)
}

func testPitRadioTyreTemperatureOptimalCelsius(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 81
	assert.InDelta(t, 81, cfg.GetPitRadioTyreTemperatureOptimalCelsius(), 0.001)

	// Act - set new value
	cfg.SetPitRadioTyreTemperatureOptimalCelsius(85)

	// Assert
	assert.InDelta(t, 85, cfg.GetPitRadioTyreTemperatureOptimalCelsius(), 0.001)
}

func testPitRadioTyreTemperatureOptimalCelsiusClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum (60)
	cfg.SetPitRadioTyreTemperatureOptimalCelsius(50)

	// Assert - should be clamped to 60
	assert.InDelta(t, 60, cfg.GetPitRadioTyreTemperatureOptimalCelsius(), 0.001)

	// Act - set above maximum (120)
	cfg.SetPitRadioTyreTemperatureOptimalCelsius(130)

	// Assert - should be clamped to 120
	assert.InDelta(t, 120, cfg.GetPitRadioTyreTemperatureOptimalCelsius(), 0.001)
}

func testPitRadioTyreTemperatureOptimalCelsiusIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioTyreTemperatureOptimalCelsius(85)

	// Act - increase
	result := cfg.IncreasePitRadioTyreTemperatureOptimalCelsius()

	// Assert
	assert.InDelta(t, 86, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioTyreTemperatureOptimalCelsius()

	// Assert
	assert.InDelta(t, 85, result, 0.001)
}

func testPitRadioTyreTemperatureOperatingWindow(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 6
	assert.InDelta(t, 6, cfg.GetPitRadioTyreTemperatureOperatingWindow(), 0.001)

	// Act - set new value
	cfg.SetPitRadioTyreTemperatureOperatingWindow(8)

	// Assert
	assert.InDelta(t, 8, cfg.GetPitRadioTyreTemperatureOperatingWindow(), 0.001)
}

func testPitRadioTyreTemperatureOperatingWindowClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum (0.5)
	cfg.SetPitRadioTyreTemperatureOperatingWindow(0.1)

	// Assert - should be clamped to 0.5
	assert.InDelta(t, 0.5, cfg.GetPitRadioTyreTemperatureOperatingWindow(), 0.001)

	// Act - set above maximum (20.0)
	cfg.SetPitRadioTyreTemperatureOperatingWindow(25)

	// Assert - should be clamped to 20.0
	assert.InDelta(t, 20, cfg.GetPitRadioTyreTemperatureOperatingWindow(), 0.001)
}

func testPitRadioTyreTemperatureOperatingWindowIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioTyreTemperatureOperatingWindow(6)

	// Act - increase
	result := cfg.IncreasePitRadioTyreTemperatureOperatingWindow()

	// Assert
	assert.InDelta(t, 6.5, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioTyreTemperatureOperatingWindow()

	// Assert
	assert.InDelta(t, 6, result, 0.001)
}

func testPitRadioTyreTemperatureMarginCelsius(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 3
	assert.InDelta(t, 3, cfg.GetPitRadioTyreTemperatureMarginCelsius(), 0.001)

	// Act - set new value
	cfg.SetPitRadioTyreTemperatureMarginCelsius(5)

	// Assert
	assert.InDelta(t, 5, cfg.GetPitRadioTyreTemperatureMarginCelsius(), 0.001)
}

func testPitRadioTyreTemperatureMarginCelsiusClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum (0.5)
	cfg.SetPitRadioTyreTemperatureMarginCelsius(0.1)

	// Assert - should be clamped to 0.5
	assert.InDelta(t, 0.5, cfg.GetPitRadioTyreTemperatureMarginCelsius(), 0.001)

	// Act - set above maximum (10.0)
	cfg.SetPitRadioTyreTemperatureMarginCelsius(15)

	// Assert - should be clamped to 10.0
	assert.InDelta(t, 10, cfg.GetPitRadioTyreTemperatureMarginCelsius(), 0.001)
}

func testPitRadioTyreTemperatureMarginCelsiusIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetPitRadioTyreTemperatureMarginCelsius(5)

	// Act - increase
	result := cfg.IncreasePitRadioTyreTemperatureMarginCelsius()

	// Assert
	assert.InDelta(t, 5.5, result, 0.001)

	// Act - decrease
	result = cfg.DecreasePitRadioTyreTemperatureMarginCelsius()

	// Assert
	assert.InDelta(t, 5, result, 0.001)
}

func TestPitRadioFuelAndTyreMonitoring(t *testing.T) {
	t.Parallel()

	t.Run("testPitRadioFuelMonitoringEnabled", testPitRadioFuelMonitoringEnabled)
	t.Run("testPitRadioFuelPreWarnNotifyLaps", testPitRadioFuelPreWarnNotifyLaps)
	t.Run("testPitRadioFuelPreWarnNotifyLapsClamping", testPitRadioFuelPreWarnNotifyLapsClamping)
	t.Run("testPitRadioFuelPreWarnNotifyLapsIncreaseDecrease", testPitRadioFuelPreWarnNotifyLapsIncreaseDecrease)
	t.Run("testPitRadioFuelStrategyNotifyLaps", testPitRadioFuelStrategyNotifyLaps)
	t.Run("testPitRadioFuelStrategyNotifyLapsClamping", testPitRadioFuelStrategyNotifyLapsClamping)
	t.Run("testPitRadioFuelStrategyNotifyLapsIncreaseDecrease", testPitRadioFuelStrategyNotifyLapsIncreaseDecrease)
	t.Run("testPitRadioFuelRangeSafetyMarginLaps", testPitRadioFuelRangeSafetyMarginLaps)
	t.Run("testPitRadioFuelRangeSafetyMarginLapsClamping", testPitRadioFuelRangeSafetyMarginLapsClamping)
	t.Run("testPitRadioFuelRangeSafetyMarginLapsIncreaseDecrease", testPitRadioFuelRangeSafetyMarginLapsIncreaseDecrease)
	t.Run("testPitRadioFuelRangeSafetyMarginMetres", testPitRadioFuelRangeSafetyMarginMetres)
	t.Run("testPitRadioFuelRangeSafetyMarginMetresClamping", testPitRadioFuelRangeSafetyMarginMetresClamping)
	t.Run("testPitRadioFuelRangeSafetyMarginMetresIncreaseDecrease", testPitRadioFuelRangeSafetyMarginMetresIncreaseDecrease)
	t.Run("testPitRadioTyreMonitoringEnabled", testPitRadioTyreMonitoringEnabled)
	t.Run("testPitRadioTyreTemperatureOptimalCelsius", testPitRadioTyreTemperatureOptimalCelsius)
	t.Run("testPitRadioTyreTemperatureOptimalCelsiusClamping", testPitRadioTyreTemperatureOptimalCelsiusClamping)
	t.Run("testPitRadioTyreTemperatureOptimalCelsiusIncreaseDecrease", testPitRadioTyreTemperatureOptimalCelsiusIncreaseDecrease)
	t.Run("testPitRadioTyreTemperatureOperatingWindow", testPitRadioTyreTemperatureOperatingWindow)
	t.Run("testPitRadioTyreTemperatureOperatingWindowClamping", testPitRadioTyreTemperatureOperatingWindowClamping)
	t.Run("testPitRadioTyreTemperatureOperatingWindowIncreaseDecrease", testPitRadioTyreTemperatureOperatingWindowIncreaseDecrease)
	t.Run("testPitRadioTyreTemperatureMarginCelsius", testPitRadioTyreTemperatureMarginCelsius)
	t.Run("testPitRadioTyreTemperatureMarginCelsiusClamping", testPitRadioTyreTemperatureMarginCelsiusClamping)
	t.Run("testPitRadioTyreTemperatureMarginCelsiusIncreaseDecrease", testPitRadioTyreTemperatureMarginCelsiusIncreaseDecrease)
}

// ****************************************************************************
// Discord section tests
// ****************************************************************************

func testDiscordToken(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is empty
	assert.Empty(t, cfg.GetDiscordToken(), 0.001)

	// Act - set new value
	cfg.SetDiscordToken("test-token-123")

	// Assert
	assert.Equal(t, "test-token-123", cfg.GetDiscordToken(), 0.001)
}

func testDiscordGuildID(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is empty
	assert.Empty(t, cfg.GetDiscordGuildID(), 0.001)

	// Act - set new value
	cfg.SetDiscordGuildID("guild-123")

	// Assert
	assert.Equal(t, "guild-123", cfg.GetDiscordGuildID(), 0.001)
}

func testDiscordChannelID(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is empty
	assert.Empty(t, cfg.GetDiscordChannelID(), 0.001)

	// Act - set new value
	cfg.SetDiscordChannelID("channel-456")

	// Assert
	assert.Equal(t, "channel-456", cfg.GetDiscordChannelID(), 0.001)
}

func testDiscordVoiceChannelID(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is empty
	assert.Empty(t, cfg.GetDiscordVoiceChannelID(), 0.001)

	// Act - set new value
	cfg.SetDiscordVoiceChannelID("voice-789")

	// Assert
	assert.Equal(t, "voice-789", cfg.GetDiscordVoiceChannelID(), 0.001)
}

func TestDiscordSection(t *testing.T) {
	t.Parallel()

	t.Run("testDiscordToken", testDiscordToken)
	t.Run("testDiscordGuildID", testDiscordGuildID)
	t.Run("testDiscordChannelID", testDiscordChannelID)
	t.Run("testDiscordVoiceChannelID", testDiscordVoiceChannelID)
}

// ****************************************************************************
// Telemetry section tests
// ****************************************************************************

func testTelemetrySource(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set new value
	cfg.SetTelemetrySource("gt7")

	// Assert
	assert.Equal(t, "gt7", cfg.GetTelemetrySource(), 0.001)
}

func TestTelemetrySection(t *testing.T) {
	t.Parallel()

	t.Run("testTelemetrySource", testTelemetrySource)
}

// ****************************************************************************
// Synthesizer gain tests
// ****************************************************************************

func testSynthesizer(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert
	synth := cfg.GetSynthesizer()
	require.NotNil(t, synth)
	assert.Equal(t, 8000, synth.InternalSampleRateHz)
	assert.Equal(t, 32000, synth.OutputSampleRateHz)
}

func testSynthInternalSampleRateHz(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 8000
	assert.InDelta(t, 8000, cfg.GetSynthInternalSampleRateHz(), 0.001)

	// Act - set new value
	cfg.SetSynthInternalSampleRateHz(16000)

	// Assert
	assert.InDelta(t, 16000, cfg.GetSynthInternalSampleRateHz(), 0.001)
}

func testSynthOutputSampleRateHz(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 32000
	assert.InDelta(t, 32000, cfg.GetSynthOutputSampleRateHz(), 0.001)

	// Act - set new value
	cfg.SetSynthOutputSampleRateHz(44100)

	// Assert
	assert.InDelta(t, 44100, cfg.GetSynthOutputSampleRateHz(), 0.001)
}

func testSynthGainIncrement(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 0.25
	assert.InDelta(t, 0.25, cfg.GetSynthGainIncrement(), 0.001)

	// Act - set new value
	cfg.SetSynthGainIncrement(0.5)

	// Assert
	assert.InDelta(t, 0.5, cfg.GetSynthGainIncrement(), 0.001)
}

func testSynthGainIncrementClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum
	cfg.SetSynthGainIncrement(0.001)

	// Assert - should be clamped to 0.01
	assert.InDelta(t, 0.01, cfg.GetSynthGainIncrement(), 0.001)

	// Act - set above maximum
	cfg.SetSynthGainIncrement(15)

	// Assert - should be clamped to 10
	assert.InDelta(t, 10.0, cfg.GetSynthGainIncrement(), 0.001)
}

func testSynthMasterGain(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 0.0
	assert.InDelta(t, 0.0, cfg.GetSynthMasterGain(), 0.001)

	// Act - set new value
	cfg.SetSynthMasterGain(-10.0)

	// Assert
	assert.InDelta(t, -10.0, cfg.GetSynthMasterGain(), 0.001)
}

func testSynthMasterGainClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum (-60)
	cfg.SetSynthMasterGain(-100)

	// Assert - should be clamped to MinimumGain
	assert.InDelta(t, MinimumGain, cfg.GetSynthMasterGain(), 0.001)

	// Act - set above maximum (0)
	cfg.SetSynthMasterGain(10)

	// Assert - should be clamped to MaximumGain
	assert.InDelta(t, MaximumGain, cfg.GetSynthMasterGain(), 0.001)
}

func testSynthMasterGainIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetSynthMasterGain(-10.0)

	// Act - increase
	result := cfg.IncreaseSynthMasterGain()

	// Assert
	assert.InDelta(t, -9.75, result, 0.001)

	// Act - decrease
	result = cfg.DecreaseSynthMasterGain()

	// Assert
	assert.InDelta(t, -10.0, result, 0.001)
}

func testSynthMasterMute(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.GetSynthMasterMute(), 0.001)

	// Act - set true
	cfg.SetSynthMasterMute(true)

	// Assert
	assert.True(t, cfg.GetSynthMasterMute(), 0.001)
}

func testSynthChassisGain(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 0.0
	assert.InDelta(t, 0.0, cfg.GetSynthChassisGain(), 0.001)

	// Act - set new value
	cfg.SetSynthChassisGain(-15.0)

	// Assert
	assert.InDelta(t, -15.0, cfg.GetSynthChassisGain(), 0.001)
}

func testSynthChassisGainIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetSynthChassisGain(-10.0)

	// Act - increase
	result := cfg.IncreaseSynthChassisGain()

	// Assert
	assert.InDelta(t, -9.75, result, 0.001)

	// Act - decrease
	result = cfg.DecreaseSynthChassisGain()

	// Assert
	assert.InDelta(t, -10.0, result, 0.001)
}

func testSynthChassisMute(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.GetSynthChassisMute(), 0.001)

	// Act - set true
	cfg.SetSynthChassisMute(true)

	// Assert
	assert.True(t, cfg.GetSynthChassisMute(), 0.001)
}

func testSynthTransmissionGain(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is 0.0
	assert.InDelta(t, 0.0, cfg.GetSynthTransmissionGain(), 0.001)

	// Act - set new value
	cfg.SetSynthTransmissionGain(-20.0)

	// Assert
	assert.InDelta(t, -20.0, cfg.GetSynthTransmissionGain(), 0.001)
}

func testSynthTransmissionGainIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetSynthTransmissionGain(-10.0)

	// Act - increase
	result := cfg.IncreaseSynthTransmissionGain()

	// Assert
	assert.InDelta(t, -9.75, result, 0.001)

	// Act - decrease
	result = cfg.DecreaseSynthTransmissionGain()

	// Assert
	assert.InDelta(t, -10.0, result, 0.001)
}

func testSynthTransmissionMute(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.GetSynthTransmissionMute(), 0.001)

	// Act - set true
	cfg.SetSynthTransmissionMute(true)

	// Assert
	assert.True(t, cfg.GetSynthTransmissionMute(), 0.001)
}

func testSynthTransmissionGainMinRace(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is -3.0
	assert.InDelta(t, -3.0, cfg.GetSynthTransmissionGainMinRace(), 0.001)

	// Act - set new value
	cfg.SetSynthTransmissionGainMinRace(-5.0)

	// Assert
	assert.InDelta(t, -5.0, cfg.GetSynthTransmissionGainMinRace(), 0.001)
}

func testSynthTransmissionGainMinRaceIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetSynthTransmissionGainMinRace(-3.0)

	// Act - increase
	result := cfg.IncreaseSynthTransmissionGainMinRace()

	// Assert
	assert.InDelta(t, -2.75, result, 0.001)

	// Act - decrease
	result = cfg.DecreaseSynthTransmissionGainMinRace()

	// Assert
	assert.InDelta(t, -3.0, result, 0.001)
}

func testSynthTransmissionGainMinStreet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is -6.0
	assert.InDelta(t, -6.0, cfg.GetSynthTransmissionGainMinStreet(), 0.001)

	// Act - set new value
	cfg.SetSynthTransmissionGainMinStreet(-8.0)

	// Assert
	assert.InDelta(t, -8.0, cfg.GetSynthTransmissionGainMinStreet(), 0.001)
}

func testSynthTransmissionGainMinStreetIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetSynthTransmissionGainMinStreet(-6.0)

	// Act - increase
	result := cfg.IncreaseSynthTransmissionGainMinStreet()

	// Assert
	assert.InDelta(t, -5.75, result, 0.001)

	// Act - decrease
	result = cfg.DecreaseSynthTransmissionGainMinStreet()

	// Assert
	assert.InDelta(t, -6.0, result, 0.001)
}

func testSynthEngineGain(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is -4.25
	assert.InDelta(t, -4.25, cfg.GetSynthEngineGain(), 0.001)

	// Act - set new value
	cfg.SetSynthEngineGain(-10.0)

	// Assert
	assert.InDelta(t, -10.0, cfg.GetSynthEngineGain(), 0.001)
}

func testSynthEngineGainIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetSynthEngineGain(-10.0)

	// Act - increase
	result := cfg.IncreaseSynthEngineGain()

	// Assert
	assert.InDelta(t, -9.75, result, 0.001)

	// Act - decrease
	result = cfg.DecreaseSynthEngineGain()

	// Assert
	assert.InDelta(t, -10.0, result, 0.001)
}

func testSynthEngineMute(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.GetSynthEngineMute(), 0.001)

	// Act - set true
	cfg.SetSynthEngineMute(true)

	// Assert
	assert.True(t, cfg.GetSynthEngineMute(), 0.001)
}

func testSynthOutputFile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is empty
	assert.Empty(t, cfg.GetSynthOutputFile(), 0.001)
}

func testSynthEngineProfiles(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act
	profiles := cfg.GetSynthEngineProfiles()

	// Assert
	require.NotNil(t, profiles)
	assert.NotEmpty(t, profiles)

	// Check for known profile
	i4Profile, exists := profiles["i4"]
	assert.True(t, exists)
	assert.InDelta(t, 0.76, i4Profile.PrimaryBalance, 0.001)
}

func TestSynthesizerGains(t *testing.T) { //nolint:dupl // Test runners have similar structure by design
	t.Parallel()

	t.Run("testSynthesizer", testSynthesizer)
	t.Run("testSynthInternalSampleRateHz", testSynthInternalSampleRateHz)
	t.Run("testSynthOutputSampleRateHz", testSynthOutputSampleRateHz)
	t.Run("testSynthGainIncrement", testSynthGainIncrement)
	t.Run("testSynthGainIncrementClamping", testSynthGainIncrementClamping)
	t.Run("testSynthMasterGain", testSynthMasterGain)
	t.Run("testSynthMasterGainClamping", testSynthMasterGainClamping)
	t.Run("testSynthMasterGainIncreaseDecrease", testSynthMasterGainIncreaseDecrease)
	t.Run("testSynthMasterMute", testSynthMasterMute)
	t.Run("testSynthChassisGain", testSynthChassisGain)
	t.Run("testSynthChassisGainIncreaseDecrease", testSynthChassisGainIncreaseDecrease)
	t.Run("testSynthChassisMute", testSynthChassisMute)
	t.Run("testSynthTransmissionGain", testSynthTransmissionGain)
	t.Run("testSynthTransmissionGainIncreaseDecrease", testSynthTransmissionGainIncreaseDecrease)
	t.Run("testSynthTransmissionMute", testSynthTransmissionMute)
	t.Run("testSynthTransmissionGainMinRace", testSynthTransmissionGainMinRace)
	t.Run("testSynthTransmissionGainMinRaceIncreaseDecrease", testSynthTransmissionGainMinRaceIncreaseDecrease)
	t.Run("testSynthTransmissionGainMinStreet", testSynthTransmissionGainMinStreet)
	t.Run("testSynthTransmissionGainMinStreetIncreaseDecrease", testSynthTransmissionGainMinStreetIncreaseDecrease)
	t.Run("testSynthEngineGain", testSynthEngineGain)
	t.Run("testSynthEngineGainIncreaseDecrease", testSynthEngineGainIncreaseDecrease)
	t.Run("testSynthEngineMute", testSynthEngineMute)
	t.Run("testSynthOutputFile", testSynthOutputFile)
	t.Run("testSynthEngineProfiles", testSynthEngineProfiles)
}

// ****************************************************************************
// Synthesizer channel and EQ tests
// ****************************************************************************

func testSynthChannelGain(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is -30.0 for both channels
	assert.InDelta(t, -30.0, cfg.GetSynthChannelGain(0), 0.001)
	assert.InDelta(t, -30.0, cfg.GetSynthChannelGain(1), 0.001)

	// Act - set new value
	cfg.SetSynthChannelGain(0, -20.0)

	// Assert
	assert.InDelta(t, -20.0, cfg.GetSynthChannelGain(0), 0.001)
	assert.InDelta(t, -30.0, cfg.GetSynthChannelGain(1), 0.001)
}

func testSynthChannelGainInvalidChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - invalid channel returns 0.0
	assert.InDelta(t, 0.0, cfg.GetSynthChannelGain(-1), 0.001)
	assert.InDelta(t, 0.0, cfg.GetSynthChannelGain(99), 0.001)

	// Act - set invalid channel (should be a no-op)
	cfg.SetSynthChannelGain(-1, -10.0)
	cfg.SetSynthChannelGain(99, -10.0)

	// Assert - valid channels unchanged
	assert.InDelta(t, -30.0, cfg.GetSynthChannelGain(0), 0.001)
}

func testSynthChannelGainClamping(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - set below minimum
	cfg.SetSynthChannelGain(0, -100)

	// Assert - should be clamped to MinimumGain
	assert.InDelta(t, MinimumGain, cfg.GetSynthChannelGain(0), 0.001)

	// Act - set above maximum
	cfg.SetSynthChannelGain(0, 10)

	// Assert - should be clamped to MaximumGain
	assert.InDelta(t, MaximumGain, cfg.GetSynthChannelGain(0), 0.001)
}

func testSynthChannelGainIncreaseDecrease(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	cfg.SetSynthChannelGain(0, -10.0)
	cfg.SetSynthChannelGain(1, -15.0)

	// Act - increase channel 0
	result := cfg.IncreaseSynthChannelGain(0)

	// Assert
	assert.InDelta(t, -9.75, result, 0.001)

	// Act - decrease channel 0
	result = cfg.DecreaseSynthChannelGain(0)

	// Assert
	assert.InDelta(t, -10.0, result, 0.001)

	// Act - increase channel 1 (check boundary handling)
	result = cfg.IncreaseSynthChannelGain(1)

	// Assert
	assert.InDelta(t, -14.75, result, 0.001)

	// Act - decrease channel 1
	result = cfg.DecreaseSynthChannelGain(1)

	// Assert
	assert.InDelta(t, -15.0, result, 0.001)
}

func testSynthChannelGainIncreaseDecreaseInvalidChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - invalid channel returns 0.0
	result := cfg.IncreaseSynthChannelGain(-1)
	assert.InDelta(t, 0.0, result, 0.001)

	result = cfg.DecreaseSynthChannelGain(99)
	assert.InDelta(t, 0.0, result, 0.001)
}

func testSynthChannelMute(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.GetSynthChannelMute(0))
	assert.False(t, cfg.GetSynthChannelMute(1))

	// Act - set channel 0 true
	cfg.SetSynthChannelMute(0, true)

	// Assert
	assert.True(t, cfg.GetSynthChannelMute(0))
	assert.False(t, cfg.GetSynthChannelMute(1))

	// Act - set channel 1 true (check boundary handling)
	cfg.SetSynthChannelMute(1, true)

	// Assert
	assert.True(t, cfg.GetSynthChannelMute(0))
	assert.True(t, cfg.GetSynthChannelMute(1))
}

func testSynthChannelMuteInvalidChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - invalid channel returns false
	assert.False(t, cfg.GetSynthChannelMute(-1))
	assert.False(t, cfg.GetSynthChannelMute(99))

	// Act - set invalid channel (should be a no-op)
	cfg.SetSynthChannelMute(-1, true)
	cfg.SetSynthChannelMute(99, true)

	// Assert - valid channels unchanged
	assert.False(t, cfg.GetSynthChannelMute(0))
}

func testSynthChannelEqEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - default is false
	assert.False(t, cfg.GetSynthChannelEqEnabled(0))
	assert.False(t, cfg.GetSynthChannelEqEnabled(1))

	// Act - set channel 0 true
	cfg.SetSynthChannelEqEnabled(0, true)

	// Assert
	assert.True(t, cfg.GetSynthChannelEqEnabled(0))
	assert.False(t, cfg.GetSynthChannelEqEnabled(1))

	// Act - set channel 1 true (check boundary handling)
	cfg.SetSynthChannelEqEnabled(1, true)

	// Assert
	assert.True(t, cfg.GetSynthChannelEqEnabled(0))
	assert.True(t, cfg.GetSynthChannelEqEnabled(1))
}

func testSynthChannelEqEnabledInvalidChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - invalid channel returns false
	assert.False(t, cfg.GetSynthChannelEqEnabled(-1))
	assert.False(t, cfg.GetSynthChannelEqEnabled(99))
}

func testSynthChannelEq(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - test channel 0
	bands := cfg.GetSynthChannelEq(0)

	// Assert
	require.NotNil(t, bands)
	assert.Len(t, bands, 8)

	// Act - test channel 1 (check boundary handling)
	bands1 := cfg.GetSynthChannelEq(1)

	// Assert
	require.NotNil(t, bands1)
	assert.Len(t, bands1, 8)
}

func testSynthChannelEqInvalidChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act & Assert - invalid channel returns nil
	assert.Nil(t, cfg.GetSynthChannelEq(-1))
	assert.Nil(t, cfg.GetSynthChannelEq(99))
}

func testSynthChannelEqSet(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	newBands := []EQBand{
		{Frequency: 10, Gain: 2.0, Q: 2.0},
		{Frequency: 15, Gain: 1.0, Q: 2.0},
		{Frequency: 20, Gain: 0.0, Q: 2.0},
		{Frequency: 25, Gain: -1.0, Q: 2.0},
		{Frequency: 30, Gain: -2.0, Q: 2.0},
		{Frequency: 40, Gain: 0.0, Q: 2.0},
		{Frequency: 50, Gain: 1.0, Q: 2.0},
		{Frequency: 60, Gain: 2.0, Q: 2.0},
	}

	// Act - set channel 0
	cfg.SetSynthChannelEq(0, newBands)

	// Assert
	result := cfg.GetSynthChannelEq(0)
	require.NotNil(t, result)
	assert.Len(t, result, 8)
	assert.InDelta(t, 2.0, result[0].Gain, 0.001)
	assert.InDelta(t, -2.0, result[4].Gain, 0.001)

	// Act - set channel 1 (check boundary handling)
	newBandsCh1 := []EQBand{
		{Frequency: 10, Gain: 3.0, Q: 2.0},
		{Frequency: 15, Gain: 2.0, Q: 2.0},
		{Frequency: 20, Gain: 1.0, Q: 2.0},
		{Frequency: 25, Gain: 0.0, Q: 2.0},
		{Frequency: 30, Gain: -1.0, Q: 2.0},
		{Frequency: 40, Gain: -2.0, Q: 2.0},
		{Frequency: 50, Gain: 1.0, Q: 2.0},
		{Frequency: 60, Gain: 0.0, Q: 2.0},
	}
	cfg.SetSynthChannelEq(1, newBandsCh1)

	// Assert
	result1 := cfg.GetSynthChannelEq(1)
	require.NotNil(t, result1)
	assert.Len(t, result1, 8)
	assert.InDelta(t, 3.0, result1[0].Gain, 0.001)
	assert.InDelta(t, -1.0, result1[4].Gain, 0.001)
}

func testSynthChannelEqSetInvalidBandCount(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	originalBands := cfg.GetSynthChannelEq(0)
	wrongBands := []EQBand{
		{Frequency: 10, Gain: 2.0, Q: 2.0},
		{Frequency: 15, Gain: 1.0, Q: 2.0},
	} // Only 2 bands instead of 8

	// Act
	cfg.SetSynthChannelEq(0, wrongBands)

	// Assert - bands should be unchanged
	result := cfg.GetSynthChannelEq(0)
	assert.InDelta(t, originalBands[0].Frequency, result[0].Frequency, 0.001)
}

func testSynthChannelEqCurve(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act - test channel 0
	curve, minFreq, resolution := cfg.GetSynthChannelEqCurve(0)

	// Assert
	require.NotNil(t, curve)
	assert.NotEmpty(t, curve)
	assert.Greater(t, minFreq, 0.0)
	assert.Greater(t, resolution, 0.0)

	// Act - test channel 1 (check boundary handling)
	curve1, minFreq1, resolution1 := cfg.GetSynthChannelEqCurve(1)

	// Assert
	require.NotNil(t, curve1)
	assert.NotEmpty(t, curve1)
	assert.Greater(t, minFreq1, 0.0)
	assert.Greater(t, resolution1, 0.0)
}

func testSynthChannelEqCurveInvalidChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act
	curve, minFreq, resolution := cfg.GetSynthChannelEqCurve(-1)

	// Assert - should return nil/zero
	assert.Nil(t, curve)
	assert.InDelta(t, 0.0, minFreq, 0.001)
	assert.InDelta(t, 0.0, resolution, 0.001)
}

func testSynthChannelsEqEnabled(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act
	enabled := cfg.GetSynthChannelsEqEnabled()

	// Assert
	require.NotNil(t, enabled)
	assert.Len(t, enabled, 2)
}

func testSynthChannelsEq(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act
	bands := cfg.GetSynthChannelsEq()

	// Assert
	require.NotNil(t, bands)
	assert.Len(t, bands, 2)
	assert.Len(t, bands[0], 8)
	assert.Len(t, bands[1], 8)
}

func testSynthChannelsEqCurve(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()

	// Act
	curves, minFreq, resolution := cfg.GetSynthChannelsEqCurve()

	// Assert
	require.NotNil(t, curves)
	assert.Len(t, curves, 2)
	assert.Greater(t, minFreq, 0.0)
	assert.Greater(t, resolution, 0.0)
}

func testSetSynthEngineProfile(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := newTestConfig()
	newProfile := appHaptics.EngineProfile{
		PrimaryBalance:   0.85,
		SecondaryBalance: 0.90,
		Gain:             -5.0,
		PulseScale:       0.50,
	}

	// Act
	cfg.SetSynthEngineProfile("custom", newProfile)

	// Assert
	profiles := cfg.GetSynthEngineProfiles()
	customProfile, exists := profiles["custom"]
	require.True(t, exists)
	assert.InDelta(t, 0.85, customProfile.PrimaryBalance, 0.001)
	assert.InDelta(t, 0.90, customProfile.SecondaryBalance, 0.001)
	assert.InDelta(t, -5.0, customProfile.Gain, 0.001)
	assert.InDelta(t, 0.50, customProfile.PulseScale, 0.001)
}

func TestSynthesizerChannelAndEQ(t *testing.T) {
	t.Parallel()

	t.Run("testSynthChannelGain", testSynthChannelGain)
	t.Run("testSynthChannelGainInvalidChannel", testSynthChannelGainInvalidChannel)
	t.Run("testSynthChannelGainClamping", testSynthChannelGainClamping)
	t.Run("testSynthChannelGainIncreaseDecrease", testSynthChannelGainIncreaseDecrease)
	t.Run("testSynthChannelGainIncreaseDecreaseInvalidChannel", testSynthChannelGainIncreaseDecreaseInvalidChannel)
	t.Run("testSynthChannelMute", testSynthChannelMute)
	t.Run("testSynthChannelMuteInvalidChannel", testSynthChannelMuteInvalidChannel)
	t.Run("testSynthChannelEqEnabled", testSynthChannelEqEnabled)
	t.Run("testSynthChannelEqEnabledInvalidChannel", testSynthChannelEqEnabledInvalidChannel)
	t.Run("testSynthChannelEq", testSynthChannelEq)
	t.Run("testSynthChannelEqInvalidChannel", testSynthChannelEqInvalidChannel)
	t.Run("testSynthChannelEqSet", testSynthChannelEqSet)
	t.Run("testSynthChannelEqSetInvalidBandCount", testSynthChannelEqSetInvalidBandCount)
	t.Run("testSynthChannelEqCurve", testSynthChannelEqCurve)
	t.Run("testSynthChannelEqCurveInvalidChannel", testSynthChannelEqCurveInvalidChannel)
	t.Run("testSynthChannelsEqEnabled", testSynthChannelsEqEnabled)
	t.Run("testSynthChannelsEq", testSynthChannelsEq)
	t.Run("testSynthChannelsEqCurve", testSynthChannelsEqCurve)
	t.Run("testSetSynthEngineProfile", testSetSynthEngineProfile)
}

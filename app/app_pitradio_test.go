package app //nolint:testpackage // white-box testing

import (
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	gttelemetry "github.com/zetetos/gt-telemetry/v2"
	"github.com/zetetos/gt-telemetry/v2/pkg/models"
	"github.com/zetetos/simtezilo/app/circuit"
	"github.com/zetetos/simtezilo/app/config"
	"github.com/zetetos/simtezilo/app/fuelrange"
	"github.com/zetetos/simtezilo/app/i18n"
	"github.com/zetetos/simtezilo/app/pitradio"
)

// --- Mocks and stubs ---

type pitRadioMock struct {
	messages []pitradio.Message
	failSend bool
}

func (m *pitRadioMock) BackgroundTask() {}
func (m *pitRadioMock) Close() error    { return nil }
func (m *pitRadioMock) Send(msg pitradio.Message) error {
	m.messages = append(m.messages, msg)

	if m.failSend {
		return errors.New("send failed")
	}

	return nil
}

// createTestI18n creates an i18n instance with test language data.
func createTestI18n() *i18n.I18n {
	// Create I18n instance with default English language
	// This will use the actual language files from the system
	langCode := "en"

	testI18n, err := i18n.New(&langCode, zerolog.Nop())
	if err != nil {
		panic("Failed to create test i18n: " + err.Error())
	}

	return testI18n
}

// fuelRangeMock implements fuelrange.Estimator interface for testing.
type fuelRangeMock struct {
	distanceMetres float64
	distanceLaps   float64
	usageRate      float64
}

// Ensure fuelRangeMock implements fuelrange.Estimator interface.
var _ fuelrange.Estimator = (*fuelRangeMock)(nil)

func (m *fuelRangeMock) Reset()                      {}
func (m *fuelRangeMock) ResetEstimate()              {}
func (m *fuelRangeMock) IsReady() bool               { return true }
func (m *fuelRangeMock) SetLive(_ bool)              {}
func (m *fuelRangeMock) Update(_ float64, _ float32) {}

func (m *fuelRangeMock) DistanceMetres() float64 {
	return m.distanceMetres
}

func (m *fuelRangeMock) DistanceLaps(lengthMetres float64) float64 {
	if m.distanceLaps > 0 {
		return m.distanceLaps
	}

	if lengthMetres > 0 && m.distanceMetres > 0 {
		return m.distanceMetres / lengthMetres
	}

	return 0
}

func (m *fuelRangeMock) UsageRatePerKm() float64 {
	return m.usageRate
}

// SetFuelRange allows tests to configure fuel range values.
func (m *fuelRangeMock) SetFuelRange(distanceMetres, distanceLaps, usageRatePerKm float64) {
	m.distanceMetres = distanceMetres
	m.distanceLaps = distanceLaps
	m.usageRate = usageRatePerKm
}

// circuitMock implements circuit.Manager interface for testing.
type circuitMock struct {
	name                 string
	lengthMetres         float64
	lapProgress          float64
	lapProgressRemaining float64
}

// Ensure circuitMock implements circuit.Manager interface.
var _ circuit.Manager = (*circuitMock)(nil)

func (m *circuitMock) Reset()                {}
func (m *circuitMock) ResetLapProgress()     {}
func (m *circuitMock) Name() string          { return m.name }
func (m *circuitMock) Variation() string     { return "" }
func (m *circuitMock) Country() string       { return "" }
func (m *circuitMock) CandidateCount() int   { return 0 }
func (m *circuitMock) LengthMetres() float64 { return m.lengthMetres }

func (m *circuitMock) LapProgress() float64 {
	return m.lapProgress
}

// UpdateCircuit implements the circuit.Manager interface.
func (m *circuitMock) UpdateCircuit(_ float64, _ int16, _ time.Duration, _ models.Coordinate, _ models.CoordinateType) bool {
	// Mock implementation - just return false for simplicity
	return false
}

func (m *circuitMock) LapProgressRemaining() float64 {
	if m.lapProgressRemaining > 0 {
		return m.lapProgressRemaining
	}

	return 1.0 - m.lapProgress
}

// --- Helper Functions ---

func createBasicConfig() *config.Config {
	configJSON := []byte(`{
		"app": {
			"language": "en",
			"accent": "us"
		},
		"pitRadio": {
			"enabled": true,
			"fuelMonitoring": {
				"enabled": true,
				"rangeSafetyMarginLaps": 0.3,
				"preWarnNotifyLaps": 2.0,
				"strategyNotifyLaps": 5.0
			}
		}
	}`)

	return config.NewFromJSON(configJSON, zerolog.Nop())
}

// --- Tests ---

type PitRadioTestSuite struct {
	suite.Suite

	app       *App
	pitRadio  pitRadioMock
	i18n      *i18n.I18n
	fuelRange fuelRangeMock
	circuit   circuitMock
}

func TestPitRadioTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(PitRadioTestSuite))
}

func (suite *PitRadioTestSuite) SetupTest() {
	gtClient, err := gttelemetry.New(gttelemetry.Options{})
	if err != nil {
		suite.FailNow("Failed to create GTClient", err)
	}

	suite.pitRadio = pitRadioMock{
		messages: []pitradio.Message{},
	}

	suite.i18n = createTestI18n()

	suite.fuelRange = fuelRangeMock{
		distanceMetres: 10000,
		distanceLaps:   0, // Will be calculated based on circuit length
	}

	suite.circuit = circuitMock{
		name:                 "Mock Circuit",
		lengthMetres:         1000,
		lapProgress:          0.0,
		lapProgressRemaining: 1.0,
	}

	// Use interface types with proper mock implementations
	suite.app = &App{
		config:   createBasicConfig(),
		gtClient: gtClient,
		state: gameState{
			current: raceState{},
			last:    raceState{},
		},
		pitRadio:      &suite.pitRadio,
		pitRadioState: &pitRadioState{},
		i18n:          suite.i18n,
		circuit:       &suite.circuit,   // Use circuit mock
		fuelRange:     &suite.fuelRange, // Use fuelRange mock
	}
}

func (suite *PitRadioTestSuite) TestAccentPopulatedInPitRadioMessage() {
	// Arrange
	configJSON := []byte(`{"app": {"language": "en", "accent": "ie"}}`)
	suite.app.config = config.NewFromJSON(configJSON, zerolog.Nop())

	// Act
	err := suite.app.pitRadio.Send(pitradio.Message{
		MessageType: pitradio.TextMessage,
		Text:        "Test message",
		Lang:        "en",
		Accent:      suite.app.config.GetAppAccent(),
	})

	// Assert
	suite.Require().NoError(err)
	suite.Require().Len(suite.pitRadio.messages, 1)

	message := suite.pitRadio.messages[0]
	suite.Equal("Test message", message.Text)
	suite.Equal("en", message.Lang)
	suite.Equal("ie", message.Accent)
}

func (suite *PitRadioTestSuite) TestDefaultAccentWhenNotConfigured() {
	// Arrange
	configJSON := []byte(`{"app": {"language": "en"}}`)
	suite.app.config = config.NewFromJSON(configJSON, zerolog.Nop())

	// Act
	accent := suite.app.config.GetAppAccent()

	// Assert
	suite.Equal("us", accent)
}

func (suite *PitRadioTestSuite) TestNoNotifyOnRaceStart() {
	// Arrange - plenty of fuel, early in race
	suite.setupDistanceBasedScenario(1, 10, 0.1, 75000.0, 5000.0) // 15 laps worth of fuel

	// Act
	suite.app.notifyFuelWarnings()

	// Assert
	suite.Empty(suite.pitRadio.messages)
}

func (suite *PitRadioTestSuite) TestNotifyOutOfFuel() {
	// Arrange - no fuel left scenario
	suite.setupDistanceBasedScenario(8, 10, 0.5, 0.0, 5000.0)
	suite.app.gtClient.Telemetry.RawTelemetry.FuelLevel = 0 // Empty tank

	// Act
	suite.app.notifyFuelWarnings()

	// Assert
	suite.Len(suite.pitRadio.messages, 1)
	message := suite.pitRadio.messages[0]
	suite.Equal(pitradio.TextMessage, message.MessageType)
	suite.NotEmpty(message.Text)
	suite.Equal("en", message.Lang)
	suite.True(suite.app.pitRadioState.fuelNotifyEmptyIssued)
}

func (suite *PitRadioTestSuite) TestNotifyFuelCritical() {
	// Arrange - critical fuel scenario: fuel range <= distance to pit box
	// For critical: fuelRangeMetres <= distanceToPitBox
	// distanceToPitBox = (1.0 - 0.5) * 5000 = 2500m
	// We need fuel range to be <= 2500m to trigger critical
	suite.setupDistanceBasedScenario(8, 10, 0.5, 2000.0, 5000.0) // 2000m < 2500m distance to pit

	// Act
	suite.app.notifyFuelWarnings()

	// Assert
	suite.Len(suite.pitRadio.messages, 1)
	message := suite.pitRadio.messages[0]
	suite.Equal(pitradio.TextMessage, message.MessageType)
	suite.NotEmpty(message.Text)
	suite.Equal("en", message.Lang)
	suite.Equal("us", message.Accent)
	suite.Equal(int16(8), suite.app.pitRadioState.lastNotifiedLapFuelCritical)
}

func (suite *PitRadioTestSuite) TestNotifyBoxThisLap() {
	// Arrange - box this lap scenario: enough fuel to complete current lap but not next
	_, boxThisLap, _ := suite.calculateWarningDistances(0.7, 5000.0, 0.3, 2.0)
	suite.setupDistanceBasedScenario(8, 10, 0.7, boxThisLap-100, 5000.0) // Just below box threshold
	suite.app.pitRadioState.lastNotifiedLapFuelWarning = 7               // Ensure warning not sent this lap

	// Act
	suite.app.notifyFuelWarnings()

	// Assert
	suite.Len(suite.pitRadio.messages, 1)
	message := suite.pitRadio.messages[0]
	suite.Equal(pitradio.TextMessage, message.MessageType)
	suite.NotEmpty(message.Text)
	suite.Equal("en", message.Lang)
}

func (suite *PitRadioTestSuite) TestNotifyFuelPreWarn() {
	// Arrange - Use distance-based calculation helpers for clarity
	lapProgress := 0.5
	circuitLength := 5000.0
	_, _, preWarnThreshold := suite.calculateWarningDistances(lapProgress, circuitLength, 0.3, 2.0)

	// Set fuel range just below the pre-warning threshold to trigger the warning
	fuelRangeMetres := preWarnThreshold - 100.0 // 100m below threshold

	suite.setupDistanceBasedScenario(5, 10, lapProgress, fuelRangeMetres, circuitLength)

	// Act
	suite.app.notifyFuelWarnings()

	// Assert
	suite.Len(suite.pitRadio.messages, 1)
	message := suite.pitRadio.messages[0]
	suite.Equal(pitradio.TextMessage, message.MessageType)
	suite.NotEmpty(message.Text)
	suite.Equal("en", message.Lang)
	suite.True(suite.app.pitRadioState.fuelNotifyPrewarnIssued)
}

func (suite *PitRadioTestSuite) TestNotifyFuelStrategyUpdateSentOnNotifyLaps() {
	// Arrange - strategy notification on divisible-by-5 laps, less fuel than remaining laps
	suite.setupDistanceBasedScenario(10, 25, 0.0, 60000.0, 5000.0) // 12 laps of fuel vs 15 remaining
	suite.app.pitRadioState.lastNotifiedLapFuelStrategy = 5

	// Act
	suite.app.notifyFuelWarnings()

	// Assert
	suite.Len(suite.pitRadio.messages, 1)
	message := suite.pitRadio.messages[0]
	suite.Equal(pitradio.TextMessage, message.MessageType)
	suite.NotEmpty(message.Text)
	suite.Equal("en", message.Lang)
	suite.Equal(int16(10), suite.app.pitRadioState.lastNotifiedLapFuelStrategy)
}

func (suite *PitRadioTestSuite) TestNotifyFuelStrategyUpdateNotSentOnNonNotifyLaps() {
	// Arrange - non-strategy lap (13 is not divisible by 5), less fuel than remaining laps
	suite.setupDistanceBasedScenario(13, 25, 0.0, 50000.0, 5000.0) // 10 laps of fuel vs 12 remaining
	suite.app.pitRadioState.lastNotifiedLapFuelStrategy = 12

	// Act
	suite.app.notifyFuelWarnings()

	// Assert
	suite.Empty(suite.pitRadio.messages)
	suite.Equal(int16(12), suite.app.pitRadioState.lastNotifiedLapFuelStrategy)
}

// TestFuelRangeDataIsPopulated tests that fuel range interface can be properly configured with mock data.
func (suite *PitRadioTestSuite) TestFuelRangeDataIsPopulated() {
	// Arrange - Set up test data for fuel range validation
	expectedRangeMetres := 15000.0
	expectedUsageRate := 1.67
	circuitLength := 5000.0

	// Act - Configure fuel range with test data
	suite.fuelRange.SetFuelRange(expectedRangeMetres, 0, expectedUsageRate)

	// Assert - Verify fuel range data is correct
	suite.NotNil(suite.app.fuelRange, "Fuel range should be available")
	suite.InEpsilon(expectedRangeMetres, suite.app.fuelRange.DistanceMetres(), 0.001, "Distance should match")
	suite.InEpsilon(expectedRangeMetres/circuitLength, suite.app.fuelRange.DistanceLaps(circuitLength), 0.001, "Laps should be calculated correctly")
	suite.InEpsilon(expectedUsageRate, suite.app.fuelRange.UsageRatePerKm(), 0.001, "Usage rate should match")
}

func (suite *PitRadioTestSuite) TestFuelWarningsWithRealData() {
	// Arrange - Set up critical fuel scenario with custom safety margin
	configJSON := []byte(`{"pitRadio": {"fuelRangeSafetyMarginLaps": 0.5}}`)
	suite.app.config = config.NewFromJSON(configJSON, zerolog.Nop())

	// Calculate critical threshold with custom safety margin
	critical, _, _ := suite.calculateWarningDistances(0.5, 5000.0, 0.5, 2.0)
	suite.setupDistanceBasedScenario(8, 10, 0.5, critical-100, 5000.0) // Just below critical

	// Act - Check for fuel warnings
	suite.app.notifyFuelWarnings()

	// Assert - Should generate critical fuel warning
	suite.NotEmpty(suite.pitRadio.messages, "Should generate critical fuel warning")

	if len(suite.pitRadio.messages) > 0 {
		message := suite.pitRadio.messages[0]
		suite.Equal(pitradio.TextMessage, message.MessageType)
		suite.NotEmpty(message.Text, "Message should have text")
	}
}

func (suite *PitRadioTestSuite) TestNotifyCircuitChange() {
	// Arrange - Set up circuit change scenario
	configJSON := []byte(`{
		"app": {"language": "en", "accent": "us"},
		"pitRadio": {
			"notifications": {
				"enableCircuitMatching": true
			}
		}
	}`)
	suite.app.config = config.NewFromJSON(configJSON, zerolog.Nop())
	suite.app.pitRadioState.circuitName = "Old Circuit"
	suite.circuit.name = "New Circuit"

	// Act - Notify of circuit change
	suite.app.notifyCircuitChange()

	// Assert - Should send notification and update state
	suite.Len(suite.pitRadio.messages, 1, "Should send circuit change notification")
	suite.Equal("New Circuit", suite.app.pitRadioState.circuitName, "Should update stored circuit name")

	if len(suite.pitRadio.messages) > 0 {
		suite.Contains(suite.pitRadio.messages[0].Text, "New Circuit", "Should mention new circuit")
	}
}

func (suite *PitRadioTestSuite) TestNotifyCircuitChangeNotSentForSameCircuit() {
	// Arrange - Same circuit name
	configJSON := []byte(`{
		"app": {"language": "en", "accent": "us"},
		"pitRadio": {
			"notifications": {
				"enableCircuitMatching": true
			}
		}
	}`)
	suite.app.config = config.NewFromJSON(configJSON, zerolog.Nop())
	suite.app.pitRadioState.circuitName = "Same Circuit"
	suite.circuit.name = "Same Circuit"

	// Act - Try to notify circuit change
	suite.app.notifyCircuitChange()

	// Assert - Should not send notification
	suite.Empty(suite.pitRadio.messages, "Should not send notification for same circuit")
}

func (suite *PitRadioTestSuite) TestPitRadioSendError() {
	// Arrange - Set up send failure scenario with very low fuel
	suite.pitRadio.failSend = true
	configJSON := []byte(`{"pitRadio": {"fuelRangeSafetyMarginLaps": 0.3}}`)
	suite.app.config = config.NewFromJSON(configJSON, zerolog.Nop())

	// Very low fuel range to trigger critical warning
	critical, _, _ := suite.calculateWarningDistances(0.5, 5000.0, 0.3, 2.0)
	suite.setupDistanceBasedScenario(8, 10, 0.5, critical-500, 5000.0) // Well below critical

	// Act - Try to send fuel warning (will fail)
	suite.app.notifyFuelWarnings()

	// Assert - Should handle error gracefully
	suite.Len(suite.pitRadio.messages, 1, "Should attempt to send message despite error")
}

func (suite *PitRadioTestSuite) TestDuplicateWarningsSuppressed() {
	// Arrange - Set up critical fuel scenario
	configJSON := []byte(`{"pitRadio": {"fuelRangeSafetyMarginLaps": 0.3}}`)
	suite.app.config = config.NewFromJSON(configJSON, zerolog.Nop())

	// Critical fuel level to trigger warning
	critical, _, _ := suite.calculateWarningDistances(0.5, 5000.0, 0.3, 2.0)
	suite.setupDistanceBasedScenario(8, 10, 0.5, critical-500, 5000.0) // Well below critical

	// Act - Send warning twice on same lap
	suite.app.notifyFuelWarnings()
	firstMessageCount := len(suite.pitRadio.messages)
	suite.app.notifyFuelWarnings()

	// Assert - Should not send duplicate
	suite.Len(suite.pitRadio.messages, firstMessageCount, "Should not send duplicate warnings on same lap")
}

func (suite *PitRadioTestSuite) TestPitRadioStateReset() {
	// Arrange - Set up existing state and basic scenario
	suite.app.pitRadioState.fuelNotifyEmptyIssued = true
	suite.app.pitRadioState.fuelNotifyPrewarnIssued = true
	suite.app.pitRadioState.lastNotifiedLapFuelCritical = 5
	suite.setupDistanceBasedScenario(10, 20, 0.0, 50000.0, 5000.0) // Basic scenario
	suite.app.gtClient.Telemetry.RawTelemetry.GridPosition = 3

	// Act - Reset pit radio state
	suite.app.resetPitRadioState()

	// Assert - All state should be reset
	suite.False(suite.app.pitRadioState.fuelNotifyEmptyIssued, "Empty fuel flag should be reset")
	suite.False(suite.app.pitRadioState.fuelNotifyPrewarnIssued, "Pre-warn flag should be reset")
	suite.Equal(int16(0), suite.app.pitRadioState.lastNotifiedLapFuelCritical, "Critical lap counter should be reset")
	suite.Equal(int16(10), suite.app.pitRadioState.lastNotifiedLapNumber, "Should update lap number")
	suite.Equal(int16(3), suite.app.pitRadioState.lastNotifiedGridPosition, "Should update grid position")
}

func (suite *PitRadioTestSuite) TestFuelWarningPriority() {
	// Arrange - Set up multiple warning conditions (empty fuel wins)
	configJSON := []byte(`{"pitRadio": {"fuelRangeSafetyMarginLaps": 0.3, "fuelPreWarnNotifyLaps": 2.0, "fuelStrategyNotifyLaps": 5.0}}`)
	suite.app.config = config.NewFromJSON(configJSON, zerolog.Nop())

	// Set up scenario that would trigger multiple warnings, but empty fuel should win
	critical, _, _ := suite.calculateWarningDistances(0.5, 5000.0, 0.3, 2.0)
	suite.setupDistanceBasedScenario(10, 20, 0.5, critical-500, 5000.0) // Critical fuel level
	suite.app.gtClient.Telemetry.RawTelemetry.FuelLevel = 0.0           // Empty - highest priority

	// Act - Check fuel warnings
	suite.app.notifyFuelWarnings()

	// Assert - Should prioritize empty fuel over other warnings
	suite.Len(suite.pitRadio.messages, 1, "Should send exactly one message")
	suite.True(suite.app.pitRadioState.fuelNotifyEmptyIssued, "Should mark empty fuel as notified")
}

// setupFuelRangeMetres configures the fuel range mock with distance in metres.
func (suite *PitRadioTestSuite) setupFuelRangeMetres(rangeMetres float64) {
	rangeLaps := rangeMetres / suite.circuit.lengthMetres
	suite.fuelRange.SetFuelRange(rangeMetres, rangeLaps, 2.0)
}

// setupDistanceBasedScenario configures a complete test scenario with distance-based parameters.
// This is the recommended helper for new tests as it provides clear distance-based setup.
func (suite *PitRadioTestSuite) setupDistanceBasedScenario(
	currentLap, totalLaps int16,
	lapProgress float64,
	fuelRangeMetres float64,
	circuitLengthMetres float64, //nolint:unparam // Keep for clarity in tests
) {
	// Set up basic race configuration and telemetry
	suite.app.config = createBasicConfig()
	suite.app.gtClient.Telemetry.RawTelemetry.CurrentLap = currentLap
	suite.app.gtClient.Telemetry.RawTelemetry.RaceLaps = totalLaps
	suite.app.gtClient.Telemetry.RawTelemetry.FuelLevel = 50.0
	suite.app.gtClient.Telemetry.RawTelemetry.FuelCapacity = 100.0

	// Mark the race as live so pit radio is active
	suite.app.state.current.isLive = true

	// Set up circuit with specified parameters
	suite.circuit.name = "Test Circuit"
	suite.circuit.lengthMetres = circuitLengthMetres
	suite.circuit.lapProgress = lapProgress
	suite.circuit.lapProgressRemaining = 1.0 - lapProgress

	// Set up fuel range
	suite.setupFuelRangeMetres(fuelRangeMetres)
	suite.clearPitRadioState()
}

// calculateWarningDistances helps calculate the distances at which different warnings should trigger.
// Returns distances in metres for: critical, boxThisLap, preWarn thresholds based on current position and config.
func (suite *PitRadioTestSuite) calculateWarningDistances(
	lapProgress float64,
	circuitLengthMetres float64, //nolint:unparam // Keep for clarity in tests
	safetyMarginLaps float64,
	preWarnNotifyLaps float64, //nolint:unparam // Keep for clarity in tests
) (critical, boxThisLap, preWarn float64) {
	distanceToPitBox := (1.0 - lapProgress) * circuitLengthMetres
	safetyMarginMetres := safetyMarginLaps * circuitLengthMetres

	// Critical: fuel range (with safety margin) <= distance to pit box
	critical = distanceToPitBox + safetyMarginMetres

	// Box this lap: fuel range (with safety margin) <= distance to pit box + one full lap
	boxThisLap = distanceToPitBox + circuitLengthMetres + safetyMarginMetres

	// Pre-warning: fuel range (with safety margin) <= distance to pit box + preWarnNotifyLaps
	preWarn = distanceToPitBox + (preWarnNotifyLaps * circuitLengthMetres) + safetyMarginMetres

	return critical, boxThisLap, preWarn
}

// clearPitRadioState resets all pit radio notification flags.
func (suite *PitRadioTestSuite) clearPitRadioState() {
	suite.app.pitRadioState = &pitRadioState{}
	suite.pitRadio.messages = []pitradio.Message{}
}

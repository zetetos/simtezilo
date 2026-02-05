package app //nolint:testpackage // white-box testing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	gttelemetry "github.com/zetetos/gt-telemetry/v2"
)

type TelemetryTestSuite struct {
	suite.Suite

	app *App
}

func TestTelemetryTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(TelemetryTestSuite))
}

func (suite *TelemetryTestSuite) SetupTest() {
	gtClient, err := gttelemetry.New(gttelemetry.Options{})
	if err != nil {
		suite.FailNow("Failed to create GTClient", err)
	}

	suite.app = &App{
		gtClient: gtClient,
		state: gameState{
			current: raceState{},
			last:    raceState{},
		},
	}
}

// TODO: figure out how to mock the GTClient for this test
// func (suite *TelemetryTestSuite) TestUpdateStateUpdatesCurrentState() {
// 	// Arrange
// 	suite.app.state.last.seq = 100
// 	suite.app.state.last.timeOfDay = 200 * time.Millisecond
// 	suite.app.state.last.vehicleID = 1
// 	suite.app.state.last.gear = 2
//
// 	suite.app.gtClient.Telemetry.SetSequenceID(101)
// 	suite.app.gtClient.Telemetry.SetTimeOfDay(300 * time.Millisecond)
// 	suite.app.gtClient.Telemetry.SetVehicleID(2)
// 	suite.app.gtClient.Telemetry.SetCurrentGear(3)
//
// 	// Act
// 	suite.app.updateState()
//
// 	// Assert
// 	suite.Equal(uint32(101), suite.app.state.current.seq, "Sequence ID should be updated")
// 	suite.Equal(300*time.Millisecond, suite.app.state.current.timeOfDay, "Time of day should be updated")
// 	suite.Equal(uint32(2), suite.app.state.current.vehicleID, "Vehicle ID should be updated")
// 	suite.Equal(uint32(3), suite.app.state.current.gear, "Gear should be updated")
// }

func (suite *TelemetryTestSuite) TestSequenceHasAdvancedReturnsFalseWhenSequenceIDHasNotChanged() {
	// Arrange
	suite.app.state.current.sequenceNumber = 101
	suite.app.state.current.sequenceDelta = 0

	// Act
	result := suite.app.sequenceHasAdvanced()

	// Assert
	suite.False(result, "Sequence should not have advanced")
}

func (suite *TelemetryTestSuite) TestSequenceHasAdvancedReturnsTrueWhenSequenceIDIncreases() {
	// Arrange
	suite.app.state.current.sequenceNumber = 102
	suite.app.state.current.sequenceDelta = 1

	// Act
	result := suite.app.sequenceHasAdvanced()

	// Assert
	suite.True(result, "Sequence should have advanced")
}

func (suite *TelemetryTestSuite) TestTimeOfDayHasResetReturnsTrueWhenTimeOfDayDeltaIsNegative() {
	// Arrange
	suite.app.state.current.sequenceNumber = 1
	suite.app.state.last.timeOfDay = 900 * time.Second
	suite.app.state.current.timeOfDay = 780 * time.Second

	// Act
	result := suite.app.timeOfDayHasReset()

	// Assert
	suite.True(result, "Should have detected a time of day reset")
}

func (suite *TelemetryTestSuite) TestTimeOfDayHasResetReturnsFalseWhenTimeOfDayDeltaIsPositive() {
	// Arrange
	suite.app.state.last.timeOfDay = 300 * time.Millisecond
	suite.app.state.current.timeOfDay = 316 * time.Millisecond

	// Act
	result := suite.app.timeOfDayHasReset()

	// Assert
	suite.False(result, "Should not have detected a time of day reset")
}

func (suite *TelemetryTestSuite) TestTimeOfDayHasResetReturnsFalseWhenTimeOfDayDeltaIsZero() {
	// Arrange
	suite.app.state.last.timeOfDay = 300 * time.Millisecond
	suite.app.state.current.timeOfDay = 316 * time.Millisecond

	// Act
	result := suite.app.timeOfDayHasReset()

	// Assert
	suite.False(result, "Should not have detected a time of day reset")
}

// TODO: figure out how to mock the GTClient for this test
// func (suite *TelemetryTestSuite) TestTelemetryIsActiveReturnsFalseWhenGamePaused() {
// 	// Arrange
// 	suite.app.gtClient.Telemetry.RawTelemetry.Flags = &gttelemetry.NewGranTurismoTelemetry_Flags{
// 		GamePaused: true,
// 	}
//
// 	// Act
// 	result := suite.app.telemetryIsActive()
//
// 	// Assert
// 	suite.False(result, "Telemetry should not be active when game is paused")
// }

// TODO: figure out how to mock the GTClient for this test
// func (suite *TelemetryTestSuite) TestTelemetryIsActiveReturnsTrueWhenLive() {
// 	// Arrange
// 	suite.app.gtClient.Telemetry.RawTelemetry.Flags = &telemetry.NewGranTurismoTelemetry_Flags{
// 		GamePaused: false,
// 		Live:       true,
// 	}
//
//  // Act
// 	result := suite.app.telemetryIsActive()
//
// 	// Assert
// 	suite.True(result, "Telemetry should be active when live")
// }

func (suite *TelemetryTestSuite) TestTelemetryPacketsDroppedReturnsZeroWhenNoPacketsDropped() {
	// Arrange
	suite.app.state.current.sequenceDelta = 1

	// Act
	dropped := suite.app.telemetryPacketsDropped()

	// Assert
	suite.Equal(uint32(0), dropped, "No packets should be dropped")
}

func (suite *TelemetryTestSuite) TestTelemetryPacketsDroppedReturnsCorrectCountWhenPacketsDropped() {
	// Arrange
	suite.app.state.current.sequenceDelta = 5

	// Act
	dropped := suite.app.telemetryPacketsDropped()

	// Assert
	suite.Equal(uint32(4), dropped, "Should have dropped 4 packets")
}

func (suite *TelemetryTestSuite) TestVehicleHasChangedReturnsTrueWhenVehicleIDChanges() {
	// Arrange
	suite.app.vehicle.ID = 1234
	suite.app.gtClient.Telemetry.RawTelemetry.VehicleId = 1235

	// Act
	result := suite.app.vehicleHasChanged()

	// Assert
	suite.True(result, "Vehicle should have changed")
}

func (suite *TelemetryTestSuite) TestVehicleHasChangedReturnsFalseWhenVehicleIDDoesNotChange() {
	// Arrange
	suite.app.vehicle.ID = 1230
	suite.app.gtClient.Telemetry.RawTelemetry.VehicleId = 1230

	// Act
	result := suite.app.vehicleHasChanged()

	// Assert
	suite.False(result, "Vehicle should not have changed")
}

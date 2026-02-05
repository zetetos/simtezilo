package fuelrange //nolint:testpackage // white-box testing

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type FuelRangeTestSuite struct {
	suite.Suite

	fuelRange Estimator
}

func TestFuelRangeTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(FuelRangeTestSuite))
}

func (suite *FuelRangeTestSuite) SetupTest() {
	suite.fuelRange = New(zerolog.Nop())
	suite.fuelRange.SetLive(false) // Use replay mode for predictable testing
}

func (suite *FuelRangeTestSuite) TestBasicFuelRangeCalculation() {
	// Arrange - Initialize with full fuel tank
	suite.fuelRange.Update(1000.0, 100.0)
	suite.InEpsilon(rangeDistanceUnknown, suite.fuelRange.DistanceMetres(), 0.001)

	// Act - Simulate consumption: 5km distance, 10% fuel used
	suite.simulateConsumption(1000.0, 5000.0, 100.0, 10.0)

	// Assert - Should calculate range: 90% fuel remaining should give ~45km range
	calculatedRange := suite.fuelRange.DistanceMetres()
	suite.NotEqual(rangeDistanceUnknown, calculatedRange)
	suite.InDelta(45000.0, calculatedRange, 4500.0) // 10% tolerance
}

func (suite *FuelRangeTestSuite) TestFuelRangeInLaps() {
	// Arrange - Setup 50km range with 50% fuel
	suite.setFuelRange(50000.0, 50.0)

	// Act - Calculate lap range for 5km circuit
	lapsRange := suite.fuelRange.DistanceLaps(5000.0)

	// Assert - Should give 10 laps
	suite.InDelta(10.0, lapsRange, 0.5)
}

func (suite *FuelRangeTestSuite) TestFuelRangeWithRefueling() {
	// Arrange - Setup known consumption pattern (ending at 20% fuel)
	suite.setFuelRange(20000.0, 20.0)

	// Verify we have a calculated range before refueling
	initialRange := suite.fuelRange.DistanceMetres()
	suite.NotEqual(rangeDistanceUnknown, initialRange)

	// Act - Simulate refueling (fuel level increases from 20% to 95%)
	// Note: Refueling detection requires fuel increase > 1% (consumed < -1)
	suite.fuelRange.Update(10000.0, 95.0)

	// Assert - Range should reset to unknown after refueling
	suite.InEpsilon(rangeDistanceUnknown, suite.fuelRange.DistanceMetres(), 0.001)
}

func (suite *FuelRangeTestSuite) TestNormalFuelConsumptionPreservesRange() {
	// Arrange - Setup known consumption pattern (ending at 20% fuel)
	suite.setFuelRange(20000.0, 20.0)

	// Verify we have a calculated range
	initialRange := suite.fuelRange.DistanceMetres()
	suite.NotEqual(rangeDistanceUnknown, initialRange)

	// Act - Simulate normal fuel consumption with some distance traveled
	// This simulates continuing to drive and consume fuel normally
	suite.simulateConsumption(21000.0, 1000.0, 20.0, 1.0) // Drive 1km, use 1% fuel

	// Assert - Range should still be calculated (not reset to unknown)
	newRange := suite.fuelRange.DistanceMetres()
	suite.NotEqual(rangeDistanceUnknown, newRange)
	suite.Greater(newRange, 0.0)
}

func (suite *FuelRangeTestSuite) TestInsufficientSamples() {
	// Arrange & Act - Provide insufficient samples (10 < 18 minimum for replay mode)
	for i := range 10 {
		suite.fuelRange.Update(1000.0+float64(i)*10.0, 100.0-float32(i)*0.1)
	}

	// Assert - Should return unknown range
	suite.InEpsilon(rangeDistanceUnknown, suite.fuelRange.DistanceMetres(), 0.001)
}

func (suite *FuelRangeTestSuite) TestLowFuelRangeCalculation() {
	// Arrange - Setup low fuel scenario: 10km range with 5% fuel
	suite.setFuelRange(10000.0, 5.0)

	// Act - Calculate laps for 4km circuit
	rangeLaps := suite.fuelRange.DistanceLaps(4000.0)

	// Assert - Should give ~2.5 laps (critical situation)
	suite.Greater(rangeLaps, 0.0)
	suite.Less(rangeLaps, 3.0)
}

func (suite *FuelRangeTestSuite) TestZeroAndNegativeCircuitLength() {
	// Arrange - Setup known consumption
	suite.setFuelRange(50000.0, 50.0)

	// Act & Assert - Zero circuit length should return unknown laps
	suite.InEpsilon(rangeLapsUnknown, suite.fuelRange.DistanceLaps(0.0), 0.001)

	// Act & Assert - Negative circuit length should return unknown laps
	suite.InEpsilon(rangeLapsUnknown, suite.fuelRange.DistanceLaps(-1000.0), 0.001)
}

func (suite *FuelRangeTestSuite) TestOdometerRollback() {
	// Arrange - Setup consumption pattern
	suite.setFuelRange(30000.0, 30.0)
	initialRange := suite.fuelRange.DistanceMetres()
	suite.NotEqual(rangeDistanceUnknown, initialRange)

	// Act - Simulate odometer rollback (odometer goes backwards)
	suite.fuelRange.Update(500.0, 25.0) // Much lower odometer reading

	// Assert - Should reset the fuel range due to odometer rollback
	suite.InEpsilon(rangeDistanceUnknown, suite.fuelRange.DistanceMetres(), 0.001)
}

func (suite *FuelRangeTestSuite) TestSmallFuelIncreasePreservesRange() {
	// Arrange - Setup known consumption pattern (ending at 20% fuel)
	suite.setFuelRange(20000.0, 20.0)
	initialRange := suite.fuelRange.DistanceMetres()
	suite.NotEqual(rangeDistanceUnknown, initialRange)

	// Act - Simulate small fuel increase (≤0.04% - should not trigger refuel detection)
	suite.fuelRange.Update(22000.0, 20.04) // Only 0.04% increase

	// Assert - Range should NOT reset (small increase is not considered refueling)
	newRange := suite.fuelRange.DistanceMetres()
	suite.NotEqual(rangeDistanceUnknown, newRange)
}

func (suite *FuelRangeTestSuite) TestZeroFuelLevel() {
	// Arrange - Setup known consumption
	suite.setFuelRange(20000.0, 20.0)

	// Act - Update with zero fuel
	suite.fuelRange.Update(25000.0, 0.0)

	// Assert - Should return zero range
	calculatedRange := suite.fuelRange.DistanceMetres()
	suite.InDelta(0.0, calculatedRange, 0.001)
}

func (suite *FuelRangeTestSuite) TestNegativeFuelLevel() {
	// Arrange - Setup known consumption
	suite.setFuelRange(20000.0, 20.0)

	// Act - Update with negative fuel
	suite.fuelRange.Update(25000.0, -5.0)

	// Assert - Should return negative range (system allows negative fuel calculations)
	calculatedRange := suite.fuelRange.DistanceMetres()
	suite.Less(calculatedRange, 0.0)
}

func (suite *FuelRangeTestSuite) TestFuelLevelAbove100PercentTriggersRefuel() {
	// Arrange - Setup consumption pattern
	suite.setFuelRange(20000.0, 20.0)

	// Act - Update with fuel level > 100% (should trigger refuel detection)
	suite.fuelRange.Update(25000.0, 150.0)

	// Assert - Should reset range due to excessive fuel increase
	suite.InEpsilon(rangeDistanceUnknown, suite.fuelRange.DistanceMetres(), 0.001)
}

func (suite *FuelRangeTestSuite) TestResetFunctionality() {
	// Arrange - Setup known consumption
	suite.setFuelRange(20000.0, 20.0)
	suite.NotEqual(rangeDistanceUnknown, suite.fuelRange.DistanceMetres())

	// Act - Reset the fuel range
	suite.fuelRange.Reset()

	// Assert - Should return unknown range after reset
	suite.InEpsilon(rangeDistanceUnknown, suite.fuelRange.DistanceMetres(), 0.001)
}

func (suite *FuelRangeTestSuite) TestResetEstimateFunctionality() {
	// Arrange - Setup known consumption
	suite.setFuelRange(20000.0, 20.0)
	suite.NotEqual(rangeDistanceUnknown, suite.fuelRange.DistanceMetres())

	// Act - Reset only the estimate (not the entire state)
	suite.fuelRange.ResetEstimate()

	// Assert - Should return unknown range after estimate reset
	suite.InEpsilon(rangeDistanceUnknown, suite.fuelRange.DistanceMetres(), 0.001)
}

func (suite *FuelRangeTestSuite) TestUsageRatePerKm() {
	// Arrange - Setup known consumption: 20% fuel for 20km = 1% per km
	suite.setFuelRange(20000.0, 20.0)

	// Act - Get usage rate per km
	usageRate := suite.fuelRange.UsageRatePerKm()

	// Assert - Should return 1% per km (1.0)
	suite.InDelta(1.0, usageRate, 0.1)
}

func (suite *FuelRangeTestSuite) TestUsageRatePerKmWithNoData() {
	// Arrange - No consumption data (fresh estimator)

	// Act - Get usage rate per km
	usageRate := suite.fuelRange.UsageRatePerKm()

	// Assert - Should return zero when no consumption data is available
	suite.InDelta(0.0, usageRate, 0.001)
}

// simulateConsumption simulates fuel consumption over distance.
func (suite *FuelRangeTestSuite) simulateConsumption(initialOdometer, distance float64, initialFuel, consumedFuel float32) {
	samples := 80 // enough samples for replay mode requirements (72 minimum)

	for i := range samples {
		progress := float64(i+1) / float64(samples)
		odometer := initialOdometer + distance*progress

		fuel := initialFuel - consumedFuel*float32(progress)

		suite.fuelRange.Update(odometer, fuel)
	}
}

// setFuelRange sets up a known consumption rate for testing.
func (suite *FuelRangeTestSuite) setFuelRange(expectedRangeMetres, currentFuelPercent float64) {
	initialOdometer := 1000.0 // start at 1km
	distance := 20000.0       // travel 20km to establish rate

	consumptionRate := currentFuelPercent / expectedRangeMetres
	consumedFuel := consumptionRate * distance
	initialFuel := float32(currentFuelPercent + consumedFuel)

	// Initialize fuel range with starting values
	suite.fuelRange.Update(initialOdometer, initialFuel)

	// Simulate consumption to establish rate
	suite.simulateConsumption(initialOdometer, distance, initialFuel, float32(consumedFuel))
}

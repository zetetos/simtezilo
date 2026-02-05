package kinematics_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/suite"
	gttelemetry "github.com/zetetos/gt-telemetry/v2"
	"github.com/zetetos/simtezilo/app/kinematics"
	"github.com/zetetos/simtezilo/app/vehicle"
)

type KinematicsTestSuite struct {
	suite.Suite

	state         kinematics.State
	gtClient      *gttelemetry.Client
	dims          vehicle.Dimensions
	windowSeconds float64
}

func TestKinematicsTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(KinematicsTestSuite))
}

func (s *KinematicsTestSuite) SetupTest() {
	s.state = kinematics.NewKinematicsState()

	var err error

	s.gtClient, err = gttelemetry.New(gttelemetry.Options{})
	s.Require().NoError(err, "Failed to create GTClient")

	s.gtClient.Telemetry.SetFormatStandard()

	s.dims = vehicle.Dimensions{
		WheelbaseMetres:    2.5,
		TrackWidthMetres:   1.6,
		LongitudinalRadius: 1.25,
		TransverseRadius:   0.8,
	}

	s.windowSeconds = 0.016
}

func (s *KinematicsTestSuite) TestNewKinematicsStateInitializesFields() {
	state := kinematics.NewKinematicsState()

	s.Equal(-100, state.Current.TransmissionGear)
	s.Equal(-100, state.Last.TransmissionGear)
	s.Equal("A", state.Current.Format)
	s.InDelta(0, state.Current.GroundSpeed, 1e-9)
	s.InDelta(0, state.Current.SurgeCalculated, 1e-9)
}

func (s *KinematicsTestSuite) TestUpdateCopiesCurrentToLast() {
	s.state.Current.SequenceID = 100
	s.state.Current.GroundSpeed = 50.0
	s.state.Current.TransmissionGear = 3

	s.state.Update(s.windowSeconds, s.dims, s.gtClient)

	s.Equal(uint32(100), s.state.Last.SequenceID)
	s.InDelta(50.0, s.state.Last.GroundSpeed, 1e-9)
	s.Equal(3, s.state.Last.TransmissionGear)
}

func (s *KinematicsTestSuite) TestUpdateSetsSequenceIDFromTelemetry() {
	s.gtClient.Telemetry.RawTelemetry.SequenceId = 42

	s.state.Update(s.windowSeconds, s.dims, s.gtClient)

	s.Equal(uint32(42), s.state.Current.SequenceID)
}

func (s *KinematicsTestSuite) TestUpdateSetsGroundSpeedFromTelemetry() {
	s.gtClient.Telemetry.RawTelemetry.GroundSpeed = 33.5

	s.state.Update(s.windowSeconds, s.dims, s.gtClient)

	s.InDelta(33.5, s.state.Current.GroundSpeed, 1e-6)
}

func (s *KinematicsTestSuite) TestUpdateWithNilTransmissionGearReturnsNeutral() {
	s.state.Update(s.windowSeconds, s.dims, s.gtClient)

	s.Equal(15, s.state.Current.TransmissionGear)
}

func (s *KinematicsTestSuite) TestUpdateWithNilTelemetryVectorsResultsInZeroMagnitudes() {
	s.state.Update(s.windowSeconds, s.dims, s.gtClient)

	s.InDelta(0, s.state.Current.SixDOFTranslation.AccelMag, 1e-9)
	s.InDelta(0, s.state.Current.SixDOFTranslationCalc.AccelMag, 1e-9)
	s.InDelta(0, s.state.Current.SixDOFRotation.AccelMag, 1e-9)
	s.InDelta(0, s.state.Current.SixDOFRotationCalc.AccelMag, 1e-9)
}

func (s *KinematicsTestSuite) TestUpdateMultipleTimesAccumulatesDerivatives() {
	windowSeconds := 0.1
	s.state.Current.SixDOFTranslation.AccelMag = 5.0

	s.state.Update(windowSeconds, s.dims, s.gtClient)

	expectedJerk := (0.0 - 5.0) / windowSeconds
	s.InDelta(expectedJerk, s.state.Current.SixDOFTranslation.Jerk, 1e-6)
}

func (s *KinematicsTestSuite) TestUpdateCalculatesSnapFromJerkChange() {
	windowSeconds := 0.1
	s.state.Current.SixDOFTranslation.AccelMag = 10.0

	s.state.Update(windowSeconds, s.dims, s.gtClient)
	firstJerk := s.state.Current.SixDOFTranslation.Jerk

	s.state.Update(windowSeconds, s.dims, s.gtClient)

	expectedSnap := (0.0 - firstJerk) / windowSeconds
	s.InDelta(expectedSnap, s.state.Current.SixDOFTranslation.Snap, 1e-6)
}

func (s *KinematicsTestSuite) TestUpdateCalculatesCrackleForTranslationCalc() {
	windowSeconds := 0.1
	s.state.Current.SixDOFTranslationCalc.Snap = 20.0

	s.state.Update(windowSeconds, s.dims, s.gtClient)
	s.state.Update(windowSeconds, s.dims, s.gtClient)

	s.False(math.IsNaN(s.state.Current.SixDOFTranslationCalc.Crackle))
}

func (s *KinematicsTestSuite) TestGetSurgeGforceReturnsCalculatedSurgeForFormatA() {
	s.state.Current.Format = "A"
	s.state.Current.SurgeCalculated = 9.81

	gForce := s.state.GetSurgeGforce()

	s.InDelta(1.0, gForce, 0.01)
}

func (s *KinematicsTestSuite) TestGetSurgeGforceReturnsTelemetrySurgeForFormatB() {
	s.state.Current.Format = "B"
	s.state.Current.SixDOFTranslation.Acceleration.Surge = 19.62
	s.state.Current.SurgeCalculated = 9.81

	gForce := s.state.GetSurgeGforce()

	s.InDelta(2.0, gForce, 0.01)
}

func (s *KinematicsTestSuite) TestGetSurgeGforceReturnsTelemetrySurgeForFormatTilde() {
	s.state.Current.Format = "~"
	s.state.Current.SixDOFTranslation.Acceleration.Surge = -9.81
	s.state.Current.SurgeCalculated = 19.62

	gForce := s.state.GetSurgeGforce()

	s.InDelta(1.0, gForce, 0.01)
}

func (s *KinematicsTestSuite) TestGetSurgeGforceReturnsAbsoluteValue() {
	s.state.Current.Format = "A"
	s.state.Current.SurgeCalculated = -9.81

	gForce := s.state.GetSurgeGforce()

	s.InDelta(1.0, gForce, 0.01)
}

func (s *KinematicsTestSuite) TestGetSurgeGforceWithZeroSurge() {
	s.state.Current.Format = "A"
	s.state.Current.SurgeCalculated = 0

	gForce := s.state.GetSurgeGforce()

	s.InDelta(0, gForce, 1e-9)
}

func (s *KinematicsTestSuite) TestUpdatePreservesFormatBetweenCalls() {
	s.state.Update(s.windowSeconds, s.dims, s.gtClient)
	firstFormat := s.state.Current.Format

	s.state.Update(s.windowSeconds, s.dims, s.gtClient)

	s.Equal(firstFormat, s.state.Current.Format)
}

func (s *KinematicsTestSuite) TestUpdateWithZeroWindowSecondsProducesInfOrNaNJerk() {
	s.state.Current.SixDOFTranslation.AccelMag = 10.0
	s.state.Update(s.windowSeconds, s.dims, s.gtClient)

	s.state.Update(0.0, s.dims, s.gtClient)

	jerk := s.state.Current.SixDOFTranslation.Jerk
	isExpectedValue := math.IsInf(jerk, 0) || math.IsNaN(jerk) || jerk == 0
	s.True(isExpectedValue, "Jerk should be Inf, NaN, or 0, got %f", jerk)
}

func (s *KinematicsTestSuite) TestUpdateWithVerySmallWindowSeconds() {
	windowSeconds := 0.0001
	s.state.Current.SixDOFTranslation.AccelMag = 5.0

	s.state.Update(windowSeconds, s.dims, s.gtClient)
	s.state.Update(windowSeconds, s.dims, s.gtClient)

	s.False(math.IsNaN(s.state.Current.SixDOFTranslation.Jerk))
}

func (s *KinematicsTestSuite) TestStateFieldsAreIndependent() {
	s.state.Current.GroundSpeed = 100.0
	s.state.Current.TransmissionGear = 5

	s.InDelta(0, s.state.Last.GroundSpeed, 1e-9)
	s.Equal(-100, s.state.Last.TransmissionGear)
}

func (s *KinematicsTestSuite) TestRotationalCalcVelocityScalesByDimensions() {
	testCases := []struct {
		name               string
		longitudinalRadius float32
		transverseRadius   float32
	}{
		{"standard dimensions", 1.25, 0.8},
		{"zero dimensions", 0, 0},
		{"large dimensions", 5.0, 3.0},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			state := kinematics.NewKinematicsState()
			dims := vehicle.Dimensions{
				LongitudinalRadius: tc.longitudinalRadius,
				TransverseRadius:   tc.transverseRadius,
			}

			state.Update(s.windowSeconds, dims, s.gtClient)

			s.False(math.IsNaN(state.Current.SixDOFRotationCalc.AccelMag))
		})
	}
}

func (s *KinematicsTestSuite) TestRotationalCalcVelocityUsesAngularVelocityFromTelemetry() {
	s.gtClient.Telemetry.SetAngularVelocityVector(1.0, 2.0, 3.0)

	dims := vehicle.Dimensions{
		LongitudinalRadius: 2.0,
		TransverseRadius:   1.5,
	}

	s.state.Update(s.windowSeconds, dims, s.gtClient)

	s.InDelta(2.0, float64(s.state.Current.SixDOFRotationCalc.Velocity.X), 1e-6)
	s.InDelta(4.0, float64(s.state.Current.SixDOFRotationCalc.Velocity.Y), 1e-6)
	s.InDelta(4.5, float64(s.state.Current.SixDOFRotationCalc.Velocity.Z), 1e-6)
}

func (s *KinematicsTestSuite) TestUpdateSetsTranslationAccelerationFromTelemetry() {
	s.gtClient.Telemetry.SetTranslationalEnvelope(1.5, 2.5, 3.5)

	s.state.Update(s.windowSeconds, s.dims, s.gtClient)

	s.InDelta(1.5, float64(s.state.Current.SixDOFTranslation.Acceleration.Sway), 1e-6)
	s.InDelta(2.5, float64(s.state.Current.SixDOFTranslation.Acceleration.Heave), 1e-6)
	s.InDelta(3.5, float64(s.state.Current.SixDOFTranslation.Acceleration.Surge), 1e-6)
}

func (s *KinematicsTestSuite) TestUpdateSetsTranslationCalcVelocityFromTelemetry() {
	s.gtClient.Telemetry.SetVelocityVector(10.0, 20.0, 30.0)

	s.state.Update(s.windowSeconds, s.dims, s.gtClient)

	s.InDelta(10.0, float64(s.state.Current.SixDOFTranslationCalc.Velocity.X), 1e-6)
	s.InDelta(20.0, float64(s.state.Current.SixDOFTranslationCalc.Velocity.Y), 1e-6)
	s.InDelta(30.0, float64(s.state.Current.SixDOFTranslationCalc.Velocity.Z), 1e-6)
}

func (s *KinematicsTestSuite) TestUpdateSetsRotationVelocityFromTelemetry() {
	s.gtClient.Telemetry.SetAngularVelocityVector(0.5, 1.0, 1.5)

	s.state.Update(s.windowSeconds, s.dims, s.gtClient)

	s.InDelta(0.5, float64(s.state.Current.SixDOFRotation.Velocity.X), 1e-6)
	s.InDelta(1.0, float64(s.state.Current.SixDOFRotation.Velocity.Y), 1e-6)
	s.InDelta(1.5, float64(s.state.Current.SixDOFRotation.Velocity.Z), 1e-6)
}

func (s *KinematicsTestSuite) TestUpdateSetsTransmissionGearFromTelemetry() {
	s.gtClient.Telemetry.SetTransmissionGear(4, 5)

	s.state.Update(s.windowSeconds, s.dims, s.gtClient)

	s.Equal(4, s.state.Current.TransmissionGear)
}

func (s *KinematicsTestSuite) TestUpdateCalculatesTranslationCalcAccelerationFromVelocityDelta() {
	windowSeconds := 0.1

	s.gtClient.Telemetry.SetVelocityVector(10.0, 0.0, 0.0)
	s.state.Update(windowSeconds, s.dims, s.gtClient)

	s.gtClient.Telemetry.SetVelocityVector(20.0, 0.0, 0.0)
	s.state.Update(windowSeconds, s.dims, s.gtClient)

	expectedAccelX := (20.0 - 10.0) / windowSeconds
	s.InDelta(expectedAccelX, float64(s.state.Current.SixDOFTranslationCalc.Acceleration.X), 1e-4)
}

func (s *KinematicsTestSuite) TestUpdateCalculatesRotationAccelerationFromVelocityDelta() {
	windowSeconds := 0.1

	s.gtClient.Telemetry.SetAngularVelocityVector(1.0, 0.0, 0.0)
	s.state.Update(windowSeconds, s.dims, s.gtClient)

	s.gtClient.Telemetry.SetAngularVelocityVector(2.0, 0.0, 0.0)
	s.state.Update(windowSeconds, s.dims, s.gtClient)

	s.InDelta(1.0, float64(s.state.Current.SixDOFRotation.Acceleration.X), 1e-6)
}

func (s *KinematicsTestSuite) TestUpdateCalculatesSurgeFromRotationalCalcAcceleration() {
	windowSeconds := 0.1

	s.gtClient.Telemetry.SetAngularVelocityVector(1.0, 0.0, 0.0)
	s.state.Update(windowSeconds, s.dims, s.gtClient)

	s.gtClient.Telemetry.SetAngularVelocityVector(3.0, 0.0, 0.0)
	s.state.Update(windowSeconds, s.dims, s.gtClient)

	// RotationCalc.Velocity.X = AngularVelocity.X * LongitudinalRadius (1.25)
	// Delta: (3.0 * 1.25) - (1.0 * 1.25) = 2.5
	// Acceleration.X = 2.5 / 0.1 = 25.0
	s.InDelta(25.0, s.state.Current.SurgeCalculated, 1e-4)
}

func (s *KinematicsTestSuite) TestGetSurgeGforceUsesTranslationSurgeForFormatB() {
	s.gtClient.Telemetry.SetFormatAddendum1()
	s.gtClient.Telemetry.SetTranslationalEnvelope(0, 0, 9.81)

	s.state.Update(s.windowSeconds, s.dims, s.gtClient)
	gForce := s.state.GetSurgeGforce()

	s.InDelta(1.0, gForce, 0.01)
}

func (s *KinematicsTestSuite) TestGetSurgeGforceUsesTranslationSurgeForFormatTilde() {
	s.gtClient.Telemetry.SetFormatAddendum2()
	s.gtClient.Telemetry.SetTranslationalEnvelope(0, 0, 19.62)

	s.state.Update(s.windowSeconds, s.dims, s.gtClient)
	gForce := s.state.GetSurgeGforce()

	s.InDelta(2.0, gForce, 0.01)
}

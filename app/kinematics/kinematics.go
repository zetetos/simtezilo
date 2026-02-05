// Package kinematics provides structures and methods for calculating and tracking vehicle kinematics.
package kinematics

import (
	"time"

	gttelemetry "github.com/zetetos/gt-telemetry/v2"
	"github.com/zetetos/gt-telemetry/v2/pkg/models"
	"github.com/zetetos/simtezilo/app/kinematics/translationalenvelope"
	"github.com/zetetos/simtezilo/app/kinematics/vector"
	"github.com/zetetos/simtezilo/app/signal"
	"github.com/zetetos/simtezilo/app/vehicle"
)

// PositionalDerivatives holds values for the 2nd to 5th derivatives of position.
type PositionalDerivatives struct {
	Acceleration float64
	Jerk         float64
	Snap         float64
	Crackle      float64
}

// CalculatedTranslationalDerivatives holds 6DOF translational derivatives calculated from the telemetry data.
type CalculatedTranslationalDerivatives struct {
	PositionalDerivatives

	Velocity     models.Vector
	Acceleration models.Vector
	AccelMag     float64
}

// CalculatedRotationalDerivatives holds 6DOF rotational derivatives provided by the telemetry data.
type CalculatedRotationalDerivatives struct {
	PositionalDerivatives

	Velocity     models.Vector
	Acceleration models.Vector
	AccelMag     float64
}

// TranslationalDerivatives holds 6DOF translational derivatives calculated from the telemetry data.
type TranslationalDerivatives struct {
	PositionalDerivatives

	Acceleration models.TranslationalEnvelope
	AccelMag     float64
}

// RotationalDerivatives holds 6DOF rotational derivatives provided by the telemetry data.
type RotationalDerivatives struct {
	PositionalDerivatives

	Velocity     models.Vector
	Acceleration models.Vector
	AccelMag     float64
}

// Kinematics holds the kinematic state of the vehicle.
type Kinematics struct {
	SequenceID  uint32 // TODO: probably should rely on a.state.current instead of tracking separately here
	ComputeTime time.Duration
	Format      string

	SixDOFTranslation     TranslationalDerivatives
	SixDOFTranslationCalc CalculatedTranslationalDerivatives
	SixDOFRotation        RotationalDerivatives
	SixDOFRotationCalc    CalculatedRotationalDerivatives

	TransmissionGear int // TODO: probably should rely on a.state.current instead of tracking separately here

	GroundSpeed     float64
	SurgeCalculated float64

	SynthOutputAmplitude float64
	SynthOutputFrequency int
}

// State tracks the current and previous kinematic states of the vehicle.
type State struct {
	Last    Kinematics
	Current Kinematics
}

// NewKinematicsState creates and initializes a new KinematicsTracker.
func NewKinematicsState() State {
	return State{
		Last:    newKinematics(),
		Current: newKinematics(),
	}
}

// newKinematics initializes a new Kinematics struct with default values.
func newKinematics() Kinematics {
	return Kinematics{
		SixDOFTranslationCalc: CalculatedTranslationalDerivatives{},
		SixDOFTranslation:     TranslationalDerivatives{},
		SixDOFRotationCalc:    CalculatedRotationalDerivatives{},
		ComputeTime:           0,
		TransmissionGear:      -100,
		GroundSpeed:           0,
		SurgeCalculated:       0,
		SynthOutputAmplitude:  0,
		SynthOutputFrequency:  0,
		Format:                "A",
	}
}

// Update updates the kinematic state based on the provided telemetry data and time window.
// TODO: ideally this should not be given the gt client.
func (k *State) Update(windowSeconds float64, vehicleDimensions vehicle.Dimensions, gtclient *gttelemetry.Client) {
	k.Last = k.Current

	k.Current.Format = getTelemetryFormat(gtclient)

	k.Current.SequenceID = gtclient.Telemetry.SequenceID()

	accelFactor := float32(1.0 / windowSeconds)

	// 6DOF translational envelope - provided by telemetry
	k.Current.SixDOFTranslation.Acceleration = gtclient.Telemetry.TranslationEnvelope()
	k.Current.SixDOFTranslation.AccelMag = translationalenvelope.Magnitude(k.Current.SixDOFTranslation.Acceleration)
	k.Current.SixDOFTranslation.Jerk = (k.Current.SixDOFTranslation.AccelMag - k.Last.SixDOFTranslation.AccelMag) / windowSeconds
	k.Current.SixDOFTranslation.Snap = (k.Current.SixDOFTranslation.Jerk - k.Last.SixDOFTranslation.Jerk) / windowSeconds

	// 6DOF translational envelope - calculated from velocity vector
	k.Current.SixDOFTranslationCalc.Velocity = gtclient.Telemetry.VelocityVector()
	translationCalcVelocityDelta := vector.Delta(k.Current.SixDOFTranslationCalc.Velocity, k.Last.SixDOFTranslationCalc.Velocity)
	k.Current.SixDOFTranslationCalc.Acceleration = vector.Scale(translationCalcVelocityDelta, accelFactor, accelFactor, accelFactor)
	k.Current.SixDOFTranslationCalc.AccelMag = vector.Magnitude(k.Current.SixDOFTranslationCalc.Acceleration)
	k.Current.SixDOFTranslationCalc.Jerk = (k.Current.SixDOFTranslationCalc.AccelMag - k.Last.SixDOFTranslationCalc.AccelMag) / windowSeconds
	k.Current.SixDOFTranslationCalc.Snap = (k.Current.SixDOFTranslationCalc.Jerk - k.Last.SixDOFTranslationCalc.Jerk) / windowSeconds
	k.Current.SixDOFTranslationCalc.Crackle = (k.Current.SixDOFTranslationCalc.Snap - k.Last.SixDOFTranslationCalc.Snap) / windowSeconds

	// 6DOF rotational envelope - provided by telemetry
	k.Current.SixDOFRotation.Velocity = gtclient.Telemetry.AngularVelocityVector()
	k.Current.SixDOFRotation.Acceleration = vector.Delta(k.Current.SixDOFRotation.Velocity, k.Last.SixDOFRotation.Velocity)
	k.Current.SixDOFRotation.AccelMag = vector.Magnitude(k.Current.SixDOFRotation.Acceleration)

	// 6DOF rotational envelope - calculated angular velocity vector
	// Convert from radians to metres at the wheels using vehicle dimensions
	k.Current.SixDOFRotationCalc.Velocity = vector.Scale(
		k.Current.SixDOFRotation.Velocity,
		vehicleDimensions.LongitudinalRadius,
		vehicleDimensions.LongitudinalRadius,
		vehicleDimensions.TransverseRadius,
	)
	rotationVelocityDelta := vector.Delta(k.Current.SixDOFRotationCalc.Velocity, k.Last.SixDOFRotationCalc.Velocity)
	k.Current.SixDOFRotationCalc.Acceleration = vector.Scale(rotationVelocityDelta, accelFactor, accelFactor, accelFactor)
	k.Current.SixDOFRotationCalc.AccelMag = vector.Magnitude(k.Current.SixDOFRotationCalc.Acceleration)
	k.Current.SixDOFRotationCalc.Jerk = (k.Current.SixDOFRotationCalc.AccelMag - k.Last.SixDOFRotationCalc.AccelMag) / windowSeconds
	k.Current.SixDOFRotationCalc.Snap = (k.Current.SixDOFRotationCalc.Jerk - k.Last.SixDOFRotationCalc.Jerk) / windowSeconds

	k.Current.GroundSpeed = float64(gtclient.Telemetry.GroundSpeedMetresPerSecond())
	k.Current.TransmissionGear = gtclient.Telemetry.CurrentGear()
	k.Current.SurgeCalculated = signal.Abs(float64(k.Current.SixDOFRotationCalc.Acceleration.X))
}

// GetSurgeGforce calculates and returns the translational envelope surge G-force based on the current kinematic state.
func (k *State) GetSurgeGforce() float64 {
	var surge float64
	if k.Current.Format == "~" || k.Current.Format == "B" {
		surge = float64(k.Current.SixDOFTranslation.Acceleration.Surge)
	} else {
		surge = float64(k.Current.SurgeCalculated)
	}

	gForce := signal.Abs(surge / GravityConstant)

	return gForce
}

// getTelemetryFormat determines the telemetry format based on the telemetry client's raw telemetry data.
// Currently supports formats "A", "B", and "~".
func getTelemetryFormat(gtClient *gttelemetry.Client) string {
	isAddendum2, _ := gtClient.Telemetry.RawTelemetry.Addendum2Format()
	if isAddendum2 {
		return "~"
	}

	isAddendum1, _ := gtClient.Telemetry.RawTelemetry.Addendum1Format()
	if isAddendum1 {
		return "B"
	}

	return "A"
}

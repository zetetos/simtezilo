package tyres

import (
	"github.com/zetetos/gt-telemetry/v2/pkg/models"
)

// attribute holds the temperature and condition of a single tyre.
type attribute struct {
	temperature float32
	condition   Condition
}

// Tyre provides temperature and condition information for all four tyres.
type Tyre struct {
	optimalLower  float32
	optimalUpper  float32
	coldThreshold float32
	hotThreshold  float32
	frontLeft     attribute
	frontRight    attribute
	rearLeft      attribute
	rearRight     attribute
}

// Position represents the physical location of a tyre on the vehicle.
type Position int

const (
	PositionFront Position = iota
	PositionRear
	PositionFrontLeft
	PositionFrontRight
	PositionRearLeft
	PositionRearRight
)

// Condition represents the operating condition of a tyre.
type Condition string

const (
	ConditionInvalid  Condition = "invalid"
	ConditionMixed    Condition = "mixed"
	ConditionMarginal Condition = "marginal"
	ConditionOptimal  Condition = "optimal"
	ConditionHot      Condition = "hot"
	ConditionCold     Condition = "cold"
)

// String returns the string representation of the Condition.
func (c Condition) String() string {
	return string(c)
}

// String returns the string representation of the Position.
func (p *Position) String() string {
	switch *p {
	case PositionFront:
		return "front"
	case PositionRear:
		return "rear"
	case PositionFrontLeft:
		return "front left"
	case PositionFrontRight:
		return "front right"
	case PositionRearLeft:
		return "rear left"
	case PositionRearRight:
		return "rear right"
	default:
		return "unknown"
	}
}

// New creates a tyreAttributes struct from the given tyre temperatures.
func New(optimalCenter float32, optimalWindow float32, margin float32, tyreTemps models.CornerSet) *Tyre {
	optimalMin := optimalCenter - (optimalWindow / 2)
	optimalMax := optimalCenter + (optimalWindow / 2)

	attributes := Tyre{
		optimalLower:  optimalMin,
		optimalUpper:  optimalMax,
		coldThreshold: optimalMin - margin,
		hotThreshold:  optimalMax + margin,
	}

	attributes.SetTemperatures(tyreTemps)

	return &attributes
}

func (a *Tyre) SetTemperatures(tyreTemps models.CornerSet) {
	a.frontLeft.temperature = tyreTemps.FrontLeft
	a.frontRight.temperature = tyreTemps.FrontRight
	a.rearLeft.temperature = tyreTemps.RearLeft
	a.rearRight.temperature = tyreTemps.RearRight

	a.assessTyreConditions()
}

// ConditionAtPosition returns the temperature of the tyre at the given position.
func (a *Tyre) ConditionAtPosition(position Position) Condition {
	switch position {
	case PositionFront:
		if a.frontLeft.condition == a.frontRight.condition {
			return a.frontLeft.condition
		}

		return ConditionMixed
	case PositionRear:
		if a.rearLeft.condition == a.rearRight.condition {
			return a.rearLeft.condition
		}

		return ConditionMixed
	case PositionFrontLeft:
		return a.frontLeft.condition
	case PositionFrontRight:
		return a.frontRight.condition
	case PositionRearLeft:
		return a.rearLeft.condition
	case PositionRearRight:
		return a.rearRight.condition
	default:
		return ConditionInvalid
	}
}

// TemperatureAtPosition returns the temperature of the tyre at the given position.
func (a *Tyre) TemperatureAtPosition(position Position) float32 {
	switch position {
	case PositionFront:
		return (a.frontLeft.temperature + a.frontRight.temperature) / 2
	case PositionRear:
		return (a.rearLeft.temperature + a.rearRight.temperature) / 2
	case PositionFrontLeft:
		return a.frontLeft.temperature
	case PositionFrontRight:
		return a.frontRight.temperature
	case PositionRearLeft:
		return a.rearLeft.temperature
	case PositionRearRight:
		return a.rearRight.temperature
	default:
		return 0
	}
}

// GeneralCondition returns a summary condition for the entire tyre set.
func (a *Tyre) GeneralCondition() Condition {
	if a.ConditionOptimal() {
		return ConditionOptimal
	}

	axleFront := a.AxleCondition(PositionFront)
	axleRear := a.AxleCondition(PositionRear)
	hotTyres := len(a.PositionsInCondition(ConditionHot))
	coldTyres := len(a.PositionsInCondition(ConditionCold))

	if axleFront == ConditionHot || axleRear == ConditionHot || hotTyres >= 3 {
		return ConditionHot
	}

	if axleFront == ConditionCold || axleRear == ConditionCold || coldTyres >= 3 {
		return ConditionCold
	}

	return ConditionMarginal
}

// GeneralTemperature calculates the average temperature of the entire tyre set.
func (a *Tyre) GeneralTemperature() float32 {
	return (a.frontLeft.temperature + a.frontRight.temperature + a.rearLeft.temperature + a.rearRight.temperature) / 4.0
}

// AxleCondition returns the condition for the given axle (front or rear).
// It returns the shared condition if both tyres on the axle have the same condition,
// otherwise returns ConditionMixed if conditions differ.
// If the axle is not front or rear, it returns ConditionInvalid.
func (a *Tyre) AxleCondition(axle Position) Condition {
	switch axle { //nolint:exhaustive // Only front/rear axles are valid inputs
	case PositionFront:
		if a.frontLeft.condition == a.frontRight.condition {
			return a.frontLeft.condition
		}

		return ConditionMixed
	case PositionRear:
		if a.rearLeft.condition == a.rearRight.condition {
			return a.rearLeft.condition
		}

		return ConditionMixed
	default:
		return ConditionInvalid
	}
}

// PositionsInCondition checks if any tyre has the given condition.
// It returns a slice of tyre positions in that condition.
func (a *Tyre) PositionsInCondition(condition Condition) (positions []Position) {
	for _, position := range []Position{PositionFrontLeft, PositionFrontRight, PositionRearLeft, PositionRearRight} {
		if a.ConditionAtPosition(position) == condition {
			positions = append(positions, position)
		}
	}

	return positions
}

// ConditionOptimal determines if all tyres are in optimal.
// It returns true if the following conditions are met:
// 1. Average temperature of all 4 corners is within the ideal range.
// 2. None of the individual tyres are overheating.
func (a *Tyre) ConditionOptimal() bool {
	// Not optimal if any tyre is hot
	if len(a.PositionsInCondition(ConditionHot)) > 0 {
		return false
	}

	// Not optimal if any tyre is cold
	if len(a.PositionsInCondition(ConditionCold)) > 0 {
		return false
	}

	averageTemp := a.GeneralTemperature()

	if averageTemp < a.optimalLower {
		return false
	}

	if averageTemp > a.optimalUpper {
		return false
	}

	return true
}

// assessTyreConditions evaluates and sets the condition for each tyre based on its temperature.
func (a *Tyre) assessTyreConditions() {
	corners := []*attribute{&a.frontLeft, &a.frontRight, &a.rearLeft, &a.rearRight}

	for _, corner := range corners {
		temp := corner.temperature

		if temp <= 0 {
			corner.condition = ConditionInvalid

			continue
		}

		switch {
		case temp < a.coldThreshold:
			corner.condition = ConditionCold
		case temp > a.hotThreshold:
			corner.condition = ConditionHot
		case temp >= a.optimalLower && temp <= a.optimalUpper:
			corner.condition = ConditionOptimal
		default:
			// In warming-up range, treat as optimal even though not strictly optimal/cold/hot
			corner.condition = ConditionOptimal
		}
	}
}

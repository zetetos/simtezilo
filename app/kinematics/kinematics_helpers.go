package kinematics

import (
	"strconv"
)

// TODO: find a better place for gear constants and functions

const (
	// NeutralGear is the integer value representing neutral gear.
	NeutralGear int = 15

	// ReverseGear is the integer value representing reverse gear.
	ReverseGear int = 0

	// NullGear is the integer value representing an initialised, unknown gear state.
	NullGear int = -100
)

// GearName returns the string representation for a given gear number.
func GearName(gearNum int) string {
	GearMap := map[int]string{
		-100: "NULL",
		0:    "R",
		1:    "1",
		2:    "2",
		3:    "3",
		4:    "4",
		5:    "5",
		6:    "6",
		7:    "7",
		8:    "8",
		9:    "9",
		10:   "10",
		15:   "N",
	}

	gearName, ok := GearMap[gearNum]
	if !ok {
		gearName = strconv.Itoa(gearNum)
	}

	return gearName
}

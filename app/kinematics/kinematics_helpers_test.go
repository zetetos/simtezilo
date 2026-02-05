package kinematics_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/zetetos/simtezilo/app/kinematics"
)

type HelpersTestSuite struct {
	suite.Suite
}

func TestHelpersTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(HelpersTestSuite))
}

func (suite *HelpersTestSuite) TestGearNameReturnsCorrectNameForValidGear() {
	gearMap := map[int]string{
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

	for gearNum, gearName := range gearMap {
		suite.Run("Gear "+strconv.Itoa(gearNum), func() {
			// Arrange
			wantName := gearName
			// Act
			result := kinematics.GearName(gearNum)

			// Assert
			suite.Equal(wantName, result, "Gear name should match expected value")
		})
	}
}

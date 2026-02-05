package app //nolint:testpackage // white-box testing

import (
	"testing"
)

func TestCalculateEngineOverlap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		cylinderAngle   float32
		crankPlaneAngle float32
		chambers        int
		geometry        string
		expectedRange   [2]float64 // min, max expected values
		description     string
	}{
		{
			name:            "V8 90-degree aligned",
			cylinderAngle:   90.0,
			crankPlaneAngle: 90.0,
			chambers:        8,
			geometry:        "V",
			expectedRange:   [2]float64{0.0, 0.3},
			description:     "Perfect alignment should produce minimal overlap",
		},
		{
			name:            "V8 90-degree perpendicular",
			cylinderAngle:   90.0,
			crankPlaneAngle: 0.0,
			chambers:        8,
			geometry:        "V",
			expectedRange:   [2]float64{0.15, 0.3},
			description:     "Perpendicular arrangement should produce maximum overlap",
		},
		{
			name:            "Boxer engine (flat-6)",
			cylinderAngle:   180.0,
			crankPlaneAngle: 0.0,
			chambers:        6,
			geometry:        "H",
			expectedRange:   [2]float64{0.0, 0.1},
			description:     "Boxer layout should have minimal overlap",
		},
		{
			name:            "Inline 4-cylinder",
			cylinderAngle:   0.0,
			crankPlaneAngle: 0.0,
			chambers:        4,
			geometry:        "I",
			expectedRange:   [2]float64{0.0, 0.25},
			description:     "Inline engines should have low to moderate overlap",
		},
		{
			name:            "2-stroke V-twin",
			cylinderAngle:   45.0,
			crankPlaneAngle: 0.0,
			chambers:        2,
			geometry:        "S",
			expectedRange:   [2]float64{0.1, 0.4},
			description:     "2-stroke engines should have higher base overlap",
		},
		{
			name:            "Single rotor Wankel",
			cylinderAngle:   0.0,
			crankPlaneAngle: 0.0,
			chambers:        1,
			geometry:        "K",
			expectedRange:   [2]float64{0.04, 0.06},
			description:     "Single rotor should have minimal overlap",
		},
		{
			name:            "Twin rotor Wankel",
			cylinderAngle:   0.0,
			crankPlaneAngle: 0.0,
			chambers:        2,
			geometry:        "K",
			expectedRange:   [2]float64{0.19, 0.21},
			description:     "Twin rotor should have moderate overlap",
		},
		{
			name:            "Single cylinder",
			cylinderAngle:   0.0,
			crankPlaneAngle: 0.0,
			chambers:        1,
			geometry:        "I",
			expectedRange:   [2]float64{0.0, 0.0},
			description:     "Single cylinder should have no overlap",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := calculatePulseOverlap(
				testCase.cylinderAngle,
				testCase.crankPlaneAngle,
				testCase.chambers,
				testCase.geometry,
			)

			if result < testCase.expectedRange[0] || result > testCase.expectedRange[1] {
				t.Errorf("calculateEngineOverlap() = %v, expected range [%v, %v] for %s",
					result, testCase.expectedRange[0], testCase.expectedRange[1], testCase.description)
			}

			// Ensure result is always within valid bounds
			if result < 0.0 || result > 1.0 {
				t.Errorf("calculateEngineOverlap() = %v, result should be between 0.0 and 1.0", result)
			}

			t.Logf("%s: cylinder=%.1f°, crank=%.1f°, chambers=%d, geometry=%s -> overlap=%.3f",
				testCase.name, testCase.cylinderAngle, testCase.crankPlaneAngle, testCase.chambers, testCase.geometry, result)
		})
	}
}

func TestCalculateEngineOverlapEdgeCases(t *testing.T) {
	t.Parallel()

	// Test angle wraparound (angles > 180 should be treated as 360-angle)
	result1 := calculatePulseOverlap(0.0, 270.0, 8, "V") // 270° difference = 90° effective
	result2 := calculatePulseOverlap(0.0, 90.0, 8, "V")  // 90° difference

	if result1 != result2 {
		t.Errorf("Angle wraparound failed: 270° difference (%v) should equal 90° difference (%v)", result1, result2)
	}

	// Test negative angles
	result3 := calculatePulseOverlap(-45.0, 45.0, 6, "V") // 90° difference
	result4 := calculatePulseOverlap(45.0, -45.0, 6, "V") // 90° difference

	if result3 != result4 {
		t.Errorf("Negative angle handling failed: results should be symmetric (%v vs %v)", result3, result4)
	}
}

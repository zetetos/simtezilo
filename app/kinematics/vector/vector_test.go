package vector_test

import (
	"math"
	"testing"

	"github.com/zetetos/gt-telemetry/v2/pkg/models"
	"github.com/zetetos/simtezilo/app/kinematics/vector"
)

func TestDeltaReturnsDifferenceBetweenVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		vector1  models.Vector
		vector2  models.Vector
		expected models.Vector
	}{
		{
			name:     "both zero vectors",
			vector1:  models.Vector{X: 0, Y: 0, Z: 0},
			vector2:  models.Vector{X: 0, Y: 0, Z: 0},
			expected: models.Vector{X: 0, Y: 0, Z: 0},
		},
		{
			name:     "positive vectors",
			vector1:  models.Vector{X: 5, Y: 10, Z: 15},
			vector2:  models.Vector{X: 2, Y: 3, Z: 4},
			expected: models.Vector{X: 3, Y: 7, Z: 11},
		},
		{
			name:     "negative vectors",
			vector1:  models.Vector{X: -5, Y: -10, Z: -15},
			vector2:  models.Vector{X: -2, Y: -3, Z: -4},
			expected: models.Vector{X: -3, Y: -7, Z: -11},
		},
		{
			name:     "mixed positive and negative",
			vector1:  models.Vector{X: 5, Y: -10, Z: 15},
			vector2:  models.Vector{X: -2, Y: 3, Z: -4},
			expected: models.Vector{X: 7, Y: -13, Z: 19},
		},
		{
			name:     "first vector zero",
			vector1:  models.Vector{X: 0, Y: 0, Z: 0},
			vector2:  models.Vector{X: 5, Y: 10, Z: 15},
			expected: models.Vector{X: -5, Y: -10, Z: -15},
		},
		{
			name:     "second vector zero",
			vector1:  models.Vector{X: 5, Y: 10, Z: 15},
			vector2:  models.Vector{X: 0, Y: 0, Z: 0},
			expected: models.Vector{X: 5, Y: 10, Z: 15},
		},
		{
			name:     "identical vectors",
			vector1:  models.Vector{X: 7, Y: 8, Z: 9},
			vector2:  models.Vector{X: 7, Y: 8, Z: 9},
			expected: models.Vector{X: 0, Y: 0, Z: 0},
		},
		{
			name:     "floating point values",
			vector1:  models.Vector{X: 1.5, Y: 2.5, Z: 3.5},
			vector2:  models.Vector{X: 0.5, Y: 1.5, Z: 2.5},
			expected: models.Vector{X: 1, Y: 1, Z: 1},
		},
		{
			name:     "large values",
			vector1:  models.Vector{X: 1000000, Y: 2000000, Z: 3000000},
			vector2:  models.Vector{X: 500000, Y: 1000000, Z: 1500000},
			expected: models.Vector{X: 500000, Y: 1000000, Z: 1500000},
		},
		{
			name:     "very small values",
			vector1:  models.Vector{X: 0.0001, Y: 0.0002, Z: 0.0003},
			vector2:  models.Vector{X: 0.00005, Y: 0.0001, Z: 0.00015},
			expected: models.Vector{X: 0.00005, Y: 0.0001, Z: 0.00015},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := vector.Delta(testCase.vector1, testCase.vector2)
			if !vectorsEqual(result, testCase.expected, 1e-6) {
				t.Errorf("Delta(%v, %v) = %v, expected %v",
					testCase.vector1, testCase.vector2, result, testCase.expected)
			}
		})
	}
}

func TestMagnitudeReturnsVectorLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		vector   models.Vector
		expected float64
	}{
		{
			name:     "zero vector",
			vector:   models.Vector{X: 0, Y: 0, Z: 0},
			expected: 0,
		},
		{
			name:     "unit vector X",
			vector:   models.Vector{X: 1, Y: 0, Z: 0},
			expected: 1,
		},
		{
			name:     "unit vector Y",
			vector:   models.Vector{X: 0, Y: 1, Z: 0},
			expected: 1,
		},
		{
			name:     "unit vector Z",
			vector:   models.Vector{X: 0, Y: 0, Z: 1},
			expected: 1,
		},
		{
			name:     "negative unit vector X",
			vector:   models.Vector{X: -1, Y: 0, Z: 0},
			expected: 1,
		},
		{
			name:     "3-4-5 triangle in XY plane",
			vector:   models.Vector{X: 3, Y: 4, Z: 0},
			expected: 5,
		},
		{
			name:     "3-4-5 triangle in XZ plane",
			vector:   models.Vector{X: 3, Y: 0, Z: 4},
			expected: 5,
		},
		{
			name:     "3-4-5 triangle in YZ plane",
			vector:   models.Vector{X: 0, Y: 3, Z: 4},
			expected: 5,
		},
		{
			name:     "positive vector all components",
			vector:   models.Vector{X: 1, Y: 2, Z: 2},
			expected: 3,
		},
		{
			name:     "negative vector all components",
			vector:   models.Vector{X: -1, Y: -2, Z: -2},
			expected: 3,
		},
		{
			name:     "mixed positive and negative",
			vector:   models.Vector{X: 2, Y: -3, Z: 6},
			expected: 7,
		},
		{
			name:     "floating point values",
			vector:   models.Vector{X: 1.5, Y: 2.0, Z: 2.5},
			expected: math.Sqrt(1.5*1.5 + 2.0*2.0 + 2.5*2.5),
		},
		{
			name:     "large values",
			vector:   models.Vector{X: 1000, Y: 2000, Z: 2000},
			expected: 3000,
		},
		{
			name:     "very small values",
			vector:   models.Vector{X: 0.001, Y: 0.002, Z: 0.002},
			expected: 0.003,
		},
		{
			name:     "pythagorean quadruple 1-2-2-3",
			vector:   models.Vector{X: 2, Y: 2, Z: 1},
			expected: 3,
		},
		{
			name:     "pythagorean quadruple 2-3-6-7",
			vector:   models.Vector{X: 2, Y: 3, Z: 6},
			expected: 7,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := vector.Magnitude(testCase.vector)
			if !floatsEqual(result, testCase.expected, 1e-6) {
				t.Errorf("Magnitude(%v) = %v, expected %v",
					testCase.vector, result, testCase.expected)
			}
		})
	}
}

func TestScaleMultipliesVectorByFactors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		vector   models.Vector
		xScale   float32
		yScale   float32
		zScale   float32
		expected models.Vector
	}{
		{
			name:     "zero vector with any scale",
			vector:   models.Vector{X: 0, Y: 0, Z: 0},
			xScale:   5,
			yScale:   10,
			zScale:   15,
			expected: models.Vector{X: 0, Y: 0, Z: 0},
		},
		{
			name:     "any vector with zero scales",
			vector:   models.Vector{X: 5, Y: 10, Z: 15},
			xScale:   0,
			yScale:   0,
			zScale:   0,
			expected: models.Vector{X: 0, Y: 0, Z: 0},
		},
		{
			name:     "unit scales (identity)",
			vector:   models.Vector{X: 5, Y: 10, Z: 15},
			xScale:   1,
			yScale:   1,
			zScale:   1,
			expected: models.Vector{X: 5, Y: 10, Z: 15},
		},
		{
			name:     "uniform positive scale",
			vector:   models.Vector{X: 2, Y: 3, Z: 4},
			xScale:   2,
			yScale:   2,
			zScale:   2,
			expected: models.Vector{X: 4, Y: 6, Z: 8},
		},
		{
			name:     "different positive scales",
			vector:   models.Vector{X: 2, Y: 3, Z: 4},
			xScale:   2,
			yScale:   3,
			zScale:   4,
			expected: models.Vector{X: 4, Y: 9, Z: 16},
		},
		{
			name:     "negative scales (flip direction)",
			vector:   models.Vector{X: 2, Y: 3, Z: 4},
			xScale:   -1,
			yScale:   -1,
			zScale:   -1,
			expected: models.Vector{X: -2, Y: -3, Z: -4},
		},
		{
			name:     "mixed positive and negative scales",
			vector:   models.Vector{X: 2, Y: 3, Z: 4},
			xScale:   2,
			yScale:   -3,
			zScale:   0.5,
			expected: models.Vector{X: 4, Y: -9, Z: 2},
		},
		{
			name:     "negative vector with positive scales",
			vector:   models.Vector{X: -2, Y: -3, Z: -4},
			xScale:   2,
			yScale:   3,
			zScale:   4,
			expected: models.Vector{X: -4, Y: -9, Z: -16},
		},
		{
			name:     "negative vector with negative scales",
			vector:   models.Vector{X: -2, Y: -3, Z: -4},
			xScale:   -2,
			yScale:   -3,
			zScale:   -4,
			expected: models.Vector{X: 4, Y: 9, Z: 16},
		},
		{
			name:     "fractional scales",
			vector:   models.Vector{X: 10, Y: 20, Z: 30},
			xScale:   0.5,
			yScale:   0.25,
			zScale:   0.1,
			expected: models.Vector{X: 5, Y: 5, Z: 3},
		},
		{
			name:     "large scales",
			vector:   models.Vector{X: 1, Y: 2, Z: 3},
			xScale:   1000,
			yScale:   1000,
			zScale:   1000,
			expected: models.Vector{X: 1000, Y: 2000, Z: 3000},
		},
		{
			name:     "very small scales",
			vector:   models.Vector{X: 1000, Y: 2000, Z: 3000},
			xScale:   0.001,
			yScale:   0.001,
			zScale:   0.001,
			expected: models.Vector{X: 1, Y: 2, Z: 3},
		},
		{
			name:     "scale only X",
			vector:   models.Vector{X: 5, Y: 10, Z: 15},
			xScale:   3,
			yScale:   1,
			zScale:   1,
			expected: models.Vector{X: 15, Y: 10, Z: 15},
		},
		{
			name:     "scale only Y",
			vector:   models.Vector{X: 5, Y: 10, Z: 15},
			xScale:   1,
			yScale:   3,
			zScale:   1,
			expected: models.Vector{X: 5, Y: 30, Z: 15},
		},
		{
			name:     "scale only Z",
			vector:   models.Vector{X: 5, Y: 10, Z: 15},
			xScale:   1,
			yScale:   1,
			zScale:   3,
			expected: models.Vector{X: 5, Y: 10, Z: 45},
		},
		{
			name:     "floating point vector with floating point scales",
			vector:   models.Vector{X: 1.5, Y: 2.5, Z: 3.5},
			xScale:   1.5,
			yScale:   2.5,
			zScale:   3.5,
			expected: models.Vector{X: 2.25, Y: 6.25, Z: 12.25},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := vector.Scale(testCase.vector, testCase.xScale, testCase.yScale, testCase.zScale)
			if !vectorsEqual(result, testCase.expected, 1e-6) {
				t.Errorf("Scale(%v, %v, %v, %v) = %v, expected %v",
					testCase.vector, testCase.xScale, testCase.yScale, testCase.zScale, result, testCase.expected)
			}
		})
	}
}

// vectorsEqual compares two vectors for equality within a tolerance.
func vectorsEqual(v1, v2 models.Vector, tolerance float64) bool {
	return floatsEqual(float64(v1.X), float64(v2.X), tolerance) &&
		floatsEqual(float64(v1.Y), float64(v2.Y), tolerance) &&
		floatsEqual(float64(v1.Z), float64(v2.Z), tolerance)
}

// floatsEqual compares two floats for equality within a tolerance.
func floatsEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

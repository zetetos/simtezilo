package app_test

import (
	"testing"
	"time"

	"github.com/zetetos/simtezilo/app"
)

func TestFormatDeltaTimeFormatsLapTimeDeltasCorrectly(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		delta    time.Duration
		expected string
	}{
		// Seconds range (>= 0.95s)
		{
			name:     "exactly 1 second",
			delta:    1 * time.Second,
			expected: "1 second",
		},
		{
			name:     "boundary case: 0.95s rounds to 1 second",
			delta:    950 * time.Millisecond,
			expected: "1 second",
		},
		{
			name:     "1.5 seconds",
			delta:    1500 * time.Millisecond,
			expected: "1.5 seconds",
		},
		{
			name:     "2.3 seconds",
			delta:    2300 * time.Millisecond,
			expected: "2.3 seconds",
		},
		{
			name:     "2.0 seconds exactly",
			delta:    2000 * time.Millisecond,
			expected: "2 seconds",
		},
		{
			name:     "10.5 seconds",
			delta:    10500 * time.Millisecond,
			expected: "10.5 seconds",
		},

		// Tenths range (0.095s to 0.949s)
		{
			name:     "0.948s is 0.9 seconds (avoids 10 tenths)",
			delta:    948 * time.Millisecond,
			expected: "0.9 seconds",
		},
		{
			name:     "0.5 seconds is 5 tenths",
			delta:    500 * time.Millisecond,
			expected: "5 tenths",
		},
		{
			name:     "0.1 seconds is 1 tenth",
			delta:    100 * time.Millisecond,
			expected: "1 tenth",
		},
		{
			name:     "boundary case: 0.095s rounds to 1 tenth",
			delta:    95 * time.Millisecond,
			expected: "1 tenth",
		},
		{
			name:     "0.9 seconds is 9 tenths",
			delta:    900 * time.Millisecond,
			expected: "9 tenths",
		},

		// Hundredths range (0.0095s to 0.0949s)
		{
			name:     "0.0948s is 1 tenth (avoids 10 hundredths)",
			delta:    time.Duration(94.8 * float64(time.Millisecond)),
			expected: "1 tenth",
		},
		{
			name:     "0.05 seconds is 5 hundredths",
			delta:    50 * time.Millisecond,
			expected: "5 hundredths",
		},
		{
			name:     "0.01 seconds is 1 hundredth",
			delta:    10 * time.Millisecond,
			expected: "1 hundredth",
		},
		{
			name:     "boundary case: 0.0095s rounds to 1 hundredth",
			delta:    time.Duration(9.5 * float64(time.Millisecond)),
			expected: "1 hundredth",
		},

		// Thou range (< 0.0095s)
		{
			name:     "0.0094s is 1 hundredth (avoids 10 thou)",
			delta:    time.Duration(9.4 * float64(time.Millisecond)),
			expected: "1 hundredth",
		},
		{
			name:     "0.005 seconds is 5 thou",
			delta:    5 * time.Millisecond,
			expected: "5 thou",
		},
		{
			name:     "0.001 seconds is 1 thou",
			delta:    1 * time.Millisecond,
			expected: "1 thou",
		},
		{
			name:     "0.002 seconds is 2 thou",
			delta:    2 * time.Millisecond,
			expected: "2 thou",
		},

		// Negative deltas (faster laps - use floor instead of ceil)
		{
			name:     "negative 1 second",
			delta:    -1 * time.Second,
			expected: "1 second",
		},
		{
			name:     "negative 0.5 seconds is 5 tenths",
			delta:    -500 * time.Millisecond,
			expected: "5 tenths",
		},
		{
			name:     "negative 0.05 seconds is 5 hundredths",
			delta:    -50 * time.Millisecond,
			expected: "5 hundredths",
		},
		{
			name:     "negative 0.005 seconds is 5 thou",
			delta:    -5 * time.Millisecond,
			expected: "5 thou",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := app.FormatDeltaTime(testCase.delta)
			if result != testCase.expected {
				t.Errorf("formatDeltaTime(%v) = %q, want %q", testCase.delta, result, testCase.expected)
			}
		})
	}
}

func TestRoundDeltaRoundsValuesBasedOnSign(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		value    float64
		expected float64
	}{
		{
			name:     "positive value rounds up",
			value:    5.1,
			expected: 6.0,
		},
		{
			name:     "positive exact value",
			value:    5.0,
			expected: 5.0,
		},
		{
			name:     "negative value rounds down (floor)",
			value:    -5.1,
			expected: 6.0,
		},
		{
			name:     "negative exact value",
			value:    -5.0,
			expected: 5.0,
		},
		{
			name:     "positive small fraction rounds up",
			value:    0.1,
			expected: 1.0,
		},
		{
			name:     "negative small fraction rounds down",
			value:    -0.1,
			expected: 1.0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := app.RoundDelta(testCase.value)
			if result != testCase.expected {
				t.Errorf("roundDelta(%v) = %v, want %v", testCase.value, result, testCase.expected)
			}
		})
	}
}

func TestPluraliseDeltaHandlesSingularAndPluralForms(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		value    float64
		scale    string
		expected string
	}{
		{
			name:     "1 tenth singular",
			value:    1.0,
			scale:    "tenth",
			expected: "1 tenth",
		},
		{
			name:     "2 tenths plural",
			value:    2.0,
			scale:    "tenth",
			expected: "2 tenths",
		},
		{
			name:     "1 hundredth singular",
			value:    1.0,
			scale:    "hundredth",
			expected: "1 hundredth",
		},
		{
			name:     "5 hundredths plural",
			value:    5.0,
			scale:    "hundredth",
			expected: "5 hundredths",
		},
		{
			name:     "1 thou no pluralization",
			value:    1.0,
			scale:    "thou",
			expected: "1 thou",
		},
		{
			name:     "5 thou no pluralization",
			value:    5.0,
			scale:    "thou",
			expected: "5 thou",
		},
		{
			name:     "0 tenths plural",
			value:    0.0,
			scale:    "tenth",
			expected: "0 tenths",
		},
		{
			name:     "9 tenths plural",
			value:    9.0,
			scale:    "tenth",
			expected: "9 tenths",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := app.PluraliseDelta(testCase.value, testCase.scale)
			if result != testCase.expected {
				t.Errorf("pluraliseDelta(%v, %q) = %q, want %q", testCase.value, testCase.scale, result, testCase.expected)
			}
		})
	}
}

func TestPronounceTimeFormatsTimeComponentsForSpeechWithUnits(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		minutes      string
		seconds      string
		milliseconds string
		expected     string
	}{
		{
			name:         "seconds, milliseconds",
			minutes:      "0",
			seconds:      "06",
			milliseconds: "504",
			expected:     "6.504 seconds",
		},
		{
			name:         "minute, milliseconds",
			minutes:      "1",
			seconds:      "00",
			milliseconds: "456",
			expected:     "1 minute 0.456 seconds",
		},
		{
			name:         "single digit seconds, milliseconds",
			minutes:      "0",
			seconds:      "02",
			milliseconds: "001",
			expected:     "2.001 seconds",
		},
		{
			name:         "double digit seconds, milliseconds",
			minutes:      "0",
			seconds:      "30",
			milliseconds: "001",
			expected:     "30.001 seconds",
		},
		{
			name:         "minute, second",
			minutes:      "1",
			seconds:      "01",
			milliseconds: "000",
			expected:     "1 minute 1 second flat",
		},
		{
			name:         "minutes, seconds",
			minutes:      "1",
			seconds:      "02",
			milliseconds: "000",
			expected:     "1 minute 2 seconds flat",
		},
		{
			name:         "minute, second, milliseconds",
			minutes:      "1",
			seconds:      "01",
			milliseconds: "001",
			expected:     "1 minute 1.001 seconds",
		},
		{
			name:         "minutes, second, milliseconds",
			minutes:      "2",
			seconds:      "01",
			milliseconds: "001",
			expected:     "2 minutes 1.001 seconds",
		},
		{
			name:         "minutes, seconds, milliseconds",
			minutes:      "2",
			seconds:      "02",
			milliseconds: "001",
			expected:     "2 minutes 2.001 seconds",
		},
		{
			name:         "exactly 1 second",
			minutes:      "0",
			seconds:      "01",
			milliseconds: "000",
			expected:     "1 second flat",
		},
		{
			name:         "exactly 2 seconds",
			minutes:      "0",
			seconds:      "02",
			milliseconds: "000",
			expected:     "2 seconds flat",
		},
		{
			name:         "exactly 1 minute",
			minutes:      "1",
			seconds:      "0",
			milliseconds: "000",
			expected:     "1 minute flat",
		},
		{
			name:         "exactly 2 minutes",
			minutes:      "2",
			seconds:      "00",
			milliseconds: "000",
			expected:     "2 minutes flat",
		},
		{
			name:         "seconds leading zeros stripped",
			minutes:      "1",
			seconds:      "02",
			milliseconds: "123",
			expected:     "1 minute 2.123 seconds",
		},
		{
			name:         "millisecond trailing zeros stripped",
			minutes:      "0",
			seconds:      "05",
			milliseconds: "050",
			expected:     "5.05 seconds",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := app.PronounceTime(testCase.minutes, testCase.seconds, testCase.milliseconds, true)
			if result != testCase.expected {
				t.Errorf("pronounceTime(%q, %q, %q, %v) = %q, want %q",
					testCase.minutes, testCase.seconds, testCase.milliseconds, true, result, testCase.expected)
			}
		})
	}
}

func TestPronounceTimeFormatsTimeComponentsForSpeechWithoutUnits(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		minutes      string
		seconds      string
		milliseconds string
		expected     string
	}{
		{
			name:         "single digit seconds, milliseconds",
			minutes:      "0",
			seconds:      "06",
			milliseconds: "504",
			expected:     "6.504",
		},
		{
			name:         "double digit seconds, milliseconds",
			minutes:      "0",
			seconds:      "30",
			milliseconds: "001",
			expected:     "30.001",
		},
		{
			name:         "minutes, milliseconds",
			minutes:      "1",
			seconds:      "00",
			milliseconds: "456",
			expected:     "1 0.456",
		},
		{
			name:         "minutes, second",
			minutes:      "1",
			seconds:      "02",
			milliseconds: "000",
			expected:     "1:02 flat",
		},
		{
			name:         "minutes, second, milliseconds",
			minutes:      "1",
			seconds:      "01",
			milliseconds: "001",
			expected:     "1:01.001",
		},
		{
			name:         "exactly 1 second",
			minutes:      "0",
			seconds:      "01",
			milliseconds: "000",
			expected:     "1 second flat",
		},
		{
			name:         "exactly 2 seconds",
			minutes:      "0",
			seconds:      "06",
			milliseconds: "000",
			expected:     "6 seconds flat",
		},
		{
			name:         "exactly 1 minute",
			minutes:      "1",
			seconds:      "00",
			milliseconds: "000",
			expected:     "1 minute flat",
		},
		{
			name:         "exactly 2 minutes",
			minutes:      "2",
			seconds:      "00",
			milliseconds: "000",
			expected:     "2 minutes flat",
		},
		{
			name:         "seconds leading zeros not stripped",
			minutes:      "1",
			seconds:      "02",
			milliseconds: "123",
			expected:     "1:02.123",
		},
		{
			name:         "millisecond trailing zeros not stripped",
			minutes:      "1",
			seconds:      "02",
			milliseconds: "050",
			expected:     "1:02.050",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := app.PronounceTime(testCase.minutes, testCase.seconds, testCase.milliseconds, false)
			if result != testCase.expected {
				t.Errorf("pronounceTime(%q, %q, %q, %v) = %q, want %q",
					testCase.minutes, testCase.seconds, testCase.milliseconds, false, result, testCase.expected)
			}
		})
	}
}

func TestFormatDurationFormatsLapTimesCorrectly(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		// Zero and sub-second times
		{
			name:     "zero duration",
			duration: 0,
			expected: "0 seconds flat",
		},
		{
			name:     "sub-second with milliseconds",
			duration: 500 * time.Millisecond,
			expected: "0.500",
		},
		{
			name:     "sub-second with partial milliseconds",
			duration: 123 * time.Millisecond,
			expected: "0.123",
		},

		// Seconds only (flat times)
		{
			name:     "exactly 1 second",
			duration: 1 * time.Second,
			expected: "1 second flat",
		},
		{
			name:     "multiple seconds flat",
			duration: 30 * time.Second,
			expected: "30 seconds flat",
		},

		// Seconds with milliseconds
		{
			name:     "seconds with milliseconds",
			duration: 5*time.Second + 123*time.Millisecond,
			expected: "5.123",
		},
		{
			name:     "seconds with trailing zero milliseconds",
			duration: 12*time.Second + 50*time.Millisecond,
			expected: "12.050",
		},

		// Minutes only (flat times)
		{
			name:     "exactly 1 minute",
			duration: 1 * time.Minute,
			expected: "1 minute flat",
		},
		{
			name:     "multiple minutes flat",
			duration: 2 * time.Minute,
			expected: "2 minutes flat",
		},

		// Minutes with seconds (flat times)
		{
			name:     "minutes and seconds flat",
			duration: 1*time.Minute + 30*time.Second,
			expected: "1:30 flat",
		},
		{
			name:     "minutes with leading zero seconds flat",
			duration: 1*time.Minute + 5*time.Second,
			expected: "1:05 flat",
		},

		// Minutes with milliseconds only (no whole seconds)
		{
			name:     "minutes with only milliseconds",
			duration: 1*time.Minute + 500*time.Millisecond,
			expected: "1 0.500",
		},

		// Full lap times (minutes, seconds, milliseconds)
		{
			name:     "typical lap time",
			duration: 1*time.Minute + 34*time.Second + 567*time.Millisecond,
			expected: "1:34.567",
		},
		{
			name:     "lap time with leading zero seconds",
			duration: 1*time.Minute + 5*time.Second + 123*time.Millisecond,
			expected: "1:05.123",
		},
		{
			name:     "lap time with trailing zero milliseconds",
			duration: 2*time.Minute + 15*time.Second + 50*time.Millisecond,
			expected: "2:15.050",
		},
		{
			name:     "longer lap time",
			duration: 8*time.Minute + 42*time.Second + 999*time.Millisecond,
			expected: "8:42.999",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			duration := testCase.duration

			// Act
			result := app.FormatDuration(duration)

			// Assert
			if result != testCase.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", duration, result, testCase.expected)
			}
		})
	}
}

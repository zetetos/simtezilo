package app

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// FormatDeltaTime formats a time delta for slower laps in tenths, hundredths, or thousandths.
func FormatDeltaTime(delta time.Duration) string {
	absSeconds := math.Abs(delta.Seconds())

	var (
		value float64
		units string
	)

	switch {
	case absSeconds > 0.90:
		// Values > 0.90s might ceil to 10 tenths, so treat as seconds
		rounded := math.Round(absSeconds*10) / 10

		unit := "seconds"
		if rounded == 1.0 {
			unit = "second"
		}

		if rounded == math.Floor(rounded) {
			return fmt.Sprintf("%.0f %s", rounded, unit)
		}

		return fmt.Sprintf("%.1f %s", rounded, unit)
	case absSeconds > 0.090:
		// Values > 0.090s might ceil to 10 hundredths, so treat as tenths
		value = delta.Seconds() * 10
		units = "tenth"
	case absSeconds > 0.0090:
		// Values > 0.0090s might ceil to 10 thou, so treat as hundredths
		value = delta.Seconds() * 100
		units = "hundredth"
	default:
		value = float64(delta.Milliseconds())
		units = "thou"
	}

	return PluraliseDelta(RoundDelta(value), units)
}

// RoundDelta rounds the delta value up or down based on whether it's faster or slower.
func RoundDelta(value float64) float64 {
	if value < 0 {
		return math.Abs(math.Floor(value))
	}

	return math.Abs(math.Ceil(value))
}

func PluraliseDelta(value float64, scale string) (format string) {
	roundedValue := int(value)

	format = strconv.Itoa(roundedValue) + " " + scale

	// Thousandths do not get pluralised
	if scale == "thou" {
		return format
	}

	if roundedValue != 1 {
		format += "s"
	}

	return format
}

// FormatDuration formats a time.Duration value for text and speech output.
func FormatDuration(lapTime time.Duration) string {
	minutes := int(lapTime.Minutes())
	lapTime -= time.Duration(minutes) * time.Minute

	seconds := int(lapTime.Seconds())
	lapTime -= time.Duration(seconds) * time.Second

	milliseconds := int(lapTime.Milliseconds())

	minutesStr := strconv.Itoa(minutes)

	var secondsStr string

	secondsFmt := "%02d"
	if minutesStr == "0" {
		secondsFmt = "%d"
	}

	secondsStr = fmt.Sprintf(secondsFmt, seconds)

	millisecondsStr := fmt.Sprintf("%03d", milliseconds)

	return PronounceTime(minutesStr, secondsStr, millisecondsStr, false)
}

// PronounceTime formats minutes, seconds and millisecond time components for text and speech output.
func PronounceTime(minutes string, seconds string, milliseconds string, includeUnits bool) string {
	secondsStripped := strings.TrimLeft(seconds, "0")
	if secondsStripped == "" {
		secondsStripped = "0"
	}

	hasMinutes := minutes != "0"
	hasSeconds := secondsStripped != "0"
	hasMilliseconds := milliseconds != "000"

	if includeUnits {
		return pronounceTimeWithUnits(minutes, secondsStripped, milliseconds, hasMinutes, hasSeconds, hasMilliseconds)
	}

	return pronounceTimeWithoutUnits(minutes, seconds, secondsStripped, milliseconds, hasMinutes, hasSeconds, hasMilliseconds)
}

func pronounceTimeWithUnits(minutes, seconds, milliseconds string, hasMinutes, hasSeconds, hasMilliseconds bool) string {
	var parts []string

	if hasMinutes {
		unit := "minute"
		if minutes != "1" {
			unit = "minutes"
		}

		if !hasSeconds && !hasMilliseconds {
			return minutes + " " + unit + " flat"
		}

		parts = append(parts, minutes+" "+unit)
	}

	secondsPart := seconds
	if hasMilliseconds {
		secondsPart += "." + strings.TrimRight(milliseconds, "0")
	}

	switch {
	case hasMilliseconds:
		parts = append(parts, secondsPart+" seconds")
	case seconds == "1":
		parts = append(parts, seconds+" second flat")
	default:
		parts = append(parts, seconds+" seconds flat")
	}

	return strings.Join(parts, " ")
}

func pronounceTimeWithoutUnits(minutes, seconds, secondsStripped, milliseconds string, hasMinutes, hasSeconds, hasMilliseconds bool) string {
	// Has milliseconds - format with decimal
	if hasMilliseconds {
		return formatWithMilliseconds(minutes, seconds, secondsStripped, milliseconds, hasMinutes, hasSeconds)
	}

	// No milliseconds - format as "flat" time
	return formatFlatTime(minutes, seconds, secondsStripped, hasMinutes, hasSeconds)
}

func formatWithMilliseconds(minutes, seconds, secondsStripped, milliseconds string, hasMinutes, hasSeconds bool) string {
	// No minutes: strip leading zeros from seconds
	if !hasMinutes {
		return secondsStripped + "." + milliseconds
	}

	// Minutes but 0 seconds: "M 0.xxx" (space separator)
	if !hasSeconds {
		return minutes + " 0." + milliseconds
	}

	// Minutes, seconds, and milliseconds: "M:SS.xxx"
	return minutes + ":" + seconds + "." + milliseconds
}

func formatFlatTime(minutes, seconds, secondsStripped string, hasMinutes, hasSeconds bool) string {
	// Only seconds, no minutes: "X second(s) flat"
	if !hasMinutes {
		unit := "seconds"
		if secondsStripped == "1" {
			unit = "second"
		}

		return secondsStripped + " " + unit + " flat"
	}

	// Minutes but no seconds: "X minute(s) flat"
	if !hasSeconds {
		unit := "minute"
		if minutes != "1" {
			unit = "minutes"
		}

		return minutes + " " + unit + " flat"
	}

	// Minutes and seconds: "M:SS flat"
	return minutes + ":" + seconds + " flat"
}

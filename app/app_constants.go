package app

import "time"

const frameRate = 60 // 60 Hz

const (
	pitRadioFrameRate     = 1   // 1 Hz
	displayFrameRate      = 15  // 15 Hz
	engineHapticFrameRate = 30  // 30 Hz
	hapticFrameRate       = 120 // 120 Hz
	telemetryFrameRate    = 60  // 60 Hz
)

// TODO: should these be user configurable?
const (
	tyreConditionStablisationTime = 5 * time.Second
	tyreInterNotifyGap            = 30 * time.Second
)

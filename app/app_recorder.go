package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kennygrant/sanitize"
	"github.com/zetetos/simtezilo/app/codec"
	"github.com/zetetos/simtezilo/app/pitradio"
)

// recordingState tracks telemetry recording state.
type recordingState struct {
	startTime        time.Time // When recording started
	filepath         string    // Current recording file path and name
	lastTriggerState bool      // Previous state of the trigger input
	lastTriggerID    uint32    // Sequence ID of last trigger change
	triggerCount     int       // Number of trigger changes in detection window
	triggerHistory   []uint32  // Sequence IDs of recent trigger changes
}

// resetRecording resets the recording state to a non-recording state.
func (r *recordingState) resetRecording() {
	r.startTime = time.Time{}
	r.filepath = ""
}

// resetTrigger resets the recording trigger state.
func (r *recordingState) resetTrigger() {
	r.triggerHistory = []uint32{}
	r.triggerCount = 0
}

// filePath returns the current recording file path.
func (r *recordingState) getFilePath() string {
	return r.filepath
}

// triggerCount returns the number of trigger events in the detection window.
func (r *recordingState) getTriggerCount() int {
	return r.triggerCount
}

// triggerStateHasChanged checks if the trigger state has changed since the last check.
func (r *recordingState) triggerStateHasChanged(triggerState bool) bool {
	if triggerState == r.lastTriggerState {
		return false
	}

	r.lastTriggerState = triggerState

	return true
}

// addTriggerEvent adds a trigger event to the recording trigger history.
func (r *recordingState) addTriggerEvent(sequenceID uint32) {
	r.lastTriggerID = sequenceID
	r.triggerHistory = append(r.triggerHistory, sequenceID)

	r.assessTriggerWindow(sequenceID)
}

// assessTriggerWindow determines which trigger events are within the detection window.
func (r *recordingState) assessTriggerWindow(currentSequenceID uint32) {
	toggleWindowSeconds := uint32(1) // Remove toggles older than 2 seconds
	windowMinID := currentSequenceID - (toggleWindowSeconds * frameRate)

	triggersInWindow := []uint32{}

	for _, triggerSequenceID := range r.triggerHistory {
		if triggerSequenceID > windowMinID {
			triggersInWindow = append(triggersInWindow, triggerSequenceID)
		}
	}

	r.triggerHistory = triggersInWindow
	r.triggerCount = len(triggersInWindow)
}

// triggerActive checks if the recording trigger condition is met.
func (r *recordingState) triggerActive() bool {
	requiredToggles := 3

	return r.triggerCount >= requiredToggles
}

// setStart sets the start time and file path for the recording.
func (r *recordingState) setStart(startTime time.Time, filePath string) {
	r.startTime = startTime
	r.filepath = filePath
}

// setStop stops the recording and returns the total duration.
func (r *recordingState) setStop() (duration time.Duration) {
	duration = time.Since(r.startTime)

	r.resetRecording()

	return duration
}

// manageRecordingState handles recording state based on application conditions.
func (a *App) manageRecordingState() {
	// Stop recording when entering the post-race menu
	if a.state.isInPostRaceMenu {
		a.stopRecording()

		return
	}

	a.detectRecordingTrigger()
}

// detectRecordingTrigger processes high beam state changes and triggers recording when toggled 3 times quickly.
func (a *App) detectRecordingTrigger() {
	if !a.shouldDetectTrigger() {
		return
	}

	triggerState := a.gtClient.Telemetry.Flags().HighBeamActive

	if !a.state.recorder.triggerStateHasChanged(triggerState) {
		return
	}

	// Only trigger on high state
	if !triggerState {
		return
	}

	currentSequenceID := a.state.current.sequenceNumber

	a.state.recorder.addTriggerEvent(currentSequenceID)

	a.log.Info().
		Int("toggle_count", a.state.recorder.getTriggerCount()).
		Uint32("sequence_id", currentSequenceID).
		Bool("high_beam_active", triggerState).
		Msg("Recording trigger toggle detected")

	if a.state.recorder.triggerActive() {
		a.toggleRecording()
		a.state.recorder.resetTrigger()
	}
}

// shouldDetectTrigger checks if conditions are met to monitor for recording triggers.
func (a *App) shouldDetectTrigger() bool {
	isReplaySource, err := a.gtClient.IsReplaySource()
	if err != nil {
		a.log.Error().
			Err(err).
			Msg("GT Client replay source check")

		return false
	}

	// Ignore recording triggers for replays
	if !a.gtClient.Telemetry.Flags().Live || isReplaySource {
		return false
	}

	return true
}

// toggleRecording starts or stops telemetry recording.
func (a *App) toggleRecording() {
	if a.gtClient.IsRecording() {
		a.stopRecording()
	} else {
		a.startRecording()
	}
}

// startRecording begins telemetry capture.
func (a *App) startRecording() {
	if a.gtClient.IsRecording() {
		a.notifyRecordingEvent("error")

		a.log.Warn().Msg("Recording already in progress")

		return
	}

	timestamp := time.Now()

	filepath := filepath.Join(
		a.config.GetAppBaseDir(),
		"data",
		"replays",
		a.generateRecordingFilename(timestamp),
	)

	a.state.recorder.setStart(timestamp, filepath)

	err := a.gtClient.StartRecording(filepath)
	if err != nil {
		a.state.recorder.resetRecording()

		a.notifyRecordingEvent("error")

		a.log.Error().
			Err(err).
			Str("file", filepath).
			Msg("Start recording")

		return
	}

	a.notifyRecordingEvent("start")

	a.log.Info().
		Str("file", filepath).
		Msg("Start recording")
}

// stopRecording ends telemetry capture.
func (a *App) stopRecording() {
	a.state.recorder.resetTrigger()

	if !a.gtClient.IsRecording() {
		return
	}

	filePath := a.state.recorder.getFilePath()

	err := a.gtClient.StopRecording()
	if err != nil {
		a.notifyRecordingEvent("error")

		a.log.Error().
			Err(err).
			Str("file", filePath).
			Msg("Stop recording")
	}

	duration := a.state.recorder.setStop()

	a.notifyRecordingEvent("stop")

	a.log.Info().
		Str("file", filePath).
		Dur("duration", duration).
		Msg("Stop recording")
}

// notifyRecordingEvent sends a pitRadio notification for recording events.
func (a *App) notifyRecordingEvent(event string) {
	if !a.config.PitRadioEnabled() || a.pitRadio == nil {
		return
	}

	var sample string

	switch event {
	case "start":
		sample = "recordingStartTone"
	case "stop":
		sample = "recordingStopTone"
	case "error":
		sample = "errorTone"
	default:
		return
	}

	effectSample := a.synth.EffectSampleBank().GetSample(sample, codec.OpusSampleRate)

	dcaData, err := effectSample.ToDCA()
	if err != nil {
		a.log.Error().
			Err(err).
			Msg("generate talk permit tone")
	} else {
		err := a.pitRadio.Send(pitradio.Message{
			MessageType: pitradio.AudioMessage,
			Text:        "recording " + event,
			Audio:       dcaData,
			NoCache:     false,
		})
		if err != nil {
			a.log.Error().
				Err(err).
				Msg("send message")
		}
	}
}

// generateRecordingFilename creates a filename for the telemetry recording.
func (a *App) generateRecordingFilename(timestampe time.Time) string {
	trackName := a.sanitizeFilename(a.circuit.Name())
	if trackName == "" {
		trackName = "unknown_track"
	}

	vehicleManufacturer := a.sanitizeFilename(a.gtClient.Telemetry.VehicleManufacturer())
	if vehicleManufacturer == "" {
		vehicleManufacturer = "unknown_manufacturer"
	}

	vehicleModel := a.sanitizeFilename(a.gtClient.Telemetry.VehicleModel())
	if vehicleModel == "" {
		vehicleModel = "unknown_model"
	}

	// format: yyyymmdd.hhmmss-track_name-vehicle_manufacturer_vehicle_model.gtz
	filename := fmt.Sprintf("%s-%s-%s-%s.gtz",
		timestampe.Format("20060102.150405"),
		trackName,
		vehicleManufacturer,
		vehicleModel,
	)

	return filename
}

// sanitizeFilename cleans a string to be safe for use as a filename.
func (a *App) sanitizeFilename(input string) string {
	if input == "" {
		return input
	}

	result := sanitize.Name(input)
	result = strings.ReplaceAll(result, " ", "_")

	return result
}

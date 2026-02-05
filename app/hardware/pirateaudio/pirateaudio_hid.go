package pirateaudio

import (
	"sync"

	"github.com/zetetos/simtezilo/app/hardware"
	"github.com/zetetos/simtezilo/app/ui"
)

// HIDMapper manages the button-to-event mappings for Pirate Audio hardware,
// allowing dynamic remapping when display orientation changes.
type HIDMapper struct {
	mu           sync.RWMutex
	mapping      []ui.HIDInputEvent
	hidEventChan chan ui.HIDInputEvent
	buttonsSetup bool
}

var hidMapper *HIDMapper //nolint:gochecknoglobals // singleton for hardware HID mapping

// SetupHID configures the HID input event mapping of the Pirate Audio device buttons based on the device orientation.
func SetupHID(orientation int, hidEvent chan ui.HIDInputEvent) {
	hidMapper = &HIDMapper{
		hidEventChan: hidEvent,
		mapping:      make([]ui.HIDInputEvent, 4),
	}

	hidMapper.UpdateOrientation(orientation)
	hidMapper.setupButtons()
}

// UpdateOrientation updates the button mappings based on the new orientation.
func UpdateOrientation(orientation int) {
	if hidMapper != nil {
		hidMapper.UpdateOrientation(orientation)
	}
}

// UpdateOrientation recalculates the button mappings based on the new orientation.
func (h *HIDMapper) UpdateOrientation(orientation int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	rotationOffset := (orientation / 90) % 4

	baseMapping := []ui.HIDInputEvent{
		ui.HIDInputLeft,  // Button A
		ui.HIDInputRight, // Button X
		ui.HIDInputUp,    // Button Y
		ui.HIDInputDown,  // Button B
	}

	for i := range 4 {
		h.mapping[i] = baseMapping[(i-rotationOffset+4)%4]
	}
}

// setupButtons registers the GPIO button callbacks. This should only be called once.
func (h *HIDMapper) setupButtons() {
	if h.buttonsSetup {
		return
	}

	h.buttonsSetup = true

	OnButtonAPressed(func() {
		h.mu.RLock()
		event := h.mapping[0]
		h.mu.RUnlock()

		h.hidEventChan <- event
	})

	OnButtonXPressed(func() {
		h.mu.RLock()
		event := h.mapping[1]
		h.mu.RUnlock()

		h.hidEventChan <- event
	})

	OnButtonYPressed(func() {
		h.mu.RLock()
		event := h.mapping[2]
		h.mu.RUnlock()

		h.hidEventChan <- event
	})

	OnButtonBPressed(func() {
		h.mu.RLock()
		event := h.mapping[3]
		h.mu.RUnlock()

		h.hidEventChan <- event
	})
}

// OnButtonAPressed registers a callback function to be called when the A button is pressed.
func OnButtonAPressed(callback func()) {
	hardware.OnGPIOButtonPressed(5, callback)
}

// OnButtonBPressed registers a callback function to be called when the B buttonis pressed.
func OnButtonBPressed(callback func()) {
	hardware.OnGPIOButtonPressed(6, callback)
}

// OnButtonXPressed registers a callback function to be called when the X button is pressed.
func OnButtonXPressed(callback func()) {
	hardware.OnGPIOButtonPressed(16, callback)
}

// OnButtonYPressed registers a callback function to be called when the Y button is pressed.
func OnButtonYPressed(callback func()) {
	hardware.OnGPIOButtonPressed(24, callback)
}

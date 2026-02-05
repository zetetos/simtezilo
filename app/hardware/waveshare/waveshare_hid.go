package waveshare

import (
	"sync"

	"github.com/zetetos/simtezilo/app/hardware"
	"github.com/zetetos/simtezilo/app/ui"
)

// HIDMapper manages the button-to-event mappings for Waveshare hardware,
// allowing dynamic remapping when display orientation changes.
type HIDMapper struct {
	mu           sync.RWMutex
	dpadMapping  []ui.HIDInputEvent
	auxMapping   []ui.HIDInputEvent
	hidEventChan chan ui.HIDInputEvent
	buttonsSetup bool
}

var hidMapper *HIDMapper //nolint:gochecknoglobals // singleton for hardware HID mapping

// SetupHID configures the HID input event mapping of the Waveshare device buttons based on the device orientation.
func SetupHID(orientation int, hidEvent chan ui.HIDInputEvent) {
	hidMapper = &HIDMapper{
		hidEventChan: hidEvent,
		dpadMapping:  make([]ui.HIDInputEvent, 4),
		auxMapping:   make([]ui.HIDInputEvent, 3),
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

	baseDpadMapping := []ui.HIDInputEvent{
		ui.HIDInputUp,    // Dpad Up
		ui.HIDInputRight, // Dpad Right
		ui.HIDInputLeft,  // Dpad Down
		ui.HIDInputDown,  // Dpad Left
	}

	// Rotate Dpad mapping based on orientation
	for i := range 4 {
		h.dpadMapping[i] = baseDpadMapping[(i-rotationOffset+4)%4]
	}

	baseAuxMapping := []ui.HIDInputEvent{
		ui.HIDInputEscape, // Button 1
		ui.HIDInputNone,   // Button 2
		ui.HIDInputPower,  // Button 3
	}

	// Reverse auxiliary button order at 90 and 270 degree orientation
	if rotationOffset == 1 || rotationOffset == 3 {
		for i := range 3 {
			h.auxMapping[i] = baseAuxMapping[2-i]
		}
	} else {
		copy(h.auxMapping, baseAuxMapping)
	}
}

// setupButtons registers the GPIO button callbacks. This should only be called once.
func (h *HIDMapper) setupButtons() {
	if h.buttonsSetup {
		return
	}

	h.buttonsSetup = true

	// OnButtonUpPressed registers a callback function to be called when the up button is pressed.
	OnButtonUpPressed(func() {
		h.mu.RLock()
		event := h.dpadMapping[0]
		h.mu.RUnlock()

		h.hidEventChan <- event
	})

	// OnButtonRightPressed registers a callback function to be called when the right button is pressed.
	OnButtonRightPressed(func() {
		h.mu.RLock()
		event := h.dpadMapping[1]
		h.mu.RUnlock()

		h.hidEventChan <- event
	})

	// OnButtonDownPressed registers a callback function to be called when the down button is pressed.
	OnButtonDownPressed(func() {
		h.mu.RLock()
		event := h.dpadMapping[2]
		h.mu.RUnlock()

		h.hidEventChan <- event
	})

	// OnButtonLeftPressed registers a callback function to be called when the left button is pressed.
	OnButtonLeftPressed(func() {
		h.mu.RLock()
		event := h.dpadMapping[3]
		h.mu.RUnlock()

		h.hidEventChan <- event
	})

	// OnButtonCenterPressed registers a callback function to be called when the center button is pressed.
	OnButtonCenterPressed(func() {
		h.hidEventChan <- ui.HIDInputEnter
	})

	// OnButtonOnePressed registers a callback function to be called when button 1 is pressed.
	OnButtonOnePressed(func() {
		h.mu.RLock()
		event := h.auxMapping[0]
		h.mu.RUnlock()

		h.hidEventChan <- event
	})

	// OnButtonTwoPressed registers a callback function to be called when button 2 is pressed.
	OnButtonTwoPressed(func() {
		h.mu.RLock()
		event := h.auxMapping[1]
		h.mu.RUnlock()

		h.hidEventChan <- event
	})

	// OnButtonThreePressed registers a callback function to be called when button 3 is pressed.
	OnButtonThreePressed(func() {
		h.mu.RLock()
		event := h.auxMapping[2]
		h.mu.RUnlock()

		h.hidEventChan <- event
	})
}

func OnButtonUpPressed(callback func()) {
	hardware.OnGPIOButtonPressed(6, callback)
}

func OnButtonDownPressed(callback func()) {
	hardware.OnGPIOButtonPressed(19, callback)
}

func OnButtonLeftPressed(callback func()) {
	hardware.OnGPIOButtonPressed(5, callback)
}

func OnButtonRightPressed(callback func()) {
	hardware.OnGPIOButtonPressed(26, callback)
}

func OnButtonCenterPressed(callback func()) {
	hardware.OnGPIOButtonPressed(13, callback)
}

func OnButtonOnePressed(callback func()) {
	hardware.OnGPIOButtonPressed(21, callback)
}

func OnButtonTwoPressed(callback func()) {
	hardware.OnGPIOButtonPressed(20, callback)
}

func OnButtonThreePressed(callback func()) {
	hardware.OnGPIOButtonPressed(16, callback)
}

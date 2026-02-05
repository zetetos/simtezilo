package spotpear

import (
	"github.com/zetetos/simtezilo/app/hardware"
	"github.com/zetetos/simtezilo/app/ui"
)

// SetupHID configures the Spotpear hardware buttons to send HID events to the provided channel.
// TODO: implement button rotation logic.
func SetupHID(hidEvent chan ui.HIDInputEvent) {
	OnButtonUpPressed(func() {
		hidEvent <- ui.HIDInputUp
	})

	OnButtonDownPressed(func() {
		hidEvent <- ui.HIDInputDown
	})

	OnButtonLeftPressed(func() {
		hidEvent <- ui.HIDInputLeft
	})

	OnButtonRightPressed(func() {
		hidEvent <- ui.HIDInputRight
	})

	OnButtonAPressed(func() {
		hidEvent <- ui.HIDInputEnd
	})

	OnButtonBPressed(func() {
		hidEvent <- ui.HIDInputPageDown
	})

	OnButtonXPressed(func() {
		hidEvent <- ui.HIDInputPageUp
	})

	OnButtonYPressed(func() {
		hidEvent <- ui.HIDInputHome
	})

	OnButtonTriggerLeftPressed(func() {
		hidEvent <- ui.HIDInputEscape
	})

	OnButtonTriggerRightPressed(func() {
		hidEvent <- ui.HIDInputTab
	})

	OnButtonStartPressed(func() {
		hidEvent <- ui.HIDInputPower
	})

	OnButtonSelectPressed(func() {
		hidEvent <- ui.HIDInputEnter
	})
}

// OnButtonUpPressed registers a callback function to be called when the up button is pressed.
func OnButtonUpPressed(callback func()) {
	hardware.OnGPIOButtonPressed(5, callback)
}

// OnButtonDownPressed registers a callback function to be called when the down button is pressed.
func OnButtonDownPressed(callback func()) {
	hardware.OnGPIOButtonPressed(6, callback)
}

// OnButtonLeftPressed registers a callback function to be called when the left button is pressed.
func OnButtonLeftPressed(callback func()) {
	hardware.OnGPIOButtonPressed(16, callback)
}

// OnButtonRightPressed registers a callback function to be called when the right button is pressed.
func OnButtonRightPressed(callback func()) {
	hardware.OnGPIOButtonPressed(13, callback)
}

// OnButtonAPressed registers a callback function to be called when the A button is pressed.
func OnButtonAPressed(callback func()) {
	hardware.OnGPIOButtonPressed(21, callback)
}

// OnButtonBPressed registers a callback function to be called when the B button is pressed.
func OnButtonBPressed(callback func()) {
	hardware.OnGPIOButtonPressed(20, callback)
}

// OnButtonXPressed registers a callback function to be called when the X button is pressed.
func OnButtonXPressed(callback func()) {
	hardware.OnGPIOButtonPressed(15, callback)
}

// OnButtonYPressed registers a callback function to be called when the Y button is pressed.
func OnButtonYPressed(callback func()) {
	hardware.OnGPIOButtonPressed(12, callback)
}

// OnButtonTriggerLeftPressed registers a callback function to be called when the left trigger button is pressed.
func OnButtonTriggerLeftPressed(callback func()) {
	hardware.OnGPIOButtonPressed(23, callback)
}

// OnButtonTriggerRightPressed registers a callback function to be called when the right trigger button is pressed.
func OnButtonTriggerRightPressed(callback func()) {
	hardware.OnGPIOButtonPressed(14, callback)
}

// OnButtonStartPressed registers a callback function to be called when the start button is pressed.
func OnButtonStartPressed(callback func()) {
	hardware.OnGPIOButtonPressed(26, callback)
}

// OnButtonSelectPressed registers a callback function to be called when the select button is pressed.
func OnButtonSelectPressed(callback func()) {
	hardware.OnGPIOButtonPressed(19, callback)
}

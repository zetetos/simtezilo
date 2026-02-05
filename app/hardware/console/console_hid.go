package console

import (
	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
	"github.com/zetetos/simtezilo/app/ui"
)

// SetupHID initializes the Human Interface Device input listener and sends events to the provided channel.
func SetupHID(hidEvent chan ui.HIDInputEvent) {
	_ = keyboard.Listen(func(key keys.Key) (stop bool, err error) {
		switch key.Code {
		case keys.CtrlC, keys.Escape:
			hidEvent <- ui.HIDInputEscape

			return true, nil // Stop listener
		case keys.RuneKey:
			if key.String() == "q" {
				hidEvent <- ui.HIDInputEscape

				return true, nil // Stop listener
			}
		case keys.Up:
			hidEvent <- ui.HIDInputUp
		case keys.Down:
			hidEvent <- ui.HIDInputDown
		case keys.Left:
			hidEvent <- ui.HIDInputLeft
		case keys.Right:
			hidEvent <- ui.HIDInputRight
		}

		return false, nil // Continue listening
	})
}

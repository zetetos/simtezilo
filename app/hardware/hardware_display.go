package hardware

import (
	"github.com/zetetos/simtezilo/app/hardware/display"
)

// Display is the interface for controlling a hardware display device.
type Display interface { //nolint:interfacebloat // Interface is unlikely to grow much more
	Clear()
	Close()
	Wakeup()
	Sleep()
	ToggleSleep() bool
	IsAwake() bool
	IsSleeping() bool
	GetResolution() (uint16, uint16)
	GetDPI() float64
	Write(content *display.Content) error
	GetOrientation() int
	SetOrientation(degrees int)
	RotateCW() int
	RotateCCW() int
}

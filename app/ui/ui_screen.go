package ui

import "github.com/zetetos/simtezilo/app/ui/gui"

// ScreenMode provides a limited set of modes for the screen.
type ScreenMode int

const (
	ScreenModeOff ScreenMode = iota
	ScreenModeSleep
	ScreenModeWait
	ScreenModeStartup
	ScreenModeLive
	ScreenModeSettings
)

// Screen defines the interface for a screen to which a user interface can be rendered.
type Screen interface {
	SetMode(mode ScreenMode)
	GetMode() ScreenMode
	RenderSplashScreen(value string)
	RenderErrorScreen(value string)
	RenderLiveScreen(gear int)
	RenderSettingScreen(layout gui.Layout, title string, value string)
}

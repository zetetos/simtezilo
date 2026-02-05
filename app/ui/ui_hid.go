package ui

import (
	"context"
	"strconv"
	"time"

	"github.com/zetetos/simtezilo/app/exitcode"
	"github.com/zetetos/simtezilo/app/i18n/languagedb"
	"github.com/zetetos/simtezilo/app/kinematics"
	"github.com/zetetos/simtezilo/app/ui/gui"
)

const unknownMenuTitle = "???"

type HIDInputEvent int

const (
	HIDInputNone HIDInputEvent = iota
	HIDInputUp
	HIDInputDown
	HIDInputLeft
	HIDInputRight
	HIDInputPageUp
	HIDInputPageDown
	HIDInputHome
	HIDInputEnd
	HIDInputEnter
	HIDInputTab
	HIDInputEscape
	HIDInputPower
)

const (
	actionIncrease    = "increase"
	actionDecrease    = "decrease"
	actionNext        = "next"
	actionPrevious    = "previous"
	actionEnter       = "enter"
	actionExit        = "exit"
	actionGet         = "get"
	actionNone        = "none"
	menuPageSetupMode = "setupMode"
	menuPageRoot      = "root"
)

// HIDEventHandler processes HID input events and updates the UI based on the event type.
func (u *UserInterface) HIDEventHandler(ctx context.Context) {
	u.log.Debug().
		Str("component", "HID event handler").
		Msg("Start")

	ready := false

	for {
		select {
		case <-ctx.Done():
			return
		case key := <-u.hidEvents:
			if !u.shouldProcessHIDEvent(&ready) {
				continue
			}

			if u.handleSpecialKeys(key) {
				continue
			}

			title, value := u.handleMenuNavigation(key)

			// Special case: entering live view should render the live screen
			if title == string(languagedb.UIMenuLiveView) {
				u.mode = ScreenModeLive
				u.lastMenuActivity = time.Now()
				_ = u.Screen.RenderLiveScreen(kinematics.GearName(u.displayData.Gear))

				continue
			}

			layout := u.determineLayout(title)

			u.renderSettingScreen(layout, title, value)
		}
	}
}

// shouldProcessHIDEvent checks if HID events should be processed, handling startup delay.
func (u *UserInterface) shouldProcessHIDEvent(ready *bool) bool {
	if !*ready {
		if time.Since(u.startTime) < 2*time.Second {
			return false // discard hid events in the first 2 seconds after app start
		}

		*ready = true
	}

	return true
}

// handleSpecialKeys handles special keys that don't require menu updates.
func (u *UserInterface) handleSpecialKeys(key HIDInputEvent) bool {
	switch key { //nolint:exhaustive // no need to handle all keys
	case HIDInputTab:
		u.handleTabKey()

		return true
	case HIDInputEscape:
		u.handleEscapeKey()

		return true
	case HIDInputPower:
		u.handlePowerKey()

		return true
	default:
		return false
	}
}

// handleTabKey handles the tab key for screen rotation.
func (u *UserInterface) handleTabKey() {
	orientation := u.display.RotateCW()
	u.log.Debug().
		Str("key", "tab").
		Str("action", "screen rotate").
		Str("type", "hardware").
		Str("value", strconv.Itoa(orientation)).
		Msg("HID event")
}

// handleEscapeKey handles the escape key for quitting the application.
func (u *UserInterface) handleEscapeKey() {
	u.log.Debug().
		Str("key", "escape").
		Str("action", "quit").
		Str("type", "app").
		Msg("HID event")

	u.done <- exitcode.Success
}

// handlePowerKey handles the power key for toggling display off.
func (u *UserInterface) handlePowerKey() {
	state := u.DisplayToggleOff()
	u.log.Debug().
		Str("key", "power").
		Str("action", "toggle off").
		Str("type", "hardware").
		Bool("value", state).
		Msg("HID event")
}

// handleMenuNavigation handles menu navigation keys and returns the menu page and value.
func (u *UserInterface) handleMenuNavigation(key HIDInputEvent) (title string, value string) {
	title = string(u.menuSystem.GetCurrentMenuPage())

	switch key { //nolint:exhaustive // no need to handle all keys
	case HIDInputUp:
		return u.handleUpKey()
	case HIDInputDown:
		return u.handleDownKey()
	case HIDInputLeft:
		return u.handleLeftKey()
	case HIDInputRight:
		return u.handleRightKey()
	default:
		u.log.Debug().Msgf("HID Input: Unknown (%d)", key)

		return title, ""
	}
}

// handleUpKey handles the up key for navigating to parent node.
func (u *UserInterface) handleUpKey() (title string, value string) {
	// Reset inactivity timer on any menu interaction
	u.lastMenuActivity = time.Now()

	node, action := u.menuSystem.NavigateUp()
	menuPage := node.name

	switch action {
	case actionExit:
		u.log.Debug().
			Str("key", "up").
			Str("action", actionExit).
			Str("type", "navigate").
			Str("value", string(menuPage)).
			Msg("HID event")

		value = ""
		// If exited to a branch, don't get value
		if u.menuSystem.IsCurrentNodeBranch() {
			return string(menuPage), value
		}
	case actionIncrease:
		// On a leaf node - increase value
		value = u.SettingAction(menuPage, actionIncrease)
		u.log.Debug().
			Str("key", "up").
			Str("action", actionIncrease).
			Str("type", string(menuPage)).
			Str("value", value).
			Msg("HID event")

		return string(menuPage), value
	}

	// Get current value if we're on a leaf
	if u.menuSystem.IsCurrentNodeLeaf() {
		value = u.SettingAction(menuPage, actionGet)
	}

	return string(menuPage), value
}

// handleDownKey handles the down key for entering branches or navigating.
func (u *UserInterface) handleDownKey() (title string, value string) {
	// Reset inactivity timer on any menu interaction
	u.lastMenuActivity = time.Now()

	node, action := u.menuSystem.NavigateDown()
	menuPage := node.name

	switch action {
	case actionEnter:
		u.log.Debug().
			Str("key", "down").
			Str("action", actionEnter).
			Str("type", "navigate").
			Str("value", string(menuPage)).
			Msg("HID event")
	case actionExit:
		u.log.Debug().
			Str("key", "down").
			Str("action", actionExit).
			Str("type", "navigate").
			Str("value", string(menuPage)).
			Msg("HID event")

		value = ""
		// If exited to a branch, don't get value
		if u.menuSystem.IsCurrentNodeBranch() {
			return string(menuPage), value
		}
	case actionDecrease:
		// On a leaf node - decrease value
		value = u.SettingAction(menuPage, actionDecrease)
		u.log.Debug().
			Str("key", "down").
			Str("action", actionDecrease).
			Str("type", string(menuPage)).
			Str("value", value).
			Msg("HID event")

		return string(menuPage), value
	}

	// Reset setupMode countdown if we navigated to setupMode
	if string(menuPage) == menuPageSetupMode {
		_ = u.menuSystem.ResetSetupModeCountdown()
	}

	// Get current value if we're on a leaf
	if u.menuSystem.IsCurrentNodeLeaf() {
		value = u.SettingAction(menuPage, actionGet)
	} else {
		value = ""
	}

	return string(menuPage), value
}

// handleLeftKey handles the left key for previous sibling or decreasing values.
func (u *UserInterface) handleLeftKey() (title string, value string) {
	// Reset inactivity timer on any menu interaction
	u.lastMenuActivity = time.Now()

	node, action := u.menuSystem.NavigateLeft()
	menuPage := node.name

	if action == actionDecrease {
		// On a leaf node - decrease value
		value = u.SettingAction(menuPage, actionDecrease)
		u.log.Debug().
			Str("key", "left").
			Str("action", actionDecrease).
			Str("type", string(menuPage)).
			Str("value", value).
			Msg("HID event")
	} else {
		// Navigation to previous sibling
		u.log.Debug().
			Str("key", "left").
			Str("action", actionPrevious).
			Str("type", "navigate").
			Str("value", string(menuPage)).
			Msg("HID event")

		// Reset setupMode countdown if we navigated to setupMode
		if string(menuPage) == menuPageSetupMode {
			_ = u.menuSystem.ResetSetupModeCountdown()
		}

		if u.display.IsAwake() && u.menuSystem.IsCurrentNodeLeaf() {
			value = u.SettingAction(menuPage, actionGet)
		} else {
			value = ""
		}
	}

	return string(menuPage), value
}

// handleRightKey handles the right key for next sibling or increasing values.
func (u *UserInterface) handleRightKey() (title string, value string) {
	// Reset inactivity timer on any menu interaction
	u.lastMenuActivity = time.Now()

	node, action := u.menuSystem.NavigateRight()
	menuPage := node.name

	if action == actionIncrease {
		// On a leaf node - increase value
		value = u.SettingAction(menuPage, actionIncrease)
		u.log.Debug().
			Str("key", "right").
			Str("action", actionIncrease).
			Str("type", string(menuPage)).
			Str("value", value).
			Msg("HID event")
	} else {
		// Navigation to next sibling
		u.log.Debug().
			Str("key", "right").
			Str("action", actionNext).
			Str("type", "navigate").
			Str("value", string(menuPage)).
			Msg("HID event")

		// Reset setupMode countdown if we navigated to setupMode
		if string(menuPage) == menuPageSetupMode {
			_ = u.menuSystem.ResetSetupModeCountdown()
		}

		if u.display.IsAwake() && u.menuSystem.IsCurrentNodeLeaf() {
			value = u.SettingAction(menuPage, actionGet)
		} else {
			value = ""
		}
	}

	return string(menuPage), value
}

// determineLayout determines the appropriate layout based on the current node type.
func (u *UserInterface) determineLayout(menuPage string) gui.Layout {
	// Special info pages
	if menuPage == "version" || menuPage == "commitHash" || menuPage == "buildTime" || menuPage == "platform" || menuPage == "ipAddress" {
		return gui.LayoutInfo
	}

	// Branch nodes (including return nodes)
	if u.menuSystem.IsCurrentNodeBranch() {
		return gui.LayoutMenuSub
	}

	// Leaf nodes (settings)
	return gui.LayoutSetting
}

// renderSettingScreen renders the setting screen with the given title and value.
func (u *UserInterface) renderSettingScreen(layout gui.Layout, menuPage string, value string) {
	// Switch to settings mode to display the menu
	u.mode = ScreenModeSettings

	// Reset inactivity timer on any menu display
	u.lastMenuActivity = time.Now()

	var (
		title        string
		displayValue string
	)

	switch layout { //nolint:exhaustive // only relevant layout types are handled
	case gui.LayoutMenuSub:
		// Branch nodes: show parent at top (empty for top-level), current item in center
		parent := u.menuSystem.GetCurrentNode().parent
		if parent != nil && parent.name != menuPageRoot {
			title = u.getBranchTitle(string(parent.name))
		} else {
			title = "" // Empty for top-level branches
		}

		displayValue = u.getBranchTitle(menuPage)
		_ = u.Screen.RenderSettingScreen(layout, title, displayValue)

		return

	case gui.LayoutInfo:
		// Info pages: title at top, multi-line value in center
		title = u.getLeafTitle(menuPage)
		_ = u.Screen.RenderSettingScreen(layout, title, value)

		return

	case gui.LayoutSetting:
		// Leaf nodes: parent at top, value in center, setting name at bottom
		parent := u.menuSystem.GetCurrentNode().parent
		if parent != nil && parent.name != menuPageRoot {
			title = u.getBranchTitle(string(parent.name))
		} else {
			title = u.i18n.GetString(languagedb.UIMenuSettings)
		}

		// For "return" leaf, show parent name as title and "return" as value
		if menuPage == "return" {
			displayValue = u.i18n.GetString(languagedb.UIMenuReturn)
		} else {
			// Format as "settingName|settingValue"
			settingName := u.getSettingName(menuPage)
			displayValue = settingName + "|" + value
		}

		_ = u.Screen.RenderSettingScreen(layout, title, displayValue)

		return
	}
}

// getSettingName returns the translated name of the setting (leaf node).
func (u *UserInterface) getSettingName(menuPage string) string {
	key, err := languagedb.StringToKey(menuPage)
	if err != nil {
		return menuPage
	}

	return u.i18n.GetString(key)
}

// getBranchTitle returns the title for a branch node.
func (u *UserInterface) getBranchTitle(menuPage string) string {
	key, err := languagedb.StringToKey(menuPage)
	if err != nil {
		u.log.Error().
			Err(err).
			Str("menuPage", menuPage).
			Msg("Failed to convert menu page to translation key")

		return "???"
	}

	return u.i18n.GetString(key)
}

// getLeafTitle returns the title for a leaf node.
func (u *UserInterface) getLeafTitle(menuPage string) string {
	// Just translate the leaf name directly
	key, err := languagedb.StringToKey(menuPage)
	if err != nil {
		u.log.Error().
			Err(err).
			Str("menuPage", menuPage).
			Msg("Failed to convert menu page to translation key")

		return unknownMenuTitle
	}

	return u.i18n.GetString(key)
}

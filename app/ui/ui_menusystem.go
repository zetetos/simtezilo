package ui

import "github.com/zetetos/simtezilo/app/i18n/languagedb"

const (
	setupModeCountdownStart = 5
	menuNodeRoot            = languagedb.Key("root")
)

// PageContext defines when a menu page should be visible.
type PageContext string

const (
	PageContextAlways   PageContext = "always"
	PageContextDevTools PageContext = "devtools"
)

// NodeType defines whether a menu node is a branch or leaf.
type NodeType int

const (
	NodeTypeBranch NodeType = iota
	NodeTypeLeaf
)

// MenuNode represents a node in the hierarchical menu tree.
type MenuNode struct {
	name     languagedb.Key
	nodeType NodeType
	context  PageContext
	children []*MenuNode
	parent   *MenuNode
}

type MenuSystem struct {
	root               *MenuNode
	currentNode        *MenuNode
	setupModeCountdown int
	devToolsEnabled    func() bool
}

func NewMenuSystem() *MenuSystem {
	// Build the menu tree structure
	root := &MenuNode{
		name:     menuNodeRoot,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		children: make([]*MenuNode, 0),
	}

	// Top level menu items
	liveNode := &MenuNode{
		name:     languagedb.UIMenuLive,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   root,
		children: make([]*MenuNode, 0),
	}

	liveNode.children = append(liveNode.children,
		&MenuNode{name: languagedb.UIMenuLiveView, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: liveNode},
	)

	settingsNode := &MenuNode{
		name:     languagedb.UIMenuSettings,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   root,
		children: make([]*MenuNode, 0),
	}

	infoNode := &MenuNode{
		name:     languagedb.UIMenuInfo,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   root,
		children: make([]*MenuNode, 0),
	}

	infoNode.children = append(infoNode.children,
		&MenuNode{name: languagedb.UIMenuInfoVersion, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: infoNode},
		&MenuNode{name: languagedb.UIMenuInfoCommitHash, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: infoNode},
		&MenuNode{name: languagedb.UIMenuInfoBuildTime, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: infoNode},
		&MenuNode{name: languagedb.UIMenuInfoPlatform, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: infoNode},
		&MenuNode{name: languagedb.UIMenuInfoIPAddress, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: infoNode},
	)

	devNode := &MenuNode{
		name:     languagedb.UIMenuDevtools,
		nodeType: NodeTypeBranch,
		context:  PageContextDevTools,
		parent:   root,
		children: make([]*MenuNode, 0),
	}

	root.children = append(root.children, liveNode, settingsNode, infoNode, devNode)

	// Settings -> Application submenu
	appNode := &MenuNode{
		name:     languagedb.UIMenuApp,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   settingsNode,
		children: make([]*MenuNode, 0),
	}

	appNode.children = append(appNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: appNode},
		&MenuNode{name: languagedb.UIMenuAppLanguage, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: appNode},
		&MenuNode{name: languagedb.UIMenuAppLoglevel, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: appNode},
		&MenuNode{name: languagedb.UIMenuAppTelemetrySource, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: appNode},
		&MenuNode{name: languagedb.UIMenuAppDevtools, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: appNode},
	)

	// Settings -> System submenu
	systemNode := &MenuNode{
		name:     languagedb.UIMenuSystem,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   settingsNode,
		children: make([]*MenuNode, 0),
	}

	systemNode.children = append(systemNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: systemNode},
		&MenuNode{name: languagedb.UIMenuSystemDisplayOrientation, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: systemNode},
		&MenuNode{name: languagedb.UIMenuSystemSetupmode, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: systemNode},
	)

	// Settings -> Synthesizer submenu
	synthNode := &MenuNode{
		name:     languagedb.UIMenuSynth,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   settingsNode,
		children: make([]*MenuNode, 0),
	}

	synthNode.children = append(synthNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: synthNode},
	)

	// Synthesizer -> Sample Rates submenu
	sampleRatesNode := &MenuNode{
		name:     languagedb.UIMenuSynthSampleRates,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   synthNode,
		children: make([]*MenuNode, 0),
	}
	sampleRatesNode.children = append(sampleRatesNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: sampleRatesNode},
		&MenuNode{name: languagedb.UIMenuSynthInternalSampleRate, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: sampleRatesNode},
		&MenuNode{name: languagedb.UIMenuSynthOutputSampleRate, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: sampleRatesNode},
	)

	// Synthesizer -> Mute submenu
	muteControlsNode := &MenuNode{
		name:     languagedb.UIMenuSynthMute,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   synthNode,
		children: make([]*MenuNode, 0),
	}
	muteControlsNode.children = append(muteControlsNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: muteControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthMuteMaster, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: muteControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthMuteLeft, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: muteControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthMuteRight, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: muteControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthMuteChassis, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: muteControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthMuteEngine, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: muteControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthMuteTransmission, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: muteControlsNode},
	)

	// Synthesizer -> Gain Controls submenu
	gainControlsNode := &MenuNode{
		name:     languagedb.UIMenuSynthGainControls,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   synthNode,
		children: make([]*MenuNode, 0),
	}

	// Gain Controls -> Calibration submenu
	calibrationNode := &MenuNode{
		name:     languagedb.UIMenuSynthCalibration,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   gainControlsNode,
		children: make([]*MenuNode, 0),
	}
	calibrationNode.children = append(calibrationNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: calibrationNode},
		&MenuNode{name: languagedb.UIMenuSynthCalibrationEnable, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: calibrationNode},
		&MenuNode{name: languagedb.UIMenuSynthCalibrationChannel, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: calibrationNode},
		&MenuNode{name: languagedb.UIMenuSynthCalibrationFrequency, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: calibrationNode},
		&MenuNode{name: languagedb.UIMenuSynthCalibrationSweep, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: calibrationNode},
		&MenuNode{name: languagedb.UIMenuSynthCalibrationSweepRange, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: calibrationNode},
	)

	gainControlsNode.children = append(gainControlsNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: gainControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthMasterGain, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: gainControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthLeftGain, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: gainControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthRightGain, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: gainControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthChassisGain, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: gainControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthEngineGain, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: gainControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthTransmissionGain, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: gainControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthTransmissionGainMinRace, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: gainControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthTransmissionGainMinStreet, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: gainControlsNode},
		&MenuNode{name: languagedb.UIMenuSynthEqMode, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: gainControlsNode},
		calibrationNode,
	)

	synthNode.children = append(synthNode.children, sampleRatesNode, muteControlsNode, gainControlsNode)

	// Settings -> Haptics submenu
	hapticsNode := &MenuNode{
		name:     languagedb.UIMenuHaptics,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   settingsNode,
		children: make([]*MenuNode, 0),
	}

	hapticsNode.children = append(hapticsNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: hapticsNode},
	)

	// Haptics -> Output Mode leaf
	hapticsNode.children = append(hapticsNode.children,
		&MenuNode{name: languagedb.UIMenuHapticsOutputMode, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: hapticsNode},
	)

	// Haptics -> Chassis Feedback submenu
	chassisFeedbackNode := &MenuNode{
		name:     languagedb.UIMenuHapticsChassis,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   hapticsNode,
		children: make([]*MenuNode, 0),
	}
	chassisFeedbackNode.children = append(chassisFeedbackNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: chassisFeedbackNode},
		&MenuNode{name: languagedb.UIMenuHapticsJerkCurve, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: chassisFeedbackNode},
		&MenuNode{name: languagedb.UIMenuHapticsJerkMax, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: chassisFeedbackNode},
		&MenuNode{name: languagedb.UIMenuHapticsSnapCurve, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: chassisFeedbackNode},
		&MenuNode{name: languagedb.UIMenuHapticsSnapMax, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: chassisFeedbackNode},
		&MenuNode{name: languagedb.UIMenuHapticsPulseMaxAmplitude, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: chassisFeedbackNode},
		&MenuNode{name: languagedb.UIMenuHapticsPulseMinFreq, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: chassisFeedbackNode},
		&MenuNode{name: languagedb.UIMenuHapticsPulseMaxFreq, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: chassisFeedbackNode},
	)

	// Haptics -> Transmission Feedback submenu
	transmissionFeedbackNode := &MenuNode{
		name:     languagedb.UIMenuHapticsTransmission,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   hapticsNode,
		children: make([]*MenuNode, 0),
	}
	transmissionFeedbackNode.children = append(transmissionFeedbackNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: transmissionFeedbackNode},
		&MenuNode{name: languagedb.UIMenuHapticsTransmissionFFBStrength, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: transmissionFeedbackNode},
		&MenuNode{name: languagedb.UIMenuHapticsTransmissionCurve, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: transmissionFeedbackNode},
		&MenuNode{name: languagedb.UIMenuHapticsTransmissionGforceMax, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: transmissionFeedbackNode},
	)

	// Engine Profiles -> Engine Profile submenu (nested)
	engineProfileNode := &MenuNode{
		name:     languagedb.UIMenuHapticsEngineProfile,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   hapticsNode,
		children: make([]*MenuNode, 0),
	}
	engineProfileNode.children = append(engineProfileNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: engineProfileNode},
		&MenuNode{name: languagedb.UIMenuHapticsEnginePrimaryBalance, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: engineProfileNode},
		&MenuNode{name: languagedb.UIMenuHapticsEngineSecondaryBalance, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: engineProfileNode},
		&MenuNode{name: languagedb.UIMenuHapticsEnginePulseGain, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: engineProfileNode},
		&MenuNode{name: languagedb.UIMenuHapticsEnginePulseScale, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: engineProfileNode},
	)

	hapticsNode.children = append(hapticsNode.children, chassisFeedbackNode, transmissionFeedbackNode, engineProfileNode)

	// Settings -> Pit Radio submenu
	pitRadioNode := &MenuNode{
		name:     languagedb.UIMenuPitRadio,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   settingsNode,
		children: make([]*MenuNode, 0),
	}

	// Settings -> Pit Radio -> Notifications submenu
	pitRadioNotificationsNode := &MenuNode{
		name:     languagedb.UIMenuPitRadioNotifications,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   pitRadioNode,
		children: make([]*MenuNode, 0),
	}

	// Settings -> Pit Radio -> Notifications -> Lap Times submenu
	pitRadioLapTimesNode := &MenuNode{
		name:     languagedb.UIMenuPitRadioLapTimes,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   pitRadioNotificationsNode,
		children: make([]*MenuNode, 0),
	}

	pitRadioLapTimesNode.children = append(pitRadioLapTimesNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: pitRadioLapTimesNode},
		&MenuNode{name: languagedb.UIMenuPitRadioLapTimesEnable, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioLapTimesNode},
		&MenuNode{name: languagedb.UIMenuPitRadioLapTimesMaxDelta, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioLapTimesNode},
	)

	// Settings -> Pit Radio -> Notifications -> Race Laps submenu
	pitRadioRaceLapsNode := &MenuNode{
		name:     languagedb.UIMenuPitRadioRaceLaps,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   pitRadioNotificationsNode,
		children: make([]*MenuNode, 0),
	}

	pitRadioRaceLapsNode.children = append(pitRadioRaceLapsNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: pitRadioRaceLapsNode},
		&MenuNode{name: languagedb.UIMenuPitRadioRaceLapsEnable, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioRaceLapsNode},
		&MenuNode{name: languagedb.UIMenuPitRadioRaceLapsCountdown, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioRaceLapsNode},
		&MenuNode{name: languagedb.UIMenuPitRadioRaceLapsInterval, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioRaceLapsNode},
	)

	// Settings -> Pit Radio -> Notifications -> Race Progress submenu
	pitRadioRaceProgressNode := &MenuNode{
		name:     languagedb.UIMenuPitRadioRaceProgress,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   pitRadioNotificationsNode,
		children: make([]*MenuNode, 0),
	}

	pitRadioRaceProgressNode.children = append(pitRadioRaceProgressNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: pitRadioRaceProgressNode},
		&MenuNode{name: languagedb.UIMenuPitRadioRaceProgressEnable, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioRaceProgressNode},
		&MenuNode{name: languagedb.UIMenuPitRadioRaceProgressMinLaps, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioRaceProgressNode},
		&MenuNode{name: languagedb.UIMenuPitRadioRaceProgressInterval, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioRaceProgressNode},
	)

	pitRadioNotificationsNode.children = append(pitRadioNotificationsNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: pitRadioNotificationsNode},
		pitRadioLapTimesNode,
		pitRadioRaceLapsNode,
		pitRadioRaceProgressNode,
	)

	// Settings -> Pit Radio -> Fuel Management submenu
	pitRadioFuelNode := &MenuNode{
		name:     languagedb.UIMenuPitRadioFuel,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   pitRadioNode,
		children: make([]*MenuNode, 0),
	}

	pitRadioFuelNode.children = append(pitRadioFuelNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: pitRadioFuelNode},
		&MenuNode{name: languagedb.UIMenuPitRadioFuelEnable, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioFuelNode},
		&MenuNode{name: languagedb.UIMenuPitRadioFuelPreWarn, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioFuelNode},
		&MenuNode{name: languagedb.UIMenuPitRadioFuelStrategy, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioFuelNode},
		&MenuNode{name: languagedb.UIMenuPitRadioFuelSafetyLaps, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioFuelNode},
		&MenuNode{name: languagedb.UIMenuPitRadioFuelSafetyMetres, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioFuelNode},
	)

	// Settings -> Pit Radio -> Tyre Management submenu
	pitRadioTyreNode := &MenuNode{
		name:     languagedb.UIMenuPitRadioTyre,
		nodeType: NodeTypeBranch,
		context:  PageContextAlways,
		parent:   pitRadioNode,
		children: make([]*MenuNode, 0),
	}

	pitRadioTyreNode.children = append(pitRadioTyreNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: pitRadioTyreNode},
		&MenuNode{name: languagedb.UIMenuPitRadioTyreEnable, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioTyreNode},
		&MenuNode{name: languagedb.UIMenuPitRadioTyreTempOptimal, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioTyreNode},
		&MenuNode{name: languagedb.UIMenuPitRadioTyreTempWindow, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioTyreNode},
		&MenuNode{name: languagedb.UIMenuPitRadioTyreTempMargin, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioTyreNode},
	)

	pitRadioNode.children = append(pitRadioNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: pitRadioNode},
		&MenuNode{name: languagedb.UIMenuPitRadioEnable, nodeType: NodeTypeLeaf, context: PageContextAlways, parent: pitRadioNode},
		pitRadioNotificationsNode,
		pitRadioFuelNode,
		pitRadioTyreNode,
	)

	settingsNode.children = append(settingsNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextAlways, parent: settingsNode},
		appNode, systemNode, synthNode, hapticsNode, pitRadioNode,
	)

	// Dev submenu
	devNode.children = append(devNode.children,
		&MenuNode{name: languagedb.UIMenuReturn, nodeType: NodeTypeBranch, context: PageContextDevTools, parent: devNode},
		&MenuNode{name: languagedb.UIMenuDevtoolsRecord, nodeType: NodeTypeLeaf, context: PageContextDevTools, parent: devNode},
	)

	menuSystem := &MenuSystem{
		root:               root,
		currentNode:        liveNode, // Start at Live (first top-level item)
		setupModeCountdown: setupModeCountdownStart,
	}

	return menuSystem
}

// NavigateLeft moves to the previous sibling or is used for value adjustment on leaves.
func (m *MenuSystem) NavigateLeft() (*MenuNode, string) {
	// All nodes navigate left to previous sibling
	return m.previousSibling(), "navigate"
}

// NavigateRight moves to the next sibling.
func (m *MenuSystem) NavigateRight() (*MenuNode, string) {
	// All nodes navigate right to next sibling
	return m.nextSibling(), "navigate"
}

// NavigateDown enters a branch node or toggles adjust mode on leaf nodes.
func (m *MenuSystem) NavigateDown() (*MenuNode, string) {
	// Special case: on "return" node, navigate up to parent (same as up button)
	if m.currentNode.name == languagedb.UIMenuReturn {
		if m.currentNode.parent != nil && m.currentNode.parent.name != menuNodeRoot {
			m.currentNode = m.currentNode.parent

			return m.currentNode, actionExit
		}
	}

	if m.currentNode.nodeType == NodeTypeBranch {
		// Enter the branch (move to first visible child)
		if len(m.currentNode.children) > 0 {
			firstVisible := m.getFirstVisibleChild(m.currentNode)
			if firstVisible != nil {
				m.currentNode = firstVisible

				return m.currentNode, actionEnter
			}
		}
	}

	// For leaves, decrease value (except info and live nodes which return to parent)
	if m.currentNode.nodeType == NodeTypeLeaf {
		if m.currentNode.parent != nil && m.currentNode.parent.name == languagedb.UIMenuInfo {
			m.currentNode = m.currentNode.parent

			return m.currentNode, actionExit
		}

		if m.currentNode.parent != nil && m.currentNode.parent.name == languagedb.UIMenuLive {
			m.currentNode = m.currentNode.parent

			return m.currentNode, actionExit
		}

		return m.currentNode, actionDecrease
	}

	return m.currentNode, actionNone
}

// NavigateUp exits to parent on return node or increases value on regular leaves.
func (m *MenuSystem) NavigateUp() (*MenuNode, string) {
	// Special case: on "return" node, navigate up to parent
	if m.currentNode.name == languagedb.UIMenuReturn {
		if m.currentNode.parent != nil && m.currentNode.parent.name != menuNodeRoot {
			m.currentNode = m.currentNode.parent

			return m.currentNode, actionExit
		}
	}

	// Special case: on Live branch, enter the live view (same as down)
	if m.currentNode.name == languagedb.UIMenuLive && m.currentNode.nodeType == NodeTypeBranch {
		if len(m.currentNode.children) > 0 {
			firstVisible := m.getFirstVisibleChild(m.currentNode)
			if firstVisible != nil {
				m.currentNode = firstVisible

				return m.currentNode, actionEnter
			}
		}
	}

	// For leaves, increase value (except info and live nodes which return to parent)
	if m.currentNode.nodeType == NodeTypeLeaf {
		if m.currentNode.parent != nil && m.currentNode.parent.name == languagedb.UIMenuInfo {
			m.currentNode = m.currentNode.parent

			return m.currentNode, actionExit
		}

		if m.currentNode.parent != nil && m.currentNode.parent.name == languagedb.UIMenuLive {
			m.currentNode = m.currentNode.parent

			return m.currentNode, actionExit
		}

		return m.currentNode, actionIncrease
	}

	// For branches, do nothing
	return m.currentNode, actionNone
}

// GetCurrentNode returns the current menu node.
func (m *MenuSystem) GetCurrentNode() *MenuNode {
	return m.currentNode
}

// GetCurrentMenuPage returns the name of the current menu node (for compatibility).
func (m *MenuSystem) GetCurrentMenuPage() languagedb.Key {
	return m.currentNode.name
}

// GetBreadcrumb returns the navigation path from root to current node.
func (m *MenuSystem) GetBreadcrumb() []string {
	path := make([]string, 0)
	node := m.currentNode

	for node != nil && node.name != menuNodeRoot {
		path = append([]string{string(node.name)}, path...)
		node = node.parent
	}

	return path
}

// IsCurrentNodeLeaf returns true if the current node is a leaf.
func (m *MenuSystem) IsCurrentNodeLeaf() bool {
	return m.currentNode.nodeType == NodeTypeLeaf
}

// IsCurrentNodeBranch returns true if the current node is a branch.
func (m *MenuSystem) IsCurrentNodeBranch() bool {
	return m.currentNode.nodeType == NodeTypeBranch
}

// NextMenuPage navigates right (for compatibility with existing code).
func (m *MenuSystem) NextMenuPage() languagedb.Key {
	node, _ := m.NavigateRight()

	return node.name
}

// PreviousMenuPage navigates left (for compatibility with existing code).
func (m *MenuSystem) PreviousMenuPage() languagedb.Key {
	node, _ := m.NavigateLeft()

	return node.name
}

// GetSetupModeCountdown returns the current setup countdown value.
func (m *MenuSystem) GetSetupModeCountdown() int {
	return m.setupModeCountdown
}

// ResetSetupModeCountdown resets the setup countdown to 5.
func (m *MenuSystem) ResetSetupModeCountdown() int {
	m.setupModeCountdown = setupModeCountdownStart

	return m.setupModeCountdown
}

// DecrementSetupModeCountdown decrements the setup countdown by 1.
func (m *MenuSystem) DecrementSetupModeCountdown() int {
	if m.setupModeCountdown > 0 {
		m.setupModeCountdown--
	}

	return m.setupModeCountdown
}

// IsSetupModeCountdownZero returns true if the setup countdown is zero.
func (m *MenuSystem) IsSetupModeCountdownZero() bool {
	return m.setupModeCountdown == 0
}

// SetDevToolsEnabledCallback sets the callback function to check if dev tools are enabled.
func (m *MenuSystem) SetDevToolsEnabledCallback(callback func() bool) {
	m.devToolsEnabled = callback
}

// previousSibling navigates to the previous visible sibling with wrapping.
func (m *MenuSystem) previousSibling() *MenuNode {
	if m.currentNode.parent == nil {
		return m.currentNode
	}

	siblings := m.getVisibleChildren(m.currentNode.parent)
	if len(siblings) <= 1 {
		return m.currentNode
	}

	// Find current index
	currentIndex := -1

	for i, sibling := range siblings {
		if sibling == m.currentNode {
			currentIndex = i

			break
		}
	}

	if currentIndex == -1 {
		return m.currentNode
	}

	// Move to previous with wrapping
	prevIndex := (currentIndex - 1 + len(siblings)) % len(siblings)
	m.currentNode = siblings[prevIndex]

	return m.currentNode
}

// nextSibling navigates to the next visible sibling with wrapping.
func (m *MenuSystem) nextSibling() *MenuNode {
	if m.currentNode.parent == nil {
		return m.currentNode
	}

	siblings := m.getVisibleChildren(m.currentNode.parent)
	if len(siblings) <= 1 {
		return m.currentNode
	}

	// Find current index
	currentIndex := -1

	for i, sibling := range siblings {
		if sibling == m.currentNode {
			currentIndex = i

			break
		}
	}

	if currentIndex == -1 {
		return m.currentNode
	}

	// Move to next with wrapping
	nextIndex := (currentIndex + 1) % len(siblings)
	m.currentNode = siblings[nextIndex]

	return m.currentNode
}

// getVisibleChildren returns only the children that should be visible based on context.
func (m *MenuSystem) getVisibleChildren(node *MenuNode) []*MenuNode {
	visible := make([]*MenuNode, 0)

	for _, child := range node.children {
		if m.isNodeVisible(child) {
			visible = append(visible, child)
		}
	}

	return visible
}

// getFirstVisibleChild returns the first visible child of a node.
func (m *MenuSystem) getFirstVisibleChild(node *MenuNode) *MenuNode {
	for _, child := range node.children {
		if m.isNodeVisible(child) {
			return child
		}
	}

	return nil
}

// isNodeVisible checks if a node should be visible based on its context.
func (m *MenuSystem) isNodeVisible(node *MenuNode) bool {
	switch node.context {
	case PageContextAlways:
		return true
	case PageContextDevTools:
		return m.isDevToolsEnabled()
	default:
		return true
	}
}

// isDevToolsEnabled returns true if dev tools are enabled.
func (m *MenuSystem) isDevToolsEnabled() bool {
	if m.devToolsEnabled == nil {
		return false
	}

	return m.devToolsEnabled()
}

package st7789

// Rotation represents the display rotation in 90 degree intervals.
type Rotation uint8

const (
	// RotationNone indicates no rotation (0 degrees).
	RotationNone Rotation = iota

	// Rotation90 indicates a 90-degree rotation.
	Rotation90

	// Rotation180 indicates a 180-degree rotation.
	Rotation180

	// Rotation270 indicates a 270-degree rotation.
	Rotation270
)

const (
	// REGISTERS.

	// ***********************************************************************.
	// System function comand table 1.
	// ***********************************************************************.

	// Nop command performs no operation.
	Nop = 0x00

	// SwReset command performs a software reset, all registers are set to default values.
	SwReset = 0x01

	// Rddid reads the display identification information.
	Rddid = 0x04

	// Rddst reads the display status.
	Rddst = 0x09

	// Rddpm reads the display power mode.
	Rddpm = 0x0A

	// RddMadCtl reads the display MADCTL.
	RddMadCtl = 0x0B

	// RddColMod reads the display pixel format.
	RddColMod = 0x0C

	// DddIM reads the display image mode.
	DddIM = 0x0D

	// RddSM reads the display signal mode.
	RddSM = 0x0E

	// RdSdr reads the display self-diagnostic result.
	RdSdr = 0x0F

	// SlpIn enters sleep mode (display off, but memory contents preserved).
	SlpIn = 0x10

	// SlpOut exits sleep mode.
	SlpOut = 0x11

	// PtlOn enables partial display mode.
	PtlOn = 0x12

	// NorOn enables normal display mode (full display area used).
	NorOn = 0x13

	// InvOff disables display inversion (normal colors).
	InvOff = 0x20

	// InvOn enables display inversion (inverted colors).
	InvOn = 0x21

	// GmSet selects gamma curve.
	GmSet = 0x26

	// DispOff turns off the display (display blanked).
	DispOff = 0x28

	// DispOn turns on the display.
	DispOn = 0x29

	// CaSet sets column address range for memory write/read.
	CaSet = 0x2A

	// RaSet sets row address range for memory write/read.
	RaSet = 0x2B

	// Ramwr starts memory write operation.
	Ramwr = 0x2C

	// Ramrd starts memory read operation.
	Ramrd = 0x2E

	// PtlAr sets partial display area.
	PtlAr = 0x30

	// VScrDef defines vertical scrolling area.
	VScrDef = 0x33

	// TeOff disables tearing effect output signal.
	TeOff = 0x34

	// TeOn enables tearing effect output signal.
	TeOn = 0x35

	// MadCtl controls memory data access direction and display orientation.
	MadCtl = 0x36

	// VscsAD sets vertical scroll start address.
	VscsAD = 0x37

	// IdmOff disables idle mode (full color depth).
	IdmOff = 0x38

	// IdmOn enables idle mode (reduced color depth to reduce power consumption).
	IdmOn = 0x39

	// ColMod sets interface pixel format (color depth).
	ColMod = 0x3A

	// WrMemC continues memory write operation.
	WrMemC = 0x3C

	// RdMemC continues memory read operation.
	RdMemC = 0x3E

	// Ste sets tear scanline number.
	Ste = 0x44

	// GScan reads current scanline number.
	GScan = 0x45

	// Wrdisbv writes display brightness value for backlight control.
	Wrdisbv = 0x51

	// Rddisbv reads display brightness value.
	Rddisbv = 0x52

	// WrtCtrlD writes display control register.
	WrtCtrlD = 0x53

	// RdCtrlD reads display control register.
	RdCtrlD = 0x54

	// WrcAce writes content adaptive brightness control and color enhancement.
	WrcAce = 0x55

	// RdcAbc reads content adaptive brightness control.
	RdcAbc = 0x56

	// WrcAbcMb writes CABC minimum brightness.
	WrcAbcMb = 0x5E

	// RdcAbcMb reads CABC minimum brightness.
	RdcAbcMb = 0x5F

	// RdAbcSdr reads automatic brightness control self-diagnostic result.
	RdAbcSdr = 0x68

	// Rdid1 reads ID1 - manufacturer ID.
	Rdid1 = 0xDA

	// Rdid2 reads ID2 - module/driver version ID.
	Rdid2 = 0xDB

	// Rdid3 reads ID3 - module/driver ID.
	Rdid3 = 0xDC

	// ***********************************************************************.
	// System function comand table 2.
	// ***********************************************************************.

	// RAMCtrl controls RAM access timing and interface settings.
	RAMCtrl = 0xB0

	// RgbCtrl controls RGB interface signal timing.
	RgbCtrl = 0xB1

	// PorCtrl controls porch setting for normal and idle mode.
	PorCtrl = 0xB2

	// FrCtrl1 controls frame rate in partial mode and idle colors.
	FrCtrl1 = 0xB3

	// ParCtrl controls partial mode display inversion.
	ParCtrl = 0xB5

	// GCtrl controls gate driver timing.
	GCtrl = 0xB7

	// GtAdj controls gate timing adjustment.
	GtAdj = 0xB8

	// DgMem controls digital gamma lookup table for red.
	DgMem = 0xBA

	// VComs controls VCOM setting.
	VComs = 0xBB

	// LcmCtrl controls LCM (LCD Module) settings.
	LcmCtrl = 0xC0

	// IDSet sets module ID.
	IDSet = 0xC1

	// VdvvRhen enables VDV and VRH command write.
	VdvvRhen = 0xC2

	// Vrhs controls VRH (GVDD/GVCL) setting.
	Vrhs = 0xC3

	// Vdvs controls VDV setting.
	Vdvs = 0xC4

	// VcmOfset controls VCOM offset setting.
	VcmOfset = 0xC5

	// FrCtrl2 controls frame rate for normal mode.
	FrCtrl2 = 0xC6

	// CabcCtrl1 controls CABC (Content Adaptive Brightness Control).
	CabcCtrl1 = 0xC7

	// RegSel1 controls register bank selection 1.
	RegSel1 = 0xC8

	// RegSel2 controls register bank selection 2.
	RegSel2 = 0xCA

	// PwmfrSel controls PWM frequency selection.
	PwmfrSel = 0xCC

	// PwCtrl1 controls power control settings 1.
	PwCtrl1 = 0xD0

	// VapVanEn enables VAP/VAN control.
	VapVanEn = 0xD2

	// Cmd2En enables command set 2.
	Cmd2En = 0xDF

	// PVGAMCtrl controls positive voltage gamma correction.
	PVGAMCtrl = 0xE0

	// NVGAMCtrl controls negative voltage gamma correction.
	NVGAMCtrl = 0xE1

	// DgnLutR controls digital gamma lookup table for red.
	DgnLutR = 0xE2

	// DgmLutB controls digital gamma lookup table for blue.
	DgmLutB = 0xE3

	// GateCtrl controls gate driver timing control.
	GateCtrl = 0xE4

	// Spi2En enables 2-data lane SPI interface.
	Spi2En = 0xE7

	// PwCtrl2 controls power control settings 2.
	PwCtrl2 = 0xE8

	// EqCtrl controls equalize control.
	EqCtrl = 0xE9

	// PromCtrl controls program mode control.
	PromCtrl = 0xEC

	// PromEn enables program mode.
	PromEn = 0xFA

	// NvmSet controls NVM (Non-Volatile Memory) setting.
	NvmSet = 0xFC

	// PromAct activates program mode.
	PromAct = 0xFE

	// ***********************************************************************.
	// COMMAND PARAMETERS.
	// ***********************************************************************.

	// BgSpiCsBack selects back SPI chip select polarity.
	BgSpiCsBack = 0

	// BgSpiCsFront selects front SPI chip select polarity.
	BgSpiCsFront = 1

	// VDVVRHENCmdNNVM sets VDV and VRH register value to come from non-volatile memory.
	VDVVRHENCmdNNVM = 0x00

	// VDVVRHENCmdENWrite sets VDV and VRH register value to come from command write.
	VDVVRHENCmdENWrite = 0x01

	// ColModRGB65K sets 56K RGB interface color format.
	ColModRGB65K = 0x50

	// ColModRGB262K sets 262K RGB interface color format.
	ColModRGB262K = 0x60

	// ColModCtrl4K sets 12-bit per pixel color mode.
	ColModCtrl4K = 0x03

	// ColModCtrl65K sets 16-bit per pixel color mode.
	ColModCtrl65K = 0x05

	// ColModCtrl262K sets 18-bit per pixel color mode.
	ColModCtrl262K = 0x06

	// ColModCtrl16M sets 24-bit per pixel color mode (truncated to 18 bits).
	ColModCtrl16M = 0x07

	// FrameRate119 sets 119Hz frame rate via FRCTRL2.
	FrameRate119 = 0x00

	// FrameRate111 sets 111Hz frame rate via FRCTRL2.
	FrameRate111 = 0x01

	// FrameRate105 sets 105Hz frame rate via FRCTRL2.
	FrameRate105 = 0x02

	// FrameRate99 sets 99Hz frame rate via FRCTRL2.
	FrameRate99 = 0x03

	// FrameRate94 sets 94Hz frame rate via FRCTRL2.
	FrameRate94 = 0x04

	// FrameRate90 sets 90Hz frame rate via FRCTRL2.
	FrameRate90 = 0x05

	// FrameRate86 sets 86Hz frame rate via FRCTRL2.
	FrameRate86 = 0x06

	// FrameRate82 sets 82Hz frame rate via FRCTRL2.
	FrameRate82 = 0x07

	// FrameRate78 sets 78Hz frame rate via FRCTRL2.
	FrameRate78 = 0x08

	// FrameRate75 sets 75Hz frame rate via FRCTRL2.
	FrameRate75 = 0x09

	// FrameRate72 sets 72Hz frame rate via FRCTRL2.
	FrameRate72 = 0x0A

	// FrameRate69 sets 69Hz frame rate via FRCTRL2.
	FrameRate69 = 0x0B

	// FrameRate67 sets 67Hz frame rate via FRCTRL2.
	FrameRate67 = 0x0C

	// FrameRate64 sets 64Hz frame rate via FRCTRL2.
	FrameRate64 = 0x0D

	// FrameRate62 sets 62Hz frame rate via FRCTRL2.
	FrameRate62 = 0x0E

	// FrameRate60 sets 60Hz frame rate via FRCTRL2.
	FrameRate60 = 0x0F

	// FrameRate58 sets 58Hz frame rate via FRCTRL2.
	FrameRate58 = 0x10

	// FrameRate57 sets 57Hz frame rate via FRCTRL2.
	FrameRate57 = 0x11

	// FrameRate55 sets 55Hz frame rate via FRCTRL2.
	FrameRate55 = 0x12

	// FrameRate53 sets 53Hz frame rate via FRCTRL2.
	FrameRate53 = 0x13

	// FrameRate52 sets 52Hz frame rate via FRCTRL2.
	FrameRate52 = 0x14

	// FrameRate50 sets 50Hz frame rate via FRCTRL2.
	FrameRate50 = 0x15

	// FrameRate49 sets 49Hz frame rate via FRCTRL2.
	FrameRate49 = 0x16

	// FrameRate48 sets 48Hz frame rate via FRCTRL2.
	FrameRate48 = 0x17

	// FrameRate46 sets 46Hz frame rate via FRCTRL2.
	FrameRate46 = 0x18

	// FrameRate45 sets 45Hz frame rate via FRCTRL2.
	FrameRate45 = 0x19

	// FrameRate44 sets 44Hz frame rate via FRCTRL2.
	FrameRate44 = 0x1A

	// FrameRate43 sets 43Hz frame rate via FRCTRL2.
	FrameRate43 = 0x1B

	// FrameRate42 sets 42Hz frame rate via FRCTRL2.
	FrameRate42 = 0x1C

	// FrameRate41 sets 41Hz frame rate via FRCTRL2.
	FrameRate41 = 0x1D

	// FrameRate40 sets 40Hz frame rate via FRCTRL2.
	FrameRate40 = 0x1E

	// FrameRate39 sets 39Hz frame rate via FRCTRL2.
	FrameRate39 = 0x1F

	// LCMCtrlXMY sets XOR MY setting in MADCTL.
	LCMCtrlXMY = 0x40

	// LCMCtrlXBGR sets XOR RGB/BGR setting in MADCTL.
	LCMCtrlXBGR = 0x20

	// LCMCtrlXREV sets XOR inverse setting in INVON.
	LCMCtrlXREV = 0x10

	// LCMCtrlXMH sets reverse source output order and only support RGB interface without RAM mode.
	LCMCtrlXMH = 0x08

	// LCMCtrlXMV sets XOR MV setting in MADCTL.
	LCMCtrlXMV = 0x04

	// LCMCtrlXMX sets XOR MX setting in MADCTL.
	LCMCtrlXMX = 0x02

	// LCMCtrlXGS sets XOR GS setting in GATECTRL.
	LCMCtrlXGS = 0x01

	// MadCtrlMyTB sets page address order top to bottom.
	MadCtrlMyTB = 0x00

	// MadCtrlMyBT sets page address order bottom to top.
	MadCtrlMyBT = 0x80

	// MadCtrlMxLR sets column address order left to right.
	MadCtrlMxLR = 0x00

	// MadCtrlMxRL sets column address order right to left.
	MadCtrlMxRL = 0x40

	// MadCtrlMvNorm sets page/column order normal.
	MadCtrlMvNorm = 0x00

	// MadCtrlMvRev sets page/column order reverse.
	MadCtrlMvRev = 0x20

	// MadCtrlMlTB sets Line address order LCD refresh top to bottom.
	MadCtrlMlTB = 0x00

	// MadCtrlMlBT sets Line address order LCD refresh bottom to top.
	MadCtrlMlBT = 0x10

	// MadCtrlRGB sets RGB color order.
	MadCtrlRGB = 0x00

	// MadCtrlBGR sets BGR color order.
	MadCtrlBGR = 0x08

	// MadCtrlMhLR sets display latch order LCD refresh left to right.
	MadCtrlMhLR = 0x00

	// MadCtrlMhRL sets display latch order LCD refresh right to left.
	MadCtrlMhRL = 0x04

	// MaxVsyncScanLines is the maximum number of scan lines for vertical sync.
	MaxVsyncScanLines = 254

	// SpiClockHZ is the default SPI clock frequency in Hz.
	SpiClockHZ = 16000000
)

// DefaultCOLMOD returns the default color mode settings specified by the manufacturer.
//
// COLMOD: 18-bit color, non-RGB.
func DefaultCOLMOD() []byte {
	return []byte{ColModCtrl262K}
}

// DefaultFRCTL1 returns the default partial mode/idle colors frame rate settings specified by the manufacturer.
//
//	FRCTRL1: {
//		FRSEN = disabled, DIV = 1,
//		Idle Inversion(Dot = true, Column = false),
//		Partial Mode Inversion(Dot = true, Column = false),
//		RTNB = 60Hz, RTNC = 60Hz
//	}
func DefaultFRCTL1() []byte {
	return []byte{0x00, 0x0F, 0x0F}
}

// DefaultFRCTRL2 returns the default normal frame rate setting specified by the manufacturer.
//
// FRCTRL2: 60Hz.
func DefaultFRCTRL2() []byte {
	return []byte{FrameRate60}
}

// DefaultGCTRL returns the default gate control settings specified by the manufacturer.
//
// GCTRL: High = 13.26v, Low = -10.43v.
func DefaultGCTRL() []byte {
	return []byte{0x35}
}

// DefaultLCMCTRL returns the default LCM control settings specified by the manufacturer.
//
// LCMCTRL: XOR RGB/BGR, XOR Column Address Order, XOR Display Data Latch Order.
func DefaultLCMCTRL() []byte {
	return []byte{0x2C}
}

// DefaultNVGAMCTRL returns the default negative voltage gamma control settings specified by the manufacturer.
func DefaultNVGAMCTRL() []byte {
	return []byte{0x70, 0x2C, 0x2E, 0x15, 0x10, 0x09, 0x48, 0x33, 0x53, 0x0B, 0x19, 0x18, 0x20, 0x25}
}

// DefaultPORCTRL returns the default porch control settings specified by the maufacturer.
//
// PORCTRL: Normal(Back Front), PSEN = disabled, Idle(Back, Front).
func DefaultPORCTRL() []byte {
	return []byte{0x0C, 0x0C, 0x00, 0x33, 0x33}
}

// DefaultPVGAMCTRL returns the default positive voltage gamma control settings specified by the manufacturer.
func DefaultPVGAMCTRL() []byte {
	return []byte{0x70, 0x2C, 0x2E, 0x15, 0x10, 0x09, 0x48, 0x33, 0x53, 0x0B, 0x19, 0x18, 0x20, 0x25}
}

// DefaultPWCTRL1 returns the default power control 1 settings specified by the manufacturer.
//
// PWCTRL1: AVDD = 6.8v, AVCL = -4.6v, VDS = 2.3v.
func DefaultPWCTRL1() []byte {
	return []byte{0xA4, 0xA1}
}

// DefaultVCOMS returns the default VCOM settings specified by the manufacturer.
//
// VCOMS: 0.9v.
func DefaultVCOMS() []byte {
	return []byte{0x20}
}

// DefaultVDVVRHEN returns the default VDV and VRH setting specified by the manufacturer.
//
// VDVVRHEN: CMDEN = VDV and VRH register value comes from command write.
func DefaultVDVVRHEN() []byte {
	return []byte{VDVVRHENCmdENWrite}
}

// DefaultVDVS returns the default VDV setting specified by the manufacturer.
//
// VDV: 0v.
func DefaultVDVS() []byte {
	return []byte{0x20}
}

// DefaultVRHS returns the default VAP(GVDD) and VAN(GVCL) settings specified by the manufacturer.
//
//	VRHS: {
//		VAP(GVDD) =  4.1v + (vcom+vcom offset-vdv)
//		VAN(GVCL) = -4.1v + (vcom+vcom offset-vdv)
//	}
func DefaultVRHS() []byte {
	return []byte{0x0B}
}

// verticalScrollOffset returns the parameters for the VSCAD register to set.
func verticalScrollOffset(offset int) []byte {
	safeOffset := min(max(offset, 0), MaxVsyncScanLines)

	return []byte{0x00, uint8(safeOffset)} //nolint:gosec // offset is limited to max scan lines
}

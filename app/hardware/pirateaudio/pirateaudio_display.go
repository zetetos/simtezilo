// Package pirateaudio implements support for the Pirate Audio hardware from Pimoroni.
package pirateaudio

import (
	"time"

	"github.com/zetetos/simtezilo/app/hardware/display"
	"github.com/zetetos/simtezilo/app/hardware/display/st7789"
	"github.com/zetetos/simtezilo/app/i18n"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
)

const (
	dataCommPin             = "GPIO9"
	resetPin                = ""
	backlightPin            = "GPIO13"
	lcdPixelRows    uint16  = 240
	lcdPixelColumns uint16  = 240
	lcdDPI          float64 = 265
	lcdRotation             = st7789.RotationNone
	spiPort                 = "SPI0.1"
	spiFrequency            = 80 * physic.MegaHertz
	spiMode                 = spi.Mode0
	spiBits         uint8   = 8
)

// DisplayOptions is the configuration for the display.
type DisplayOptions struct {
	Orientation int
	I18n        *i18n.I18n
}

// NewDisplay creates a new Pirate Audio display instance.
func NewDisplay(opts DisplayOptions) (*display.ST7789LCD, error) {
	angle := st7789.RotationToDegrees(lcdRotation)
	angle = display.SumAngle90(angle, opts.Orientation)

	return display.NewST7789LCD(&display.Config{
		DataCommPin:      gpioreg.ByName(dataCommPin),
		ResetPin:         gpioreg.ByName(resetPin),
		BacklightPin:     gpioreg.ByName(backlightPin),
		SPIPort:          spiPort,
		SPIFrequency:     spiFrequency,
		SPIMode:          spiMode,
		SPIBits:          spiBits,
		PixelRows:        lcdPixelRows,
		PixelColumns:     lcdPixelColumns,
		DPI:              lcdDPI,
		Rotation:         st7789.DegreesToRotation(angle),
		SetupDisplayFunc: setupDisplayFunc(),
		I18n:             opts.I18n,
	})
}

// setupDisplayFunc returns a function that initializes the ST7789 display.
func setupDisplayFunc() func(*st7789.Device) {
	return func(dev *st7789.Device) {
		dev.Command(st7789.SwReset)
		time.Sleep(150 * time.Millisecond)

		// Memory Data Access Control: X-Y Exchange, X-Mirror, Y-Mirror
		dev.Command(st7789.MadCtl)
		dev.Data(st7789.MadCtrlMxRL | st7789.MadCtrlMvRev | st7789.MadCtrlMlBT)

		// Porch Setting: Normal(Back Front), PSEN = disabled, Idle(Back, Front)
		dev.Command(st7789.PorCtrl)
		_ = dev.SendData(st7789.DefaultPORCTRL())

		// Interface pixel format: 16bit/pixel non-RGB
		dev.Command(st7789.ColMod)
		dev.Data(st7789.ColModCtrl65K)

		// Gate Control: High = 12.54v, Low = -9.6v
		dev.Command(st7789.GCtrl)
		dev.Data(0x14)

		// VCOM Setting: 0.575v
		dev.Command(st7789.VComs)
		dev.Data(0x37)

		// LCM Control: XOR RGB/BGR order, XOR Display Latch Order, XOR Page/Column order
		dev.Command(st7789.LcmCtrl)
		dev.Data(st7789.LCMCtrlXBGR | st7789.LCMCtrlXMH | st7789.LCMCtrlXMV)

		// VDVVRHEN: CMDEN = VDV and VRH register value comes from command write.
		dev.Command(st7789.VdvvRhen)
		_ = dev.SendData(st7789.DefaultVDVVRHEN())

		// VAP(GVDD) (V) = 4.45+(vcom+vcom offset+vdv)
		dev.Command(st7789.Vrhs)
		dev.Data(0x12)

		// VDV Set: 0v
		dev.Command(st7789.Vdvs)
		_ = dev.SendData(st7789.DefaultVDVS())

		// Power Control 1: AVDD = 6.8v, AVCL = -4.6v, VDS = 2.3v
		dev.Command(st7789.PwCtrl1)
		_ = dev.SendData(st7789.DefaultPWCTRL1())

		// Frame Rate Control (normal mode): 60Hz
		dev.Command(st7789.FrCtrl2)
		_ = dev.SendData(st7789.DefaultFRCTRL2())

		// Positive Voltage Gamma Control
		dev.Command(st7789.PVGAMCtrl)
		_ = dev.SendData([]byte{0xD0, 0x04, 0x0D, 0x11, 0x13, 0x2B, 0x3F, 0x54, 0x4C, 0x18, 0x0D, 0x0B, 0x1F, 0x23})

		// Negative Voltage Gamma Control
		dev.Command(st7789.NVGAMCtrl)
		_ = dev.SendData([]byte{0xD0, 0x04, 0x0C, 0x11, 0x13, 0x2C, 0x3F, 0x44, 0x51, 0x2F, 0x1F, 0x1F, 0x20, 0x23})

		// Display Inversion: on
		dev.Command(st7789.InvOn)

		// Sleep Mode: off
		dev.Command(st7789.SlpOut)

		// Display On Recovery: on
		dev.Command(st7789.DispOn)
	}
}

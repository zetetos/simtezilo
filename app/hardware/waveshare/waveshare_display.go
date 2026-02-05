package waveshare

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
	dataCommPin             = "GPIO25"
	resetPin                = "GPIO27"
	backlightPin            = "GPIO24"
	lcdPixelRows    uint16  = 240
	lcdPixelColumns uint16  = 240
	lcdDPI          float64 = 265
	lcdRotation             = st7789.Rotation90
	spiPort                 = "SPI0.0"
	spiFrequency            = 40 * physic.MegaHertz
	spiMode                 = spi.Mode0
	spiBits         uint8   = 8
)

type DisplayOptions struct {
	Orientation int
	I18n        *i18n.I18n
}

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

// setupDisplay initializes the display with the necessary commands and settings.
//
// This function is based on the Waveshare 1.3 inch LCD HAT code and Python ST7789 driver.
// https://files.waveshare.com/upload/b/bd/1.3inch_LCD_HAT_code.7z
// lib/LCD/LCD_1in3.c and python/ST7789.py.
func setupDisplayFunc() func(*st7789.Device) {
	return func(dev *st7789.Device) {
		dev.Command(st7789.SwReset)
		time.Sleep(150 * time.Millisecond)

		// Memory Data Access Control: X-Y Exchange, X-Mirror, Y-Mirror
		dev.Command(st7789.MadCtl)
		dev.Data(st7789.MadCtrlMxRL | st7789.MadCtrlMvRev | st7789.MadCtrlMlBT)

		// Sleep Mode: off
		dev.Command(st7789.SlpOut)

		time.Sleep(120 * time.Millisecond)

		// Memory Access Control: All defaults
		dev.Command(st7789.MadCtl)
		dev.Data(0x00)

		// Interface pixel format: 16bit/pixel non-RGB
		dev.Command(st7789.ColMod)
		dev.Data(st7789.ColModCtrl65K)

		// Porch Setting: Normal(Back Front), PSEN = disabled, Idle(Back, Front)
		dev.Command(st7789.PorCtrl)
		_ = dev.SendData(st7789.DefaultPORCTRL())

		// Gate Control: High = 12.2v, Low = -7.16v
		dev.Command(st7789.GCtrl)
		dev.Data(0x00)

		// VCOM Setting = 1.675v
		dev.Command(st7789.VComs)
		dev.Data(0x3F)

		// LCM Control: XOR RGB/BGR order, XOR Display Latch Order, XOR Page/Column order
		dev.Command(st7789.LcmCtrl)
		dev.Data(st7789.LCMCtrlXBGR | st7789.LCMCtrlXMH | st7789.LCMCtrlXMV)

		// VDVVRHEN: CMDEN = VDV and VRH register value comes from command write.
		dev.Command(st7789.VdvvRhen)
		_ = dev.SendData(st7789.DefaultVDVVRHEN())

		// VHR Set: VAP(GVDD) =  4.2v + (vcom+vcom offset+vdv)
		//          VAN(GVCL) = -4.2v + (vcom+vcom offset+vdv)
		dev.Command(st7789.Vrhs)
		dev.Data(0x0D)

		// Frame Rate Control (normal mode): 60Hz
		dev.Command(st7789.FrCtrl2)
		_ = dev.SendData(st7789.DefaultFRCTRL2())

		// Power Control: strange behavior in Waveshare drivers
		dev.Command(st7789.PwCtrl1)
		_ = dev.SendData([]byte{0xA7})

		// Power Control 1: AVDD = 6.8v, AVCL = -4.6v, VDS = 2.3v
		dev.Command(st7789.PwCtrl1)
		_ = dev.SendData(st7789.DefaultPWCTRL1())

		// Undocumented command: strange behaviour in Waveshare drivers
		dev.Command(0xD6)
		_ = dev.SendData([]byte{0xA1})

		// Positive Voltage Gamma Control
		dev.Command(st7789.PVGAMCtrl)
		_ = dev.SendData([]byte{0xF0, 0x00, 0x02, 0x01, 0x00, 0x00, 0x27, 0x43, 0x3F, 0x33, 0x0E, 0x0E, 0x26, 0x2E})

		// Negative Voltage Gamma Control
		dev.Command(st7789.NVGAMCtrl)
		_ = dev.SendData([]byte{0xF0, 0x07, 0x0D, 0x0D, 0x0B, 0x16, 0x26, 0x43, 0x3E, 0x3F, 0x19, 0x19, 0x31, 0x3A})

		// Display Inversion: on
		dev.Command(st7789.InvOn)

		// Display On Recovery: on
		dev.Command(st7789.DispOn)
	}
}

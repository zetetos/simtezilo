package display

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/png" // for loading sprite data
	"sync"
	"time"

	"github.com/zetetos/simtezilo/app/hardware/display/st7789"
	"github.com/zetetos/simtezilo/app/i18n"
	"github.com/zetetos/simtezilo/app/ui/sprites"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
)

// ST7789LCD represents an ST7789-based LCD display.
type ST7789LCD struct {
	port   spi.PortCloser
	device *st7789.Device

	dpi      float64
	rotation st7789.Rotation
	sleeping bool

	i18n    *i18n.I18n
	sprites *sprites.SpriteSet
	canvas  *image.RGBA
}

// Config holds the configuration parameters for initializing the ST7789LCD.
type Config struct {
	DataCommPin      gpio.PinOut
	ResetPin         gpio.PinIO
	BacklightPin     gpio.PinIO
	SPIPort          string
	SPIFrequency     physic.Frequency
	SPIMode          spi.Mode
	SPIBits          uint8
	PixelRows        uint16
	PixelColumns     uint16
	DPI              float64
	Rotation         st7789.Rotation
	SetupDisplayFunc func(*st7789.Device)
	I18n             *i18n.I18n // TODO: move rendering outside of display package
}

var once sync.Once //nolint:gochecknoglobals // idiomatic singleton

// NewST7789LCD initializes and returns a new ST7789 base LCD device instance based on the provided configuration.
func NewST7789LCD(config *Config) (*ST7789LCD, error) {
	if config == nil {
		return nil, errors.New("no configuration provided")
	}

	var (
		err           error
		spiPortCloser spi.PortCloser
		lcdDevice     *st7789.Device
	)

	once.Do(func() {
		spiPortCloser, err = spireg.Open(config.SPIPort)
		if err != nil {
			return
		}

		spiPort, ok := spiPortCloser.(spi.Port)
		if !ok {
			err = errors.New("SPI port does not implement spi.Port interface")

			return
		}

		st7789Config := &st7789.SPIDeviceConfig{
			DataCommPin:  config.DataCommPin,
			ResetPin:     config.ResetPin,
			BacklightPin: config.BacklightPin,
			SPIPort:      spiPort,
			SPIMode:      config.SPIMode,
			SPIFrequency: config.SPIFrequency,
			SPIBits:      config.SPIBits,
			PixelColumns: config.PixelColumns,
			PixelRows:    config.PixelRows,
			Rotation:     config.Rotation,
		}

		lcdDevice, err = st7789.NewSPI(st7789Config)
	})

	if err != nil {
		return nil, fmt.Errorf("initializing lcd device: %w", err)
	}

	err = lcdDevice.Reset()
	if err != nil {
		return nil, fmt.Errorf("resetting display: %w", err)
	}

	// initialise the display configuration
	if config.SetupDisplayFunc != nil {
		config.SetupDisplayFunc(lcdDevice)
	} else {
		setupDisplayDefault(lcdDevice)
	}

	// rotation must be set after the display is configured
	lcdDevice.SetRotation(config.Rotation)

	sprites, err := sprites.NewSpriteSet()
	if err != nil {
		return nil, fmt.Errorf("loading sprite set: %w", err)
	}

	if config.I18n == nil {
		return nil, errors.New("no font provided for display")
	}

	lcd := &ST7789LCD{
		port:   spiPortCloser,
		device: lcdDevice,

		dpi:     config.DPI,
		i18n:    config.I18n,
		sprites: sprites,

		rotation: config.Rotation,
		sleeping: true,
	}

	lcd.Clear()

	return lcd, nil
}

// Clear fills the display with black.
func (l *ST7789LCD) Clear() {
	l.device.FillScreen(color.RGBA{R: 0, G: 0, B: 0, A: 0})
}

// Close powers off the display and releases associated resources.
func (l *ST7789LCD) Close() {
	l.Clear()
	time.Sleep(1 * time.Second)

	_ = l.device.PowerOff()
	_ = l.port.Close()
}

// Wakeup powers on the display if it is currently sleeping.
func (l *ST7789LCD) Wakeup() {
	_ = l.device.PowerOn()
	l.sleeping = false
}

// Sleep powers off the display if it is currently awake.
func (l *ST7789LCD) Sleep() {
	if l.sleeping {
		return
	}

	_ = l.device.PowerOff()
	l.sleeping = true
}

// ToggleSleep switches the display between sleep and awake states, returning true if it is now sleeping.
func (l *ST7789LCD) ToggleSleep() bool {
	if l.sleeping {
		l.Sleep()

		return true
	}

	l.Wakeup()

	return false
}

// IsSleeping returns true if the display is currently in a sleep state.
func (l *ST7789LCD) IsSleeping() bool {
	return l.sleeping
}

// IsAwake returns true if the display is currently awake.
func (l *ST7789LCD) IsAwake() bool {
	return !l.sleeping
}

// GetResolution returns the current pixel resolution of the display.
func (l *ST7789LCD) GetResolution() (uint16, uint16) {
	return l.device.Size()
}

// GetDPI returns the dots-per-inch (DPI) of the display.
func (l *ST7789LCD) GetDPI() float64 {
	return l.dpi
}

// Write renders the provided content onto the display.
func (l *ST7789LCD) Write(content *Content) error {
	if content.Canvas == nil {
		return errors.New("canvas is nil")
	}

	l.device.DrawRAW(content.Canvas)
	l.canvas = content.Canvas

	return nil
}

// GetOrientation returns the current orientation of the display in degrees.
func (l *ST7789LCD) GetOrientation() int {
	switch l.rotation {
	case st7789.Rotation90:
		return 90
	case st7789.Rotation180:
		return 180
	case st7789.Rotation270:
		return 270
	case st7789.RotationNone:
		return 0
	}

	return 0
}

// SetOrientation sets the display orientation to the specified degrees (0, 90, 180, or 270).
func (l *ST7789LCD) SetOrientation(degrees int) {
	var rotation st7789.Rotation

	switch degrees {
	case 90:
		rotation = st7789.Rotation90
	case 180:
		rotation = st7789.Rotation180
	case 270:
		rotation = st7789.Rotation270
	default:
		rotation = st7789.RotationNone
	}

	l.device.SetRotation(rotation)
	l.rotation = rotation

	l.device.DrawRAW(l.canvas)
}

// RotateCW rotates the display orientation 90 degrees clockwise and returns the new orientation.
func (l *ST7789LCD) RotateCW() int {
	switch l.rotation {
	case st7789.Rotation90:
		l.SetOrientation(180)

		return 180
	case st7789.Rotation180:
		l.SetOrientation(270)

		return 270
	case st7789.Rotation270:
		l.SetOrientation(0)

		return 0
	case st7789.RotationNone:
		fallthrough
	default:
		l.SetOrientation(90)

		return 90
	}
}

// RotateCCW rotates the display orientation 90 degrees counter-clockwise and returns the new orientation.
func (l *ST7789LCD) RotateCCW() int {
	switch l.rotation {
	case st7789.Rotation90:
		l.SetOrientation(0)

		return 0
	case st7789.Rotation180:
		l.SetOrientation(90)

		return 90
	case st7789.Rotation270:
		l.SetOrientation(180)

		return 180
	case st7789.RotationNone:
		fallthrough
	default:
		l.SetOrientation(270)

		return 270
	}
}

// setupDisplay initializes the display with the necessary commands and settings.
func setupDisplayDefault(dev *st7789.Device) {
	dev.Command(st7789.SwReset)
	time.Sleep(150 * time.Millisecond)

	// Porch Setting: Normal(Back Front), PSEN = disabled, Idle(Back, Front)
	dev.Command(st7789.PorCtrl)
	_ = dev.SendData(st7789.DefaultPORCTRL())

	// Interface pixel format: 16bit/pixel non-RGB
	dev.Command(st7789.ColMod)
	dev.Data(st7789.ColModCtrl65K)

	// Gate Control: High = 12.54v, Low = -9.6v
	dev.Command(st7789.GCtrl)
	_ = dev.SendData(st7789.DefaultGCTRL())

	// VCOM Setting: 0.575v
	dev.Command(st7789.VComs)
	_ = dev.SendData(st7789.DefaultVCOMS())

	// LCM Control: XOR RGB/BGR order, XOR Display Latch Order, XOR Page/Column order
	dev.Command(st7789.LcmCtrl)
	_ = dev.SendData(st7789.DefaultLCMCTRL())

	// VDVVRHEN: CMDEN = VDV and VRH register value comes from command write.
	dev.Command(st7789.VdvvRhen)
	_ = dev.SendData(st7789.DefaultVDVVRHEN())

	// VAP(GVDD) (V) = 4.45+(vcom+vcom offset+vdv)
	dev.Command(st7789.Vrhs)
	_ = dev.SendData(st7789.DefaultVRHS())

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
	_ = dev.SendData(st7789.DefaultPVGAMCTRL())

	// Negative Voltage Gamma Control
	dev.Command(st7789.NVGAMCtrl)
	_ = dev.SendData(st7789.DefaultNVGAMCTRL())

	// Display Inversion: on
	dev.Command(st7789.InvOn)

	// Display On Recovery: off
	dev.Command(st7789.DispOff)

	// Sleep Mode: off
	dev.Command(st7789.SlpOut)
}

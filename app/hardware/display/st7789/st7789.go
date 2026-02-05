// Package st7789 provides a driver for the ST7789 LCD display controller.
package st7789

import (
	"errors"
	"fmt"
	"image"
	"time"

	"github.com/rs/zerolog"
	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
)

const (
	defaultPixelColumns uint16           = 320
	defaultPixelRows    uint16           = 240
	defaultSPIFrequency physic.Frequency = 40 * physic.MegaHertz
	defaultSPIBits      uint8            = 8
)

// SPIDeviceConfig holds the configuration for setting up SPI communication with an ST7789 device.
type SPIDeviceConfig struct {
	PixelColumns uint16           // Number of pixels in the horizontal direction, defaults to 320.
	PixelRows    uint16           // Number of pixels in the vertical direction, defaults to 240.
	Rotation     Rotation         // Display rotation, defaults to ROTATION_0.
	DataCommPin  gpio.PinOut      // GPIO pin used for data/command selection, must be set to a valid GPIO pin.
	ResetPin     gpio.PinIO       // GPIO pin used for resetting the display, defaults to gpio.INVALID.
	BacklightPin gpio.PinIO       // GPIO pin used for controlling the backlight, defaults to gpio.INVALID.
	SPIPort      spi.Port         // SPI port to use for communication, must be set to a valid SPI port.
	SPIMode      spi.Mode         // SPI mode to use, defaults to spi.Mode0.
	SPIFrequency physic.Frequency // SPI frequency to use, defaults to 40MHz.
	SPIBits      uint8            // Number of bits per SPI transfer, defaults to 8 bits.
	ColorBGR     bool             // If true, the display uses BGR color format, defaults to false (RGB).

	spiConn conn.Conn // Internal connection to the SPI device.

	log zerolog.Logger // Logger for logging messages and errors.
}

// Device represents an ST7789 LCD device with associated methods for interacting with the display.
type Device struct {
	conn      conn.Conn       // SPI connection to the device.
	dataComm  gpio.PinOut     // GPIO pin for data/command selection.
	reset     gpio.PinIO      // GPIO pin for resetting the display.
	backlight gpio.PinIO      // GPIO pin for controlling the backlight.
	rect      image.Rectangle // Rectangle defining the display area.

	rotation                      Rotation // Current rotation of the display.
	pixelColumns                  uint16   // Number of pixels in the horizontal direction.
	pixelRows                     uint16   // Number of pixels in the vertical direction.
	rowOffsetCfg, rowOffset       int16    // Row offset for the display, used for rotation adjustments.
	columnOffset, columnOffsetCfg int16    // Column offset for the display, used for rotation adjustments.
	isBGR                         bool     // Indicates if the display uses BGR color format.
	batchLength                   int32    // Length of the batch for pixel data transfers.

	log zerolog.Logger // Logger for logging messages and errors.
}

// NewSPI creates a new SPI connected ST7789 device and returns a handle to it.
func NewSPI(config *SPIDeviceConfig) (*Device, error) {
	err := validateConfig(config)
	if err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// ensure successful access to the data comm pin
	err = config.DataCommPin.Out(gpio.Low)
	if err != nil {
		return nil, fmt.Errorf("set data comm pin low: %w", err)
	}

	if config.BacklightPin != gpio.INVALID {
		// ensure successful access to the backlight pin and default to an off state
		err := config.BacklightPin.Out(gpio.Low)
		if err != nil {
			return nil, fmt.Errorf("set backlight pin low: %w", err)
		}
	}

	config.spiConn, err = config.SPIPort.Connect(config.SPIFrequency, config.SPIMode, int(config.SPIBits))
	if err != nil {
		return nil, fmt.Errorf("connect to SPI port: %w", err)
	}

	return newST7789Device(config)
}

// validateConfig checks the provided configuration for the ST7789 device and sets defaults where necessary.
func validateConfig(config *SPIDeviceConfig) error {
	if config.DataCommPin == gpio.INVALID || config.DataCommPin == nil {
		return errors.New("DataComm must be set to a valid GPIO pin")
	}

	if config.ResetPin == nil {
		config.ResetPin = gpio.INVALID
	}

	if config.BacklightPin == nil {
		config.BacklightPin = gpio.INVALID
	}

	if config.PixelColumns == 0 {
		config.PixelColumns = defaultPixelColumns
	}

	if config.PixelRows == 0 {
		config.PixelRows = defaultPixelRows
	}

	if config.SPIFrequency <= 0 {
		config.SPIFrequency = defaultSPIFrequency
	}

	if config.SPIBits <= 0 {
		config.SPIBits = defaultSPIBits
	}

	return nil
}

// newST7789Device initializes a new ST7789 device with the provided configuration.
func newST7789Device(config *SPIDeviceConfig) (*Device, error) {
	device := &Device{
		conn:         config.spiConn,
		dataComm:     config.DataCommPin,
		rect:         image.Rect(0, 0, int(config.PixelColumns), int(config.PixelRows)),
		rotation:     config.Rotation,
		pixelColumns: config.PixelColumns,
		pixelRows:    config.PixelRows,
		batchLength:  int32(config.PixelColumns),
		reset:        config.ResetPin,
		backlight:    config.BacklightPin,
		isBGR:        config.ColorBGR,
		log:          config.log.With().Str("component", "st7789").Logger(),
	}

	device.batchLength &= 1

	return device, nil
}

// String returns a string representation of the ST7789 device.
// It includes the connection type, data communication pin, and display dimensions.
func (d *Device) String() string {
	return fmt.Sprintf("st7789.Device{%s, %s, %s}", d.conn, d.dataComm, d.rect.Max)
}

// Reset triggers a software reset of the ST7789 display device.
// When the reset pin is not provided, this method does nothing.
func (d *Device) Reset() error {
	if d.reset == gpio.INVALID {
		return nil
	}

	err := d.reset.Out(gpio.High)
	if err != nil {
		return err
	}

	time.Sleep(10 * time.Millisecond)

	err = d.reset.Out(gpio.Low)
	if err != nil {
		return err
	}

	time.Sleep(10 * time.Millisecond)

	err = d.reset.Out(gpio.High)
	if err != nil {
		return err
	}

	time.Sleep(10 * time.Millisecond)

	return nil
}

// PowerOff disables the backlight of the ST7789 display device.
// When the backlight pin is not provided, this method does nothing.
func (d *Device) PowerOff() error {
	if d.backlight == gpio.INVALID {
		return nil
	}

	return d.backlight.Out(gpio.Low)
}

// PowerOn enables the backlight of the ST7789 display device.
// When the backlight pin is not provided, this method does nothing.
func (d *Device) PowerOn() error {
	if d.backlight == gpio.INVALID {
		return nil
	}

	return d.backlight.Out(gpio.High)
}

// Invert performs a black/white inversion of the display contents.
func (d *Device) Invert(blackOnWhite bool) {
	b := byte(0xA6)
	if blackOnWhite {
		b = 0xA7
	}

	d.Command(b)
}

// SendData sends a block of data to the ST7789 display device.
func (d *Device) SendData(data []byte) error {
	err := d.dataComm.Out(gpio.High)
	if err != nil {
		return err
	}

	return d.conn.Tx(data, nil)
}

// SendCommand sends a command to the ST7789 display device.
func (d *Device) SendCommand(command []byte) error {
	err := d.dataComm.Out(gpio.Low)
	if err != nil {
		return err
	}

	return d.conn.Tx(command, nil)
}

// Size returns the current pixel row and column sizes of the display.
func (d *Device) Size() (uint16, uint16) {
	if d.rotation == RotationNone || d.rotation == Rotation180 {
		return d.pixelColumns, d.pixelRows
	}

	return d.pixelRows, d.pixelColumns
}

// PixelCount returns the total pixels count of the display.
func (d *Device) PixelCount() uint32 {
	return uint32(d.pixelColumns) * uint32(d.pixelRows)
}

// SetWindow sets the current window dimensions for drawing on the display.
func (d *Device) SetWindow() {
	xMin := 0
	yMin := 0
	xMax := d.pixelColumns - 1
	yMax := d.pixelRows - 1

	d.Command(CaSet)
	d.Data(byte(xMin >> 8))
	d.Data(byte(xMin & 0xFF))
	d.Data(byte(xMax >> 8))
	d.Data(byte(xMax & 0xFF))

	d.Command(RaSet)
	d.Data(byte(yMin >> 8))
	d.Data(byte(yMin & 0xFF))
	d.Data(byte(yMax >> 8))
	d.Data(byte(yMax & 0xFF))

	d.Command(Ramwr)
	d.Data(0x89)
}

// InvertColors sends and invert color command to the display device.
func (d *Device) InvertColors(invert bool) {
	if invert {
		d.Command(InvOn)
	} else {
		d.Command(InvOff)
	}
}

// Command sends a integer formatted command to the display device.
func (d *Device) Command(cmd uint8) {
	_ = d.SendCommand([]byte{cmd})
}

// Data sends and integer formated data block to the display device.
func (d *Device) Data(data uint8) {
	_ = d.SendData([]byte{data})
}

// SetRotation sets the rotation of the content on the display device.
// ROTATION_NONE is at 12 o'clock relative to the natural top of the display
// with higher values rotating as degress in a clockwise direction.
func (d *Device) SetRotation(rotation Rotation) {
	var madctlData uint8

	vscsadData := verticalScrollOffset(0)

	switch rotation % 4 {
	case Rotation90:
		madctlData = MadCtrlMxRL | MadCtrlMyBT | MadCtrlMvNorm
		vscsadData = verticalScrollOffset(320 - int(d.pixelColumns))
		d.rowOffset = d.columnOffsetCfg
		d.columnOffset = d.rowOffsetCfg
	case Rotation180:
		madctlData = MadCtrlMxLR | MadCtrlMyBT | MadCtrlMvRev
		vscsadData = verticalScrollOffset(320 - int(d.pixelColumns))
		d.rowOffset = 0
		d.columnOffset = 0
	case Rotation270:
		madctlData = MadCtrlMxLR | MadCtrlMyTB | MadCtrlMvNorm
		d.rowOffset = 0
		d.columnOffset = 0
	case RotationNone:
		fallthrough
	default:
		madctlData = MadCtrlMxRL | MadCtrlMyTB | MadCtrlMvRev
		d.rowOffset = d.rowOffsetCfg
		d.columnOffset = d.columnOffsetCfg
	}

	if d.isBGR {
		madctlData |= MadCtrlBGR
	}

	// Set the display orientation
	d.Command(MadCtl)
	d.Data(madctlData)

	// Set vertical scroll offset so that images are located correctly on 240 pixel row displays
	d.Command(VscsAD)
	_ = d.SendData(vscsadData)
}

// DegreesToRotation converts an integer representing degrees to a Rotation type.
//
// Degree values of 0, 90, 180, and 270 are mapped to their corresponding Rotation values.
// If the input does not match these values ROTATION_NONE will be returned.
func DegreesToRotation(orientation int) Rotation {
	switch orientation {
	case 90:
		return Rotation90
	case 180:
		return Rotation180
	case 270:
		return Rotation270
	default:
		return RotationNone
	}
}

// RotationToDegrees converts a Rotation type to its corresponding degree value.
func RotationToDegrees(rotation Rotation) int {
	switch rotation {
	case Rotation90:
		return 90
	case Rotation180:
		return 180
	case Rotation270:
		return 270
	case RotationNone:
		return 0
	}

	return 0
}

// getResolution returns the X/Y pixel resolution of the display based on its current rotation.
func (d *Device) getResolution() (uint16, uint16) {
	if d.rotation%Rotation180 == 0 {
		return d.pixelColumns, d.pixelRows
	}

	return d.pixelRows, d.pixelColumns
}

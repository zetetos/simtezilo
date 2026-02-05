package st7789

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
)

// FillRectangle fills a rectangle at a given coordinates with a color.
func (d *Device) FillRectangle(x, y, width, height uint16, colorRGBA color.RGBA) error {
	k, i := d.Size()
	if width == 0 || height == 0 ||
		x >= k || (x+width) > k || y >= i || (y+height) > i {
		return errors.New("rectangle coordinates outside display area")
	}

	d.SetWindow()

	c565 := RGBAToRGB565(colorRGBA)
	// Safe conversion from uint16 to uint8 - RGB565 values fit in uint8 when split
	c1 := byte(c565)      // Low byte
	c2 := byte(c565 >> 8) // High byte

	data := make([]uint8, d.PixelCount())
	for rowPixel := range int32(d.pixelColumns) {
		data[rowPixel*2] = c1
		data[rowPixel*2+1] = c2
	}

	column := int32(width) * int32(height)
	for column > 0 {
		if column >= int32(d.pixelRows) {
			_ = d.SendData(data)
		} else {
			_ = d.SendData(data[:column*2])
		}

		column -= int32(d.pixelRows)
	}

	return nil
}

// RGBAToRGB565 converts a 32 bit RGBA color to 16 bit RGB565 format.
func RGBAToRGB565(c color.RGBA) uint16 {
	r := uint16(c.R) >> 3
	g := uint16(c.G) >> 2
	b := uint16(c.B) >> 3

	return (r << 11) | (g << 5) | b
}

// SetPixel sets a pixel in the screen.
func (d *Device) SetPixel(xPos uint16, yPos uint16, colorRGBA color.RGBA) {
	resX, resY := d.getResolution()
	if xPos >= resX || yPos >= resY {
		return
	}

	_ = d.FillRectangle(xPos, yPos, 1, 1, colorRGBA)
}

// FillScreen fills the screen with a given color.
func (d *Device) FillScreen(c color.RGBA) {
	x, y := d.getResolution()

	_ = d.FillRectangle(0, 0, x, y, c)
}

// DrawRAW draws an image to the screen.
func (d *Device) DrawRAW(img image.Image) {
	d.SetWindow()

	rect := img.Bounds()
	rgbaimg := image.NewRGBA(rect)
	draw.Draw(rgbaimg, rect, img, rect.Min, draw.Src)

	data := make([]uint8, 0, int(d.pixelColumns)*int(d.pixelRows)*2)

	for column := range d.pixelColumns {
		for row := range d.pixelRows {
			x := rect.Min.X + int(d.pixelColumns) - int(column)
			y := rect.Min.Y + int(row)

			rgba, ok := rgbaimg.At(x, y).(color.RGBA)
			if !ok {
				rgba = color.RGBA{R: 252, G: 15, B: 192, A: 0} // strong pink to highlight bad pixel color
			}

			c565 := RGBAToRGB565(rgba)
			// Safe conversion from uint16 to uint8 - RGB565 values fit in uint8 when split
			data = append(data, byte(c565), byte(c565>>8))
		}
	}

	chunkSize := 4096
	for offset := 0; offset < len(data); offset += chunkSize {
		end := min(offset+chunkSize, len(data))

		err := d.SendData(data[offset:end])
		if err != nil {
			d.log.Error().
				Err(err).
				Str("result", "failure").
				Int("offset", offset).
				Msg("send data")
		}
	}
}

// Package sprites provides functionality to load and retrieve sprites from a sprite sheet.
package sprites

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"image"
)

// SpriteName represents a limited set of valid sprite names.
type SpriteName string

const (
	SplashSprite SpriteName = "splash"
	ErrorSprite  SpriteName = "error"
)

// SpriteSet holds the sprite sheet image and a mapping of sprite names to their rectangles.
type SpriteSet struct {
	image  *image.RGBA
	sprite map[SpriteName]image.Rectangle
}

//go:embed sprites.png
var spritesImage []byte

// NewSpriteSet loads the sprite sheet image and initializes the sprite mapping.
func NewSpriteSet() (*SpriteSet, error) {
	data := bytes.NewReader(spritesImage)

	img, _, err := image.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	rgbaImg, ok := img.(*image.RGBA)
	if !ok {
		return nil, errors.New("image is not *image.RGBA type")
	}

	return &SpriteSet{
		image:  rgbaImg,
		sprite: spriteMap(),
	}, nil
}

// GetSprite retrieves the sprite image corresponding to the given name.
func (s *SpriteSet) GetSprite(name SpriteName) image.Image {
	if _, ok := s.sprite[name]; !ok {
		name = ErrorSprite
	}

	rect := s.sprite[name]

	subImage, ok := s.image.SubImage(rect).(*image.RGBA)
	if !ok {
		return nil
	}

	// Create a new image with bounds starting at (0,0)
	repositionedImage := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))

	// Copy the pixels from the subimage to the new image
	for y := range rect.Dy() {
		for x := range rect.Dx() {
			repositionedImage.Set(x, y, subImage.At(rect.Min.X+x, rect.Min.Y+y))
		}
	}

	return repositionedImage
}

// spriteMap defines the mapping of sprite names to their rectangles in the sprite sheet.
func spriteMap() map[SpriteName]image.Rectangle {
	return map[SpriteName]image.Rectangle{
		SplashSprite: image.Rect(0*240, 0*240, 1*240, 1*240),
		ErrorSprite:  image.Rect(1*240, 0*240, 2*240, 1*240),
	}
}

package font

import (
	"embed"
	"fmt"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

// Font represents a font with its associated scale.
type Font struct {
	Font  *truetype.Font
	Scale float64
}

//go:embed fonts/*
var staticFiles embed.FS

// GetFont retrieves and parses a TrueType font by name from the embedded filesystem.
func GetFont(name string) (*truetype.Font, error) {
	filename := "fonts/" + name

	fontData, err := staticFiles.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read font file %q: %w", filename, err)
	}

	return parseTrueTypeFont(fontData)
}

// parseTrueTypeFont parses the provided byte slice into a TrueType font.
func parseTrueTypeFont(fontBytes []byte) (*truetype.Font, error) {
	freetypeFont, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing font: %w", err)
	}

	return freetypeFont, nil
}

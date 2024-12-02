package palette

import (
	"fmt"
	"math/rand"

	"github.com/watzon/pigmentpoet/color"
)

// Generator handles the generation of color palettes
type Generator struct {
	matcher *color.ColorMatcher
	types   []color.PaletteType
}

// NewGenerator creates a new palette generator
func NewGenerator() (*Generator, error) {
	matcher, err := color.NewPreloadedColorMatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create color matcher: %w", err)
	}

	return &Generator{
		matcher: matcher,
		types: []color.PaletteType{
			color.Complementary,
			color.Triadic,
			color.Analogous,
			color.SplitComplementary,
			color.Tetradic,
			color.Monochromatic,
		},
	}, nil
}

// GenerateRandomPalette generates a random color palette
func (g *Generator) GenerateRandomPalette() (*Palette, error) {
	// Generate a random base color
	baseColor := fmt.Sprintf("#%02X%02X%02X",
		rand.Intn(256),
		rand.Intn(256),
		rand.Intn(256))

	// Select a random palette type
	paletteType := g.types[rand.Intn(len(g.types))]

	return g.GeneratePalette(baseColor, paletteType)
}

// GeneratePalette generates a palette from a base color and type
func (g *Generator) GeneratePalette(baseColor string, paletteType color.PaletteType) (*Palette, error) {
	colors := g.matcher.GeneratePalette(baseColor, paletteType, 5)

	var hexCodes []string
	var names []string
	for _, c := range colors {
		hex := fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
		hexCodes = append(hexCodes, hex)
		if colorName, err := g.matcher.FindClosestColor(hex); err == nil {
			names = append(names, colorName.Name)
		} else {
			names = append(names, "Unknown")
		}
	}

	return &Palette{
		Colors:   colors,
		Names:    names,
		HexCodes: hexCodes,
		Type:     paletteType,
		TypeName: getPaletteTypeName(paletteType),
	}, nil
}

// getPaletteTypeName returns a human-readable name for the palette type
func getPaletteTypeName(pt color.PaletteType) string {
	switch pt {
	case color.Complementary:
		return "Complementary Palette"
	case color.Triadic:
		return "Triadic Palette"
	case color.Analogous:
		return "Analogous Palette"
	case color.SplitComplementary:
		return "Split Complementary Palette"
	case color.Tetradic:
		return "Tetradic Palette"
	case color.Monochromatic:
		return "Monochromatic Palette"
	default:
		return "Custom Palette"
	}
}

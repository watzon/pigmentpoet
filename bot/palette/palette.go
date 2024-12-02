package palette

import (
	"image"

	"github.com/watzon/pigmentpoet/color"
)

// Palette represents a generated color palette
type Palette struct {
	Colors   []color.Color
	Names    []string
	HexCodes []string
	Type     color.PaletteType
	TypeName string
}

// ToImage converts the palette to an image
func (p *Palette) ToImage() (image.Image, error) {
	cfg := color.PaletteImage{
		Colors:       p.Colors,
		Names:        p.Names,
		HexCodes:     p.HexCodes,
		ShowHexCodes: true,
		ShowNames:    true,
	}

	return color.GeneratePaletteImage(cfg)
}

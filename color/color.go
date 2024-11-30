package color

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"sort"
)

//go:embed colors.json
var colorData []byte

// ColorName represents a named color with its hex value
type ColorName struct {
	Hex  string `json:"hex"`
	Name string `json:"name"`
}

// Color represents an RGB color
type Color struct {
	R uint8
	G uint8
	B uint8
}

// HSL represents a color in HSL space
type HSL struct {
	H float64
	S float64
	L float64
}

// ColorMatcher handles color matching and analysis
type ColorMatcher struct {
	colors []ColorName
	// Cache RGB and HSL values for performance
	colorCache map[string]struct {
		RGB Color
		HSL HSL
	}
}

// PreloadedColorMatcher returns a new ColorMatcher with the embedded colors
func NewPreloadedColorMatcher() (*ColorMatcher, error) {
	return NewColorMatcher(colorData)
}

// NewColorMatcher creates a new color matcher from JSON data
func NewColorMatcher(jsonData []byte) (*ColorMatcher, error) {
	var colors []ColorName
	if err := json.Unmarshal(jsonData, &colors); err != nil {
		return nil, fmt.Errorf("failed to parse color data: %w", err)
	}

	matcher := &ColorMatcher{
		colors: colors,
		colorCache: make(map[string]struct {
			RGB Color
			HSL HSL
		}),
	}

	// Initialize cache
	for _, c := range colors {
		rgb := hexToRGB(c.Hex)
		hsl := rgbToHSL(rgb)
		matcher.colorCache[c.Hex] = struct {
			RGB Color
			HSL HSL
		}{RGB: rgb, HSL: hsl}
	}

	return matcher, nil
}

// FindClosestColor finds the closest named color to the given hex color
func (m *ColorMatcher) FindClosestColor(hex string) (ColorName, error) {
	targetRGB := hexToRGB(hex)
	targetHSL := rgbToHSL(targetRGB)

	var closest ColorName
	minDiff := math.MaxFloat64

	for _, c := range m.colors {
		cached := m.colorCache[c.Hex]

		// Calculate color difference using a combination of RGB and HSL
		rgbDiff := colorDifferenceRGB(targetRGB, cached.RGB)
		hslDiff := colorDifferenceHSL(targetHSL, cached.HSL)

		// Weight HSL difference more heavily as it better matches human perception
		totalDiff := rgbDiff + 2*hslDiff

		if totalDiff < minDiff {
			minDiff = totalDiff
			closest = c
		}
	}

	return closest, nil
}

// Helper functions for color conversion and comparison
func hexToRGB(hex string) Color {
	// Remove # prefix if present
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}
	var r, g, b uint8
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return Color{R: r, G: g, B: b}
}

func rgbToHSL(rgb Color) HSL {
	r := float64(rgb.R) / 255
	g := float64(rgb.G) / 255
	b := float64(rgb.B) / 255

	max := math.Max(math.Max(r, g), b)
	min := math.Min(math.Min(r, g), b)
	h, s, l := 0.0, 0.0, (max+min)/2

	if max != min {
		d := max - min
		s = d / (2 - max - min)
		if l > 0.5 {
			s = d / (2 - (max + min))
		}

		switch max {
		case r:
			h = (g - b) / d
			if g < b {
				h += 6
			}
		case g:
			h = (b-r)/d + 2
		case b:
			h = (r-g)/d + 4
		}
		h /= 6
	}

	return HSL{H: h * 360, S: s * 100, L: l * 100}
}

func colorDifferenceRGB(c1, c2 Color) float64 {
	return math.Pow(float64(c1.R)-float64(c2.R), 2) +
		math.Pow(float64(c1.G)-float64(c2.G), 2) +
		math.Pow(float64(c1.B)-float64(c2.B), 2)
}

func colorDifferenceHSL(c1, c2 HSL) float64 {
	return math.Pow(c1.H-c2.H, 2) +
		math.Pow(c1.S-c2.S, 2) +
		math.Pow(c1.L-c2.L, 2)
}

// Palette extraction functionality
type Box struct {
	rMin, rMax, gMin, gMax, bMin, bMax int
	colors                             []Color
}

func (b *Box) volume() int {
	return (b.rMax - b.rMin + 1) * (b.gMax - b.gMin + 1) * (b.bMax - b.bMin + 1)
}

// ExtractPalette extracts a color palette from an image
func ExtractPalette(img image.Image, numColors int) []Color {
	if numColors < 2 {
		numColors = 2
	}
	if numColors > 256 {
		numColors = 256
	}

	// Convert image to color slice
	var colors []Color
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a > 0 {
				colors = append(colors, Color{
					R: uint8(r >> 8),
					G: uint8(g >> 8),
					B: uint8(b >> 8),
				})
			}
		}
	}

	// Create initial box containing all colors
	box := createBox(colors)
	boxes := []*Box{box}

	// Extract more colors than needed to account for filtering
	targetColors := numColors * 2

	// Split boxes until we have desired number of colors
	for len(boxes) < targetColors {
		boxToSplit := findBoxToSplit(boxes)
		if boxToSplit == nil {
			break
		}
		box1, box2 := splitBox(boxToSplit)
		boxes = append(boxes[:len(boxes)-1], box1, box2)
	}

	// Extract average color from each box
	palette := make([]Color, len(boxes))
	for i, box := range boxes {
		palette[i] = averageColor(box.colors)
	}

	// Filter similar colors with a threshold of 60 (adjust this value as needed)
	palette = filterSimilarColors(palette, 60.0)

	// If we have more colors than requested, take the first numColors
	if len(palette) > numColors {
		palette = palette[:numColors]
	}

	return palette
}

// colorDistance calculates the Euclidean distance between two colors in RGB space
func colorDistance(c1, c2 Color) float64 {
	rDiff := float64(c1.R) - float64(c2.R)
	gDiff := float64(c1.G) - float64(c2.G)
	bDiff := float64(c1.B) - float64(c2.B)
	return math.Sqrt(rDiff*rDiff + gDiff*gDiff + bDiff*bDiff)
}

// filterSimilarColors removes colors that are too similar to each other
func filterSimilarColors(colors []Color, threshold float64) []Color {
	if len(colors) <= 1 {
		return colors
	}

	result := []Color{colors[0]}
	for i := 1; i < len(colors); i++ {
		isDistinct := true
		for _, existing := range result {
			if dist := colorDistance(colors[i], existing); dist < threshold {
				isDistinct = false
				break
			}
		}
		if isDistinct {
			result = append(result, colors[i])
		}
	}
	return result
}

// PaletteType represents different types of color palettes
type PaletteType int

const (
	Complementary PaletteType = iota
	Triadic
	Analogous
	SplitComplementary
	Tetradic
	Monochromatic
)

// GeneratePalette creates a palette of colors based on a base color and palette type
func (m *ColorMatcher) GeneratePalette(baseHex string, paletteType PaletteType, variations int) []Color {
	baseColor := hexToRGB(baseHex)
	baseHSL := rgbToHSL(baseColor)

	switch paletteType {
	case Complementary:
		return m.complementaryPalette(baseHSL)
	case Triadic:
		return m.triadicPalette(baseHSL)
	case Analogous:
		return m.analogousPalette(baseHSL, variations)
	case SplitComplementary:
		return m.splitComplementaryPalette(baseHSL)
	case Tetradic:
		return m.tetradicPalette(baseHSL)
	case Monochromatic:
		return m.monochromaticPalette(baseHSL, variations)
	default:
		return []Color{baseColor}
	}
}

// Helper function to rotate hue
func rotateHue(hsl HSL, degrees float64) HSL {
	hsl.H = math.Mod(hsl.H+degrees, 360)
	if hsl.H < 0 {
		hsl.H += 360
	}
	return hsl
}

// Convert HSL back to RGB
func hslToRGB(hsl HSL) Color {
	h := hsl.H / 360
	s := hsl.S / 100
	l := hsl.L / 100

	var r, g, b float64

	if s == 0 {
		r = l
		g = l
		b = l
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q

		r = hueToRGB(p, q, h+1.0/3.0)
		g = hueToRGB(p, q, h)
		b = hueToRGB(p, q, h-1.0/3.0)
	}

	return Color{
		R: uint8(math.Round(r * 255)),
		G: uint8(math.Round(g * 255)),
		B: uint8(math.Round(b * 255)),
	}
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	if t < 1.0/6.0 {
		return p + (q-p)*6*t
	}
	if t < 1.0/2.0 {
		return q
	}
	if t < 2.0/3.0 {
		return p + (q-p)*(2.0/3.0-t)*6
	}
	return p
}

// Palette generation methods
func (m *ColorMatcher) complementaryPalette(base HSL) []Color {
	complement := rotateHue(base, 180)
	// Create variations between base and complement
	color2 := HSL{
		H: base.H,
		S: base.S * 0.8,
		L: base.L * 1.2,
	}
	color3 := HSL{
		H: complement.H,
		S: complement.S * 0.8,
		L: complement.L * 1.2,
	}
	color4 := HSL{
		H: base.H,
		S: base.S * 0.6,
		L: base.L * 1.4,
	}
	return []Color{
		hslToRGB(base),
		hslToRGB(color2),
		hslToRGB(color3),
		hslToRGB(color4),
		hslToRGB(complement),
	}
}

func (m *ColorMatcher) triadicPalette(base HSL) []Color {
	color2 := rotateHue(base, 120)
	color3 := rotateHue(base, 240)
	// Add intermediate colors
	color4 := rotateHue(base, 60)
	color5 := rotateHue(base, 180)
	return []Color{
		hslToRGB(base),
		hslToRGB(color4),
		hslToRGB(color2),
		hslToRGB(color5),
		hslToRGB(color3),
	}
}

func (m *ColorMatcher) analogousPalette(base HSL, variations int) []Color {
	angleStep := 15.0
	colors := make([]Color, 5)
	colors[0] = hslToRGB(base)
	
	// Two colors clockwise
	colors[1] = hslToRGB(rotateHue(base, -angleStep))
	colors[2] = hslToRGB(rotateHue(base, -angleStep*2))
	
	// Two colors counterclockwise
	colors[3] = hslToRGB(rotateHue(base, angleStep))
	colors[4] = hslToRGB(rotateHue(base, angleStep*2))
	
	return colors
}

func (m *ColorMatcher) splitComplementaryPalette(base HSL) []Color {
	complement := rotateHue(base, 180)
	split1 := rotateHue(complement, -30)
	split2 := rotateHue(complement, 30)
	// Add intermediate colors
	color4 := HSL{
		H: base.H,
		S: base.S * 0.8,
		L: base.L * 1.2,
	}
	color5 := HSL{
		H: complement.H,
		S: complement.S * 0.8,
		L: complement.L * 1.2,
	}
	return []Color{
		hslToRGB(base),
		hslToRGB(color4),
		hslToRGB(split1),
		hslToRGB(split2),
		hslToRGB(color5),
	}
}

func (m *ColorMatcher) tetradicPalette(base HSL) []Color {
	color2 := rotateHue(base, 90)
	color3 := rotateHue(base, 180)
	color4 := rotateHue(base, 270)
	// Add intermediate color
	color5 := HSL{
		H: base.H,
		S: base.S * 0.8,
		L: base.L * 1.2,
	}
	return []Color{
		hslToRGB(base),
		hslToRGB(color2),
		hslToRGB(color3),
		hslToRGB(color4),
		hslToRGB(color5),
	}
}

func (m *ColorMatcher) monochromaticPalette(base HSL, variations int) []Color {
	colors := make([]Color, 5)
	colors[0] = hslToRGB(base)

	// Two lighter shades
	colors[1] = hslToRGB(HSL{
		H: base.H,
		S: math.Max(0, base.S*0.8),
		L: math.Min(100, base.L*1.2),
	})
	colors[2] = hslToRGB(HSL{
		H: base.H,
		S: math.Max(0, base.S*0.6),
		L: math.Min(100, base.L*1.4),
	})

	// Two darker shades
	colors[3] = hslToRGB(HSL{
		H: base.H,
		S: math.Min(100, base.S*1.2),
		L: math.Max(0, base.L*0.8),
	})
	colors[4] = hslToRGB(HSL{
		H: base.H,
		S: math.Min(100, base.S*1.4),
		L: math.Max(0, base.L*0.6),
	})

	return colors
}

func createBox(colors []Color) *Box {
	if len(colors) == 0 {
		return &Box{}
	}

	box := &Box{
		rMin: 255, rMax: 0,
		gMin: 255, gMax: 0,
		bMin: 255, bMax: 0,
		colors: colors,
	}

	for _, c := range colors {
		box.rMin = min(box.rMin, int(c.R))
		box.rMax = max(box.rMax, int(c.R))
		box.gMin = min(box.gMin, int(c.G))
		box.gMax = max(box.gMax, int(c.G))
		box.bMin = min(box.bMin, int(c.B))
		box.bMax = max(box.bMax, int(c.B))
	}

	return box
}

func findBoxToSplit(boxes []*Box) *Box {
	var maxBox *Box
	maxVolume := 0

	for _, box := range boxes {
		volume := box.volume()
		if volume > maxVolume {
			maxVolume = volume
			maxBox = box
		}
	}

	return maxBox
}

func splitBox(box *Box) (*Box, *Box) {
	// Find longest dimension
	rRange := box.rMax - box.rMin
	gRange := box.gMax - box.gMin
	bRange := box.bMax - box.bMin

	var dim byte
	switch {
	case rRange >= gRange && rRange >= bRange:
		dim = 'r'
	case gRange >= rRange && gRange >= bRange:
		dim = 'g'
	default:
		dim = 'b'
	}

	// Sort colors along chosen dimension
	sort.Slice(box.colors, func(i, j int) bool {
		switch dim {
		case 'r':
			return box.colors[i].R < box.colors[j].R
		case 'g':
			return box.colors[i].G < box.colors[j].G
		default:
			return box.colors[i].B < box.colors[j].B
		}
	})

	// Split at median
	median := len(box.colors) / 2
	box1 := createBox(box.colors[:median])
	box2 := createBox(box.colors[median:])

	return box1, box2
}

func averageColor(colors []Color) Color {
	if len(colors) == 0 {
		return Color{}
	}

	var rSum, gSum, bSum int
	for _, c := range colors {
		rSum += int(c.R)
		gSum += int(c.G)
		bSum += int(c.B)
	}

	count := len(colors)
	return Color{
		R: uint8(rSum / count),
		G: uint8(gSum / count),
		B: uint8(bSum / count),
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

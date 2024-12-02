package color

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"math/rand"
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

// LAB represents a color in CIELAB color space
type LAB struct {
	L float64
	A float64
	B float64
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

// LAB color space functions
func RGBToLAB(c Color) LAB {
	// First convert to XYZ
	r := float64(c.R) / 255.0
	g := float64(c.G) / 255.0
	b := float64(c.B) / 255.0

	// Convert to sRGB
	r = toLinear(r)
	g = toLinear(g)
	b = toLinear(b)

	// Convert to XYZ
	x := 0.4124564*r + 0.3575761*g + 0.1804375*b
	y := 0.2126729*r + 0.7151522*g + 0.0721750*b
	z := 0.0193339*r + 0.1191920*g + 0.9503041*b

	// Convert XYZ to LAB
	x = x / 0.95047
	y = y / 1.00000
	z = z / 1.08883

	x = toLAB(x)
	y = toLAB(y)
	z = toLAB(z)

	return LAB{
		L: 116*y - 16,
		A: 500 * (x - y),
		B: 200 * (y - z),
	}
}

func toLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func toLAB(t float64) float64 {
	if t > 0.008856 {
		return math.Pow(t, 1.0/3.0)
	}
	return (903.3*t + 16) / 116
}

func LABDistance(a, b LAB) float64 {
	dL := a.L - b.L
	dA := a.A - b.A
	dB := a.B - b.B
	return math.Sqrt(dL*dL + dA*dA + dB*dB)
}

// ExtractPalette extracts a color palette from an image using k-means clustering
func ExtractPalette(img image.Image, numColors int) []Color {
	if numColors < 2 {
		numColors = 2
	}
	if numColors > 256 {
		numColors = 256
	}

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	// Sample pixels using a grid to avoid over-sampling large areas
	gridSize := int(math.Sqrt(float64(width*height) / 1000)) // Adjust sampling density
	if gridSize < 1 {
		gridSize = 1
	}

	var pixels []LAB
	var rgbColors []Color
	for y := bounds.Min.Y; y < bounds.Max.Y; y += gridSize {
		for x := bounds.Min.X; x < bounds.Max.X; x += gridSize {
			r, g, b, a := img.At(x, y).RGBA()
			if a > 0 {
				color := Color{
					R: uint8(r >> 8),
					G: uint8(g >> 8),
					B: uint8(b >> 8),
				}
				lab := RGBToLAB(color)
				pixels = append(pixels, lab)
				rgbColors = append(rgbColors, color)
			}
		}
	}

	if len(pixels) == 0 {
		return []Color{}
	}

	// Initialize k-means centroids using k-means++ initialization
	centroids := make([]LAB, numColors)
	firstIdx := rand.Intn(len(pixels))
	centroids[0] = pixels[firstIdx]

	for i := 1; i < numColors; i++ {
		// Calculate distances to nearest centroid for each point
		maxDist := 0.0
		maxIdx := 0
		for j, pixel := range pixels {
			minDist := math.MaxFloat64
			for k := 0; k < i; k++ {
				dist := LABDistance(pixel, centroids[k])
				if dist < minDist {
					minDist = dist
				}
			}
			if minDist > maxDist {
				maxDist = minDist
				maxIdx = j
			}
		}
		centroids[i] = pixels[maxIdx]
	}

	// Run k-means clustering
	maxIterations := 20
	for iteration := 0; iteration < maxIterations; iteration++ {
		// Assign points to nearest centroid
		clusters := make([][]int, numColors)
		for i, pixel := range pixels {
			minDist := math.MaxFloat64
			minIdx := 0
			for j, centroid := range centroids {
				dist := LABDistance(pixel, centroid)
				if dist < minDist {
					minDist = dist
					minIdx = j
				}
			}
			clusters[minIdx] = append(clusters[minIdx], i)
		}

		// Update centroids
		moved := false
		for i := 0; i < numColors; i++ {
			if len(clusters[i]) == 0 {
				continue
			}
			var sumL, sumA, sumB float64
			for _, idx := range clusters[i] {
				sumL += pixels[idx].L
				sumA += pixels[idx].A
				sumB += pixels[idx].B
			}
			newCentroid := LAB{
				L: sumL / float64(len(clusters[i])),
				A: sumA / float64(len(clusters[i])),
				B: sumB / float64(len(clusters[i])),
			}
			if LABDistance(newCentroid, centroids[i]) > 0.1 {
				moved = true
			}
			centroids[i] = newCentroid
		}

		if !moved {
			break
		}
	}

	// Convert centroids back to RGB colors
	result := make([]Color, numColors)
	for i, cluster := range centroids {
		// Find the RGB color closest to this LAB centroid
		minDist := math.MaxFloat64
		var bestColor Color
		for j, pixel := range pixels {
			dist := LABDistance(pixel, cluster)
			if dist < minDist {
				minDist = dist
				bestColor = rgbColors[j]
			}
		}
		result[i] = bestColor
	}

	return result
}

// Palette generation methods
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

type PaletteType int

const (
	Complementary PaletteType = iota
	Triadic
	Analogous
	SplitComplementary
	Tetradic
	Monochromatic
)

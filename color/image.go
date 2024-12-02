package color

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/nfnt/resize"
)

//go:embed fonts/WorkSans-Regular.ttf fonts/WorkSans-Bold.ttf
var fonts embed.FS

const (
	imageSize    = 1400 // Reduced from 1600
	padding      = 0
	textPadding  = 20
	baseFontSize = 42 // Slightly smaller font size
)

// PaletteImage represents configuration for generating a palette image
type PaletteImage struct {
	Colors      []Color
	Names       []string
	HexCodes    []string
	SourceImage image.Image // Optional source image

	// Optional text options
	ShowHexCodes bool
	ShowNames    bool
}

// hexToRGBA converts our Color to color.RGBA
func (c Color) ToRGBA() color.RGBA {
	return color.RGBA{c.R, c.G, c.B, 255}
}

// GeneratePaletteImage creates an image of the color palette
func GeneratePaletteImage(cfg PaletteImage) (image.Image, error) {
	// Create square context
	dc := gg.NewContext(imageSize, imageSize)

	// Fill background with white to ensure no transparency
	dc.SetColor(color.White)
	dc.Clear()

	// Load regular and bold fonts
	regularFontBytes, err := fonts.ReadFile("fonts/WorkSans-Regular.ttf")
	if err != nil {
		return nil, fmt.Errorf("failed to load regular font: %w", err)
	}

	boldFontBytes, err := fonts.ReadFile("fonts/WorkSans-Bold.ttf")
	if err != nil {
		return nil, fmt.Errorf("failed to load bold font: %w", err)
	}

	regularFont, err := truetype.Parse(regularFontBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse regular font: %w", err)
	}

	boldFont, err := truetype.Parse(boldFontBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bold font: %w", err)
	}

	// Calculate the appropriate font size based on number of colors
	fontSize := baseFontSize
	if len(cfg.Colors) > 5 {
		fontSize = baseFontSize * 5 / len(cfg.Colors)
		if fontSize < 24 { // minimum font size
			fontSize = 24
		}
	}

	// Update font sizes
	regularFace := truetype.NewFace(regularFont, &truetype.Options{
		Size: float64(fontSize),
	})
	boldFace := truetype.NewFace(boldFont, &truetype.Options{
		Size: float64(fontSize),
	})

	// Calculate color bar dimensions
	numColors := len(cfg.Colors)
	if numColors == 0 {
		return nil, fmt.Errorf("no colors provided")
	}

	var barHeight float64
	startY := 0.0

	if cfg.SourceImage != nil {
		// If we have a source image, draw it first
		if err := drawSourceImage(dc, cfg.SourceImage); err != nil {
			return nil, err
		}
		// Color bars take up bottom 1/4 of the image
		barHeight = float64(imageSize) / 4
		startY = float64(imageSize) - barHeight
	} else {
		// Without source image, color bars take up full height
		barHeight = float64(imageSize)
	}

	barWidth := float64(imageSize) / float64(numColors)

	// Draw color bars and text
	for i, color := range cfg.Colors {
		x := float64(i) * barWidth

		// Draw color bar
		dc.SetColor(color.ToRGBA())
		dc.DrawRectangle(x, startY, barWidth, barHeight)
		dc.Fill()

		// Draw text
		dc.SetColor(getContrastColor(color))

		// Calculate text positions
		hexY := startY + (barHeight * 0.33)        // Position hex code 1/3 down the bar
		nameStartY := hexY + float64(fontSize)*1.4 // Increased spacing between hex and name
		lineHeight := float64(fontSize) * 1.2      // Consistent line height for wrapped lines

		// Draw hex code (without #) at fixed Y position with bold font
		hexText := cfg.HexCodes[i]
		if len(hexText) > 0 && hexText[0] == '#' {
			hexText = hexText[1:]
		}

		if cfg.ShowHexCodes {
			// Switch to bold font for hex code
			dc.SetFontFace(boldFace)
			textWidth, _ := dc.MeasureString(hexText)
			textX := x + (barWidth-textWidth)/2
			dc.DrawString(hexText, textX, hexY)
		}

		if cfg.ShowNames {
			// Switch back to regular font for color name
			dc.SetFontFace(regularFace)

			// Draw color name if provided
			if i < len(cfg.Names) {
				wrappedLines := wrapText(dc, cfg.Names[i], barWidth*0.9)

				// Draw each line of the name
				textY := nameStartY
				for _, line := range wrappedLines {
					textWidth, _ := dc.MeasureString(line)
					textX := x + (barWidth-textWidth)/2
					dc.DrawString(line, textX, textY)
					textY += lineHeight
				}
			}
		}
	}

	return dc.Image(), nil
}

// drawSourceImage draws the source image in the top 3/4 of the canvas
func drawSourceImage(dc *gg.Context, img image.Image) error {
	bounds := img.Bounds()
	imgWidth := bounds.Max.X - bounds.Min.X
	imgHeight := bounds.Max.Y - bounds.Min.Y

	// Calculate target dimensions (top 3/4 of the canvas)
	targetHeight := float64(imageSize) * 0.75
	targetWidth := float64(imageSize)

	// Calculate scaling factors for both dimensions
	scaleX := targetWidth / float64(imgWidth)
	scaleY := targetHeight / float64(imgHeight)

	// Use the larger scale to ensure the image fills the space (cover)
	scale := math.Max(scaleX, scaleY)

	// Calculate scaled dimensions
	scaledWidth := float64(imgWidth) * scale
	scaledHeight := float64(imgHeight) * scale

	// Calculate cropping offsets to center the image
	x := (float64(imageSize) - scaledWidth) / 2
	y := (targetHeight - scaledHeight) / 2

	// Resize the image
	resized := resize.Resize(uint(scaledWidth), uint(scaledHeight), img, resize.Lanczos3)

	// Create a new context for cropping
	cropDC := gg.NewContext(int(targetWidth), int(targetHeight))

	// Draw the resized image at the calculated offset
	cropDC.DrawImage(resized, int(x), int(y))

	// Draw the cropped image onto the main context
	dc.DrawImage(cropDC.Image(), 0, 0)
	return nil
}

// getContrastColor returns white or black depending on which provides better contrast
func getContrastColor(c Color) color.Color {
	// Calculate relative luminance
	r := float64(c.R) / 255
	g := float64(c.G) / 255
	b := float64(c.B) / 255

	luminance := 0.2126*math.Pow(r, 2.2) + 0.7152*math.Pow(g, 2.2) + 0.0722*math.Pow(b, 2.2)

	if luminance > 0.5 {
		return color.Black
	}
	return color.White
}

// wrapText wraps text to fit within a given width, breaking on word boundaries
func wrapText(dc *gg.Context, text string, maxWidth float64) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	var currentLine []string
	var currentLineWidth float64

	// Use a smaller space width for tighter text
	spaceWidth, _ := dc.MeasureString(" ")
	spaceWidth *= 0.8 // Reduce space width by 20%

	for i, word := range words {
		wordWidth, _ := dc.MeasureString(word)

		if len(currentLine) == 0 {
			currentLine = append(currentLine, word)
			currentLineWidth = wordWidth
			if i == len(words)-1 {
				lines = append(lines, word)
			}
			continue
		}

		// Calculate width with reduced space
		newLineWidth := currentLineWidth + spaceWidth + wordWidth

		if newLineWidth <= maxWidth {
			currentLine = append(currentLine, word)
			currentLineWidth = newLineWidth
		} else {
			lines = append(lines, strings.Join(currentLine, " "))
			currentLine = []string{word}
			currentLineWidth = wordWidth
		}

		// Add last line if we're at the end
		if i == len(words)-1 {
			lines = append(lines, strings.Join(currentLine, " "))
		}
	}

	return lines
}

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
	Colors    []Color
	Names     []string
	HexCodes  []string
	InputPath string // Optional input image path

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

	if cfg.InputPath != "" {
		// If we have an input image, draw it first
		if err := drawInputImage(dc, cfg.InputPath); err != nil {
			return nil, err
		}
		// Color bars take up bottom 1/4 of the image
		barHeight = float64(imageSize) / 4
		startY = float64(imageSize) - barHeight
	} else {
		// Without input image, color bars take up full height
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

// drawInputImage draws the input image at the top of the context
func drawInputImage(dc *gg.Context, inputPath string) error {
	img, err := gg.LoadImage(inputPath)
	if err != nil {
		return fmt.Errorf("failed to load input image: %w", err)
	}

	// Calculate dimensions for 4:3 crop
	bounds := img.Bounds()
	imgWidth := float64(bounds.Dx())
	imgHeight := float64(bounds.Dy())

	var cropWidth, cropHeight float64
	if imgWidth/imgHeight > 4.0/3.0 {
		// Image is wider than 4:3
		cropHeight = imgHeight
		cropWidth = cropHeight * 4.0 / 3.0
	} else {
		// Image is taller than 4:3
		cropWidth = imgWidth
		cropHeight = cropWidth * 3.0 / 4.0
	}

	// Calculate crop offsets to center the crop
	cropX := (imgWidth - cropWidth) / 2
	cropY := (imgHeight - cropHeight) / 2

	// Create a new context for the cropped image
	croppedDC := gg.NewContext(int(cropWidth), int(cropHeight))
	croppedDC.DrawImage(img, int(-cropX), int(-cropY))

	// Scale the cropped image to fill the top 3/4 of the output image while maintaining aspect ratio
	availableHeight := float64(imageSize) * 3.0 / 4.0
	availableWidth := float64(imageSize)

	// Calculate scaling factors for both dimensions
	scaleX := availableWidth / cropWidth
	scaleY := availableHeight / cropHeight

	// Use the larger scale to ensure the image fills the space
	scale := math.Max(scaleX, scaleY)

	// Calculate final dimensions
	finalWidth := cropWidth * scale
	finalHeight := cropHeight * scale

	// Calculate position to center the image
	x := (availableWidth - finalWidth) / 2
	y := (availableHeight - finalHeight) / 2

	// Create a new context for the scaled image
	scaledDC := gg.NewContext(int(finalWidth), int(finalHeight))

	// Draw the cropped image scaled to fill the space
	scaledDC.Scale(scale, scale)
	scaledDC.DrawImage(croppedDC.Image(), 0, 0)

	// Draw the final image onto the main context
	dc.DrawImage(scaledDC.Image(), int(x), int(y))
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

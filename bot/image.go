package bot

import (
	"image"
	"math"

	"github.com/nfnt/resize"
)

const (
	maxWidth  = 1600
	maxHeight = 1600
)

// resizeImage resizes an image maintaining aspect ratio
// to fit within maxWidth x maxHeight bounds
func resizeImage(img image.Image) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// If image is already small enough, return as is
	if width <= maxWidth && height <= maxHeight {
		return img
	}

	// Calculate scaling factor to fit within bounds
	widthRatio := float64(maxWidth) / float64(width)
	heightRatio := float64(maxHeight) / float64(height)
	ratio := math.Min(widthRatio, heightRatio)

	newWidth := uint(float64(width) * ratio)
	newHeight := uint(float64(height) * ratio)

	// Resize using Lanczos resampling
	return resize.Resize(newWidth, newHeight, img, resize.Lanczos3)
}

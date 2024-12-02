package image

import (
	"bytes"
	stdimage "image"
	"image/jpeg"
	"math"

	"github.com/nfnt/resize"
	"github.com/watzon/pigmentpoet/bot/config"
)

// Handler handles image processing operations
type Handler struct {
	config *config.Config
}

// NewHandler creates a new image handler
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{
		config: cfg,
	}
}

// Resize resizes an image maintaining aspect ratio
// to fit within maxWidth x maxHeight bounds
func (h *Handler) Resize(img stdimage.Image) stdimage.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// If image is already small enough, return as is
	if width <= h.config.MaxWidth && height <= h.config.MaxHeight {
		return img
	}

	// Calculate scaling factor to fit within bounds
	widthRatio := float64(h.config.MaxWidth) / float64(width)
	heightRatio := float64(h.config.MaxHeight) / float64(height)
	ratio := math.Min(widthRatio, heightRatio)

	newWidth := uint(float64(width) * ratio)
	newHeight := uint(float64(height) * ratio)

	// Resize using Lanczos resampling
	return resize.Resize(newWidth, newHeight, img, resize.Lanczos3)
}

// ToJPEG converts an image to JPEG bytes
func (h *Handler) ToJPEG(img stdimage.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

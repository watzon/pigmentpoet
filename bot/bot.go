package bot

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"math/rand"
	"os"

	"github.com/watzon/lining/client"
	"github.com/watzon/lining/models"
	"github.com/watzon/pigmentpoet/color"
)

// Bot represents a Bluesky bot that posts color palettes
type Bot struct {
	client     *client.BskyClient
	matcher    *color.ColorMatcher
	outputDir  string
	paletteGen *PaletteGenerator
}

// PaletteGenerator handles the generation of color palettes
type PaletteGenerator struct {
	types []color.PaletteType
}

// NewBot creates a new instance of the Bot
func NewBot(ctx context.Context, identifier, password, outputDir string) (*Bot, error) {
	bsky, err := client.NewClient(client.DefaultConfig().
		WithHandle(identifier).
		WithAPIKey(password))
	if err != nil {
		return nil, fmt.Errorf("failed to create Bluesky client: %w", err)
	}

	err = bsky.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to login to Bluesky: %w", err)
	}

	matcher, err := color.NewPreloadedColorMatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create color matcher: %w", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	paletteGen := &PaletteGenerator{
		types: []color.PaletteType{
			color.Complementary,
			color.Triadic,
			color.Analogous,
			color.SplitComplementary,
			color.Tetradic,
			color.Monochromatic,
		},
	}

	return &Bot{
		client:     bsky,
		matcher:    matcher,
		outputDir:  outputDir,
		paletteGen: paletteGen,
	}, nil
}

// RefreshSession attempts to refresh the bot's authentication session
func (b *Bot) RefreshSession(ctx context.Context) error {
	if err := b.client.RefreshSession(ctx); err != nil {
		// If refresh fails, try to reconnect
		if err := b.client.Connect(ctx); err != nil {
			return fmt.Errorf("failed to establish valid session: %w", err)
		}
	}
	return nil
}

// generateRandomColor generates a random hex color
func (b *Bot) generateRandomColor() string {
	return fmt.Sprintf("#%02X%02X%02X",
		rand.Intn(256),
		rand.Intn(256),
		rand.Intn(256))
}

// GenerateAndPost generates a color palette and posts it to Bluesky
func (b *Bot) GenerateAndPost(ctx context.Context) error {
	// Ensure we have a valid session before proceeding
	if err := b.RefreshSession(ctx); err != nil {
		return fmt.Errorf("failed to refresh session: %w", err)
	}

	// Generate a random base color
	baseColor := b.generateRandomColor()

	// Select a random palette type
	paletteType := b.paletteGen.types[rand.Intn(len(b.paletteGen.types))]

	// Generate the palette
	colors := b.matcher.GeneratePalette(baseColor, paletteType, 5)

	// Convert colors to hex codes and get their names
	var hexCodes []string
	var names []string
	for _, c := range colors {
		hex := fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
		hexCodes = append(hexCodes, hex)
		if colorName, err := b.matcher.FindClosestColor(hex); err == nil {
			names = append(names, colorName.Name)
		} else {
			names = append(names, "Unknown")
		}
	}

	// Generate image
	cfg := color.PaletteImage{
		Colors:       colors,
		Names:        names,
		HexCodes:     hexCodes,
		ShowHexCodes: true,
		ShowNames:    true,
	}

	img, err := color.GeneratePaletteImage(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate palette image: %w", err)
	}

	uploadedImage, err := b.uploadImage(ctx, img)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	// Create post text
	text := fmt.Sprintf("🎨 %s\n\n", b.getPaletteTypeName(paletteType))
	for i, name := range names {
		text += fmt.Sprintf("%s (%s)\n", name, hexCodes[i])
	}

	fmt.Printf("Posting to Bluesky: %s\n", text)

	post, err := client.NewPostBuilder().
		AddText(text).
		AddTag("Color").
		AddTag("Design").
		AddTag("Art").
		WithImages([]models.UploadedImage{*uploadedImage}).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}

	// Post to Bluesky
	_, _, err = b.client.PostToFeed(ctx, post)
	if err != nil {
		return fmt.Errorf("failed to post to feed: %w", err)
	}

	return nil
}

// GenerateAndPostFromBing generates a color palette from Bing's image of the day and posts it
func (b *Bot) GenerateAndPostFromBing(ctx context.Context) error {
	// Ensure we have a valid session before proceeding
	if err := b.RefreshSession(ctx); err != nil {
		return fmt.Errorf("failed to refresh session: %w", err)
	}

	// Fetch Bing's image of the day
	img, title, _, err := getBingImageOfDay(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Bing image: %w", err)
	}

	// Extract palette from the image
	colors := color.ExtractPalette(img, 5)

	// Get color names and hex codes
	var names []string
	var hexCodes []string
	for _, c := range colors {
		hex := fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
		hexCodes = append(hexCodes, hex)
		if colorName, err := b.matcher.FindClosestColor(hex); err == nil {
			names = append(names, colorName.Name)
		} else {
			names = append(names, "Unknown")
		}
	}

	// Save Bing image to temporary file
	tmpFile, err := os.CreateTemp(b.outputDir, "bing-*.png")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Encode and save the image
	if err := jpeg.Encode(tmpFile, img, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("failed to save image to temporary file: %w", err)
	}
	tmpFile.Close()

	// Generate palette image with Bing image as input
	cfg := color.PaletteImage{
		Colors:       colors,
		Names:        names,
		HexCodes:     hexCodes,
		ShowHexCodes: true,
		ShowNames:    true,
		InputPath:    tmpFile.Name(),
	}

	paletteImg, err := color.GeneratePaletteImage(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate palette image: %w", err)
	}

	// Upload the palette image
	uploadedImage, err := b.uploadImage(ctx, paletteImg)
	if err != nil {
		return fmt.Errorf("failed to upload palette image: %w", err)
	}

	// Create post text
	text := fmt.Sprintf("🎨 %s\n\n", title)
	for i, name := range names {
		text += fmt.Sprintf("%s (%s)\n", name, hexCodes[i])
	}

	post, err := client.NewPostBuilder().
		AddText(text).
		AddTag("Color").
		AddTag("Bing").
		AddTag("Design").
		WithImages([]models.UploadedImage{*uploadedImage}).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}

	// Post to Bluesky
	_, _, err = b.client.PostToFeed(ctx, post)
	if err != nil {
		return fmt.Errorf("failed to post to feed: %w", err)
	}

	return nil
}

func (b *Bot) uploadImage(ctx context.Context, img image.Image) (*models.UploadedImage, error) {
	buf := new(bytes.Buffer)

	// Encode as JPEG instead of PNG for smaller file size
	err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 85})
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	imgData := models.Image{
		Title: "color palette",
		Data:  buf.Bytes(),
	}

	uploadedImage, err := b.client.UploadImage(ctx, imgData)
	if err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	return uploadedImage, nil
}

// getPaletteTypeName returns a human-readable name for the palette type
func (b *Bot) getPaletteTypeName(pt color.PaletteType) string {
	switch pt {
	case color.Complementary:
		return "Complementary"
	case color.Triadic:
		return "Triadic"
	case color.Analogous:
		return "Analogous"
	case color.SplitComplementary:
		return "Split Complementary"
	case color.Tetradic:
		return "Tetradic"
	case color.Monochromatic:
		return "Monochromatic"
	default:
		return "Unknown"
	}
}

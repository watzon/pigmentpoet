package bot

import (
	"bytes"
	"context"
	"fmt"
	stdimage "image"
	"os"

	"github.com/watzon/lining/firehose"
	"github.com/watzon/lining/post"
	"github.com/watzon/pigmentpoet/bot/config"
	"github.com/watzon/pigmentpoet/bot/image"
	"github.com/watzon/pigmentpoet/bot/palette"
	"github.com/watzon/pigmentpoet/bot/platform"
	"github.com/watzon/pigmentpoet/color"
)

// Bot represents a Bluesky bot that posts color palettes
type Bot struct {
	config     *config.Config
	bluesky    *platform.BlueskyClient
	imgHandler *image.Handler
	paletteGen *palette.Generator
}

// NewBot creates a new instance of the Bot
func NewBot(ctx context.Context, identifier, password, outputDir string) (*Bot, error) {
	cfg := config.DefaultConfig().
		WithHandle(identifier).
		WithPassword(password).
		WithOutputDir(outputDir)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	imgHandler := image.NewHandler(cfg)

	bluesky, err := platform.NewBlueskyClient(ctx, identifier, password, imgHandler)
	if err != nil {
		return nil, fmt.Errorf("failed to create Bluesky client: %w", err)
	}

	paletteGen, err := palette.NewGenerator()
	if err != nil {
		return nil, fmt.Errorf("failed to create palette generator: %w", err)
	}

	return &Bot{
		config:     cfg,
		bluesky:    bluesky,
		imgHandler: imgHandler,
		paletteGen: paletteGen,
	}, nil
}

// GenerateAndPost generates a color palette and posts it to Bluesky
func (b *Bot) GenerateAndPost(ctx context.Context, replyTo *post.Post) error {
	// Ensure we have a valid session before proceeding
	if err := b.bluesky.RefreshSession(ctx); err != nil {
		return fmt.Errorf("failed to refresh session: %w", err)
	}

	// Generate a random palette
	palette, err := b.paletteGen.GenerateRandomPalette()
	if err != nil {
		return fmt.Errorf("failed to generate palette: %w", err)
	}

	// Generate image
	img, err := palette.ToImage()
	if err != nil {
		return fmt.Errorf("failed to generate palette image: %w", err)
	}

	uploadedImage, err := b.bluesky.UploadImage(ctx, img)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	// Create post text
	text := fmt.Sprintf("🎨 %s\n\n", palette.TypeName)
	for i, name := range palette.Names {
		text += fmt.Sprintf("%s (%s)\n", name, palette.HexCodes[i])
	}

	// Create the post
	if err := b.bluesky.CreatePost(ctx, text, uploadedImage, replyTo); err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}

	return nil
}

// GenerateAndPostFromBing generates a color palette from Bing's image of the day and posts it
func (b *Bot) GenerateAndPostFromBing(ctx context.Context) error {
	// Ensure we have a valid session before proceeding
	if err := b.bluesky.RefreshSession(ctx); err != nil {
		return fmt.Errorf("failed to refresh session: %w", err)
	}

	// Fetch Bing's image of the day
	bingImg, err := b.imgHandler.GetBingImageOfDay()
	if err != nil {
		return fmt.Errorf("failed to get Bing image: %w", err)
	}

	// Extract palette from the image
	colors := color.ExtractPalette(bingImg.Image, 5)
	baseColor := fmt.Sprintf("#%02X%02X%02X", colors[0].R, colors[0].G, colors[0].B)
	palette, err := b.paletteGen.GeneratePalette(baseColor, color.Analogous)
	if err != nil {
		return fmt.Errorf("failed to generate palette: %w", err)
	}

	// Generate palette image
	img, err := palette.ToImage()
	if err != nil {
		return fmt.Errorf("failed to generate palette image: %w", err)
	}

	uploadedImage, err := b.bluesky.UploadImage(ctx, img)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	// Create post text
	text := fmt.Sprintf("🎨 Color palette from Bing's Image of the Day\n📷 %s\n\n", bingImg.Title)
	for i, name := range palette.Names {
		text += fmt.Sprintf("%s (%s)\n", name, palette.HexCodes[i])
	}
	text += fmt.Sprintf("\n📝 %s", bingImg.Copyright)

	// Create the post
	if err := b.bluesky.CreatePost(ctx, text, uploadedImage, nil); err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}

	return nil
}

// StartFirehoseListener starts listening to the Bluesky firehose for posts with #pigmentpoet
func (b *Bot) StartFirehoseListener(ctx context.Context) error {
	callbacks := &firehose.EnhancedFirehoseCallbacks{
		PostHandlers: []firehose.PostHandlerWithFilter{
			{
				Filters: []firehose.PostFilter{
					func(post *post.Post) bool {
						return post.HasTag("pigmentpoet")
					},
				},
				Handler: func(post *post.Post) error {
					if err := b.handleTaggedPost(ctx, post); err != nil {
						fmt.Printf("Error handling tagged post: %v\n", err)
					}
					return nil
				},
			},
		},
	}

	return b.bluesky.GetClient().SubscribeToFirehose(ctx, callbacks)
}

// handleTaggedPost processes a post tagged with #pigmentpoet
func (b *Bot) handleTaggedPost(ctx context.Context, p *post.Post) error {
	// Try to get image from the user's post first
	imgRef, did := b.findImageRef(p)
	postToReplyTo := p

	// If no image in user's post, try to get from replied post
	if imgRef == "" && p.ReplyRef != nil {
		// Get the replied-to post
		replyUri := p.ReplyRef.Parent.Uri
		repliedPost, err := b.bluesky.GetPost(ctx, replyUri)
		if err == nil {
			imgRef, did = b.findImageRef(repliedPost)
			if imgRef != "" {
				// If we found an image in the replied post, reply to that one
				postToReplyTo = repliedPost
			}
		}
	}

	// If still no image found, generate random palette
	if imgRef == "" {
		return b.GenerateAndPost(ctx, postToReplyTo)
	}

	// Get the image
	imgBytes, err := b.bluesky.GetBlob(ctx, imgRef, did)
	if err != nil {
		return fmt.Errorf("failed to download image: %w", err)
	}

	return b.generatePaletteFromImage(ctx, imgBytes, postToReplyTo)
}

// findImageRef tries to find an image reference in a post's embeds
// Returns the image reference and the DID of the post containing the image
func (b *Bot) findImageRef(p *post.Post) (string, string) {
	if p.Embed == nil {
		return "", ""
	}

	if len(p.Embed.Images) > 0 {
		return p.Embed.Images[0].Ref, p.Repo
	}

	if p.Embed.RecordWithMedia != nil && len(p.Embed.RecordWithMedia.Media.Images) > 0 {
		return p.Embed.RecordWithMedia.Media.Images[0].Ref, p.Repo
	}

	return "", ""
}

// generatePaletteFromImage generates a palette from the given image bytes
func (b *Bot) generatePaletteFromImage(ctx context.Context, imgBytes []byte, replyTo *post.Post) error {
	// Decode the image
	img, _, err := stdimage.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Extract palette from the image
	colors := color.ExtractPalette(img, 5)
	if len(colors) == 0 {
		return fmt.Errorf("failed to extract colors from image")
	}

	// Generate hex codes
	hexCodes := make([]string, len(colors))
	for i, c := range colors {
		hexCodes[i] = fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
	}

	// Create palette config
	cfg := color.PaletteImage{
		Colors:       colors,
		HexCodes:     hexCodes,
		ShowHexCodes: true,
		ShowNames:    true,
		SourceImage:  img,
	}

	// Generate palette image
	paletteImg, err := color.GeneratePaletteImage(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate palette image: %w", err)
	}

	uploadedImage, err := b.bluesky.UploadImage(ctx, paletteImg)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	// Create post text
	text := "🎨 Here's a color palette based on your image!\n\n"
	for _, hexCode := range hexCodes {
		text += fmt.Sprintf("%s\n", hexCode)
	}

	// Create the post
	if err := b.bluesky.CreatePost(ctx, text, uploadedImage, replyTo); err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}

	return nil
}

// RefreshSession attempts to refresh the bot's authentication session
func (b *Bot) RefreshSession(ctx context.Context) error {
	return b.bluesky.RefreshSession(ctx)
}

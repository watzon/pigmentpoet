package platform

import (
	"context"
	"fmt"
	stdimage "image"
	"time"

	comatprototypes "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/watzon/lining/client"
	"github.com/watzon/lining/config"
	"github.com/watzon/lining/models"
	"github.com/watzon/lining/post"
	"github.com/watzon/pigmentpoet/bot/image"
)

// BlueskyClient handles interactions with the Bluesky platform
type BlueskyClient struct {
	client     *client.BskyClient
	imgHandler *image.Handler
}

// NewBlueskyClient creates a new Bluesky client
func NewBlueskyClient(ctx context.Context, handle, password string, imgHandler *image.Handler) (*BlueskyClient, error) {
	bsky, err := client.NewClient(config.Default().
		WithHandle(handle).
		WithAPIKey(password))
	if err != nil {
		return nil, fmt.Errorf("failed to create Bluesky client: %w", err)
	}

	err = bsky.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to login to Bluesky: %w", err)
	}

	return &BlueskyClient{
		client:     bsky,
		imgHandler: imgHandler,
	}, nil
}

// RefreshSession attempts to refresh the client's authentication session
func (b *BlueskyClient) RefreshSession(ctx context.Context) error {
	if err := b.client.RefreshSession(ctx); err != nil {
		// If refresh fails, try to reconnect
		if err := b.client.Connect(ctx); err != nil {
			return fmt.Errorf("failed to establish valid session: %w", err)
		}
	}
	return nil
}

// GetAccessToken returns the current access token
func (b *BlueskyClient) GetAccessToken() string {
	return b.client.GetAccessToken()
}

// GetTimeout returns the configured timeout duration
func (b *BlueskyClient) GetTimeout() time.Duration {
	return b.client.GetTimeout()
}

// GetFirehoseURL returns the configured firehose URL
func (b *BlueskyClient) GetFirehoseURL() string {
	return b.client.GetFirehoseURL()
}

// UploadImage uploads an image to Bluesky
func (b *BlueskyClient) UploadImage(ctx context.Context, img stdimage.Image) (*models.UploadedImage, error) {
	// Convert image to JPEG bytes
	imgBytes, err := b.imgHandler.ToJPEG(img)
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	// Upload the image
	imgData := models.Image{
		Title: "color palette",
		Data:  imgBytes,
	}

	uploadedImage, err := b.client.UploadImage(ctx, imgData)
	if err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	return uploadedImage, nil
}

// CreatePost creates a new post with optional image and reply
func (b *BlueskyClient) CreatePost(ctx context.Context, text string, img *models.UploadedImage, replyTo *post.Post) error {
	builder := b.client.NewPostBuilder().
		AddText(text).
		AddTag("Color").
		AddTag("Design").
		AddTag("Art")

	if img != nil {
		builder = builder.WithImages([]models.UploadedImage{*img})
	}

	if replyTo != nil {
		// If we're replying to a post that is itself a reply, we need the root reference
		replyRef := &bsky.FeedPost_ReplyRef{
			Parent: &comatprototypes.RepoStrongRef{
				Uri: replyTo.Uri(),
				Cid: replyTo.Cid,
			},
		}

		// If the post we're replying to is itself a reply, use its root
		if replyTo.ReplyRef != nil && replyTo.ReplyRef.Root != nil {
			replyRef.Root = replyTo.ReplyRef.Root
		} else {
			// Otherwise use the parent as the root
			replyRef.Root = replyRef.Parent
		}

		builder = builder.WithReply(replyRef)
	}

	post, err := builder.Build()
	if err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}

	// Post to Bluesky
	_, _, err = b.client.PostToFeed(ctx, post)
	if err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}

	return nil
}

// GetBlob downloads a blob from Bluesky
func (b *BlueskyClient) GetBlob(ctx context.Context, ref string, did string) ([]byte, error) {
	data, _, err := b.client.DownloadBlob(ctx, ref, did)
	if err != nil {
		return nil, fmt.Errorf("failed to download blob: %w", err)
	}
	return data, nil
}

// GetPost retrieves a post by its URI
func (b *BlueskyClient) GetPost(ctx context.Context, uri string) (*post.Post, error) {
	post, err := b.client.GetPost(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}
	return post, nil
}

// GetClient returns the underlying Bluesky client
func (b *BlueskyClient) GetClient() *client.BskyClient {
	return b.client
}

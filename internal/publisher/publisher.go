// Package publisher provides the X (Twitter) API client for posting content
// and uploading media.
package publisher

import (
	"context"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// Publisher is the interface for posting content to X/Twitter.
type Publisher interface {
	// PublishText posts a text-only tweet and returns the result.
	PublishText(ctx context.Context, content string) (*PublishResult, error)
	// PublishWithMedia posts a tweet with attached media and returns the result.
	PublishWithMedia(ctx context.Context, content string, mediaIDs []string) (*PublishResult, error)
	// DeletePost removes a previously published tweet by its X post ID.
	DeletePost(ctx context.Context, xPostID string) error
}

// PublishResult holds the outcome of a publish operation.
type PublishResult struct {
	PostID    string              `json:"post_id"`
	Status    models.PublishStatus `json:"status"`
	Error     string              `json:"error,omitempty"`
	URL       string              `json:"url,omitempty"`
}

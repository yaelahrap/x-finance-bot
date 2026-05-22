// Package storage defines the persistence layer for the x-finance-bot.
//
// The Storage interface abstracts data access so the rest of the application
// remains decoupled from the underlying database. The MVP ships with a SQLite
// implementation in sqlite.go; a Cloudflare D1 implementation may follow.
package storage

import (
	"context"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// Storage is the data-access contract used by the bot's pipeline. All methods
// accept a context for cancellation and timeouts. "Not found" lookups return
// (nil, nil) rather than an error so callers can branch without sentinel checks.
type Storage interface {
	// Articles
	SaveArticle(ctx context.Context, article models.Article) error
	GetArticleByHash(ctx context.Context, hash string) (*models.Article, error)
	GetArticlesByStatus(ctx context.Context, status models.ArticleStatus, limit int) ([]models.Article, error)
	UpdateArticleStatus(ctx context.Context, id string, status models.ArticleStatus) error

	// Drafts
	SaveDraft(ctx context.Context, draft models.DraftPost) error
	GetPendingDrafts(ctx context.Context) ([]models.DraftPost, error)
	GetDraftByID(ctx context.Context, id string) (*models.DraftPost, error)
	UpdateDraftStatus(ctx context.Context, id string, status models.DraftStatus) error
	ApproveDraft(ctx context.Context, id string) error
	RejectDraft(ctx context.Context, id string) error

	// Published
	SavePublished(ctx context.Context, post models.PublishedPost) error
	GetPublishedPosts(ctx context.Context, limit, offset int) ([]models.PublishedPost, error)

	// Market
	SaveMarketSnapshot(ctx context.Context, snap models.MarketSnapshot) error
	GetLatestSnapshot(ctx context.Context, symbol string) (*models.MarketSnapshot, error)

	// Sources
	GetEnabledSources(ctx context.Context) ([]models.Source, error)
	SaveSource(ctx context.Context, source models.Source) error

	// Lifecycle
	Close() error
}

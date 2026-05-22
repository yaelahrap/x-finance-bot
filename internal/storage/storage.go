// Package storage defines the persistence layer for the x-finance-bot.
//
// The Storage interface abstracts data access so the rest of the application
// remains decoupled from the underlying database. The MVP ships with a SQLite
// implementation in sqlite.go; a Cloudflare D1 implementation may follow.
package storage

import (
	"context"
	"time"

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
	GetRecentArticles(ctx context.Context, limit int) ([]models.Article, error)
	UpdateArticleStatus(ctx context.Context, id string, status models.ArticleStatus) error

	// Drafts
	SaveDraft(ctx context.Context, draft models.DraftPost) error
	GetPendingDrafts(ctx context.Context) ([]models.DraftPost, error)
	GetDraftsByStatus(ctx context.Context, status models.DraftStatus, limit int) ([]models.DraftPost, error)
	GetDueScheduledDrafts(ctx context.Context, before time.Time) ([]models.DraftPost, error)
	GetDraftByID(ctx context.Context, id string) (*models.DraftPost, error)
	UpdateDraftStatus(ctx context.Context, id string, status models.DraftStatus) error
	ApproveDraft(ctx context.Context, id string) error
	RejectDraft(ctx context.Context, id string) error
	ScheduleDraft(ctx context.Context, id string, at time.Time) error

	// Published
	SavePublished(ctx context.Context, post models.PublishedPost) error
	GetPublishedPosts(ctx context.Context, limit, offset int) ([]models.PublishedPost, error)

	// Market
	SaveMarketSnapshot(ctx context.Context, snap models.MarketSnapshot) error
	GetLatestSnapshot(ctx context.Context, symbol string) (*models.MarketSnapshot, error)

	// Sources
	GetEnabledSources(ctx context.Context) ([]models.Source, error)
	SaveSource(ctx context.Context, source models.Source) error

	// Stats
	Counts(ctx context.Context) (Counts, error)

	// Lifecycle
	Close() error
}

// Counts is a snapshot of key tallies for dashboard summaries.
type Counts struct {
	Articles         int `json:"articles"`
	DraftsPending    int `json:"drafts_pending"`
	DraftsApproved   int `json:"drafts_approved"`
	DraftsScheduled  int `json:"drafts_scheduled"`
	DraftsRejected   int `json:"drafts_rejected"`
	DraftsPublished  int `json:"drafts_published"`
	PublishedSuccess int `json:"published_success"`
	PublishedFailed  int `json:"published_failed"`
	Sources          int `json:"sources"`
}

package models

import "time"

// ArticleStatus represents the lifecycle stage of an Article.
type ArticleStatus string

const (
	// ArticleStatusFetched indicates the article was retrieved from a source but not yet scored.
	ArticleStatusFetched ArticleStatus = "fetched"
	// ArticleStatusScored indicates the article has been scored for relevance.
	ArticleStatusScored ArticleStatus = "scored"
	// ArticleStatusDrafted indicates a draft post was generated from the article.
	ArticleStatusDrafted ArticleStatus = "drafted"
	// ArticleStatusPublished indicates the article's draft was published to X.
	ArticleStatusPublished ArticleStatus = "published"
	// ArticleStatusSkipped indicates the article was filtered out and will not be drafted.
	ArticleStatusSkipped ArticleStatus = "skipped"
)

// Article is a normalized news or market item ingested from a Source.
type Article struct {
	// ID is the stable unique identifier (UUID) for the article.
	ID string `json:"id"`
	// SourceID references the originating Source.
	SourceID string `json:"source_id"`
	// Title is the article headline.
	Title string `json:"title"`
	// URL is the canonical link to the original article.
	URL string `json:"url"`
	// Content is the full article body when available.
	Content string `json:"content,omitempty"`
	// Summary is a short summary, when provided by the source or extracted later.
	Summary string `json:"summary,omitempty"`
	// PublishedAt is the original publication timestamp from the source, if known.
	PublishedAt *time.Time `json:"published_at,omitempty"`
	// FetchedAt is when the bot retrieved the article.
	FetchedAt time.Time `json:"fetched_at"`
	// Hash is a content hash used for deduplication.
	Hash string `json:"hash"`
	// Category is an optional domain classification (e.g., "market", "crypto").
	Category string `json:"category,omitempty"`
	// Status is the current pipeline stage of the article.
	Status ArticleStatus `json:"status"`
}

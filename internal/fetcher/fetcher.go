// Package fetcher defines the Fetcher interface and provides implementations
// for various news/data source types.
package fetcher

import (
	"context"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// Fetcher is the interface for all data source fetchers.
type Fetcher interface {
	// Name returns a human-readable identifier for this fetcher.
	Name() string
	// Fetch retrieves articles from the source. It returns all new articles
	// found since the last fetch cycle.
	Fetch(ctx context.Context) ([]models.Article, error)
}

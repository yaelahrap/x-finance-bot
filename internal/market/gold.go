package market

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// GoldFetcher fetches the gold spot price.
type GoldFetcher struct {
	client HTTPClient
	apiURL string
}

// NewGoldFetcher creates a fetcher for gold prices.
func NewGoldFetcher(apiURL string, client HTTPClient) *GoldFetcher {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &GoldFetcher{client: client, apiURL: apiURL}
}

func (f *GoldFetcher) Name() string { return "market:gold" }

func (f *GoldFetcher) FetchQuotes(ctx context.Context) ([]models.MarketSnapshot, error) {
	var resp struct {
		Price         float64 `json:"price"`
		ChangePercent float64 `json:"change_percent"`
	}

	if err := fetchJSON(ctx, f.client, f.apiURL, &resp); err != nil {
		return nil, fmt.Errorf("gold fetch: %w", err)
	}

	snap := newSnapshot("GOLD", fmt.Sprintf("%.2f", resp.Price), "metals-api", resp.ChangePercent)
	return []models.MarketSnapshot{snap}, nil
}

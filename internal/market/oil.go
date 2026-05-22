package market

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// OilFetcher fetches crude oil prices.
type OilFetcher struct {
	client HTTPClient
	apiURL string
}

// NewOilFetcher creates a fetcher for oil prices.
func NewOilFetcher(apiURL string, client HTTPClient) *OilFetcher {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &OilFetcher{client: client, apiURL: apiURL}
}

func (f *OilFetcher) Name() string { return "market:oil" }

func (f *OilFetcher) FetchQuotes(ctx context.Context) ([]models.MarketSnapshot, error) {
	var resp struct {
		Price         float64 `json:"price"`
		ChangePercent float64 `json:"change_percent"`
	}

	if err := fetchJSON(ctx, f.client, f.apiURL, &resp); err != nil {
		return nil, fmt.Errorf("oil fetch: %w", err)
	}

	snap := newSnapshot("OIL", fmt.Sprintf("%.2f", resp.Price), "oil-api", resp.ChangePercent)
	return []models.MarketSnapshot{snap}, nil
}

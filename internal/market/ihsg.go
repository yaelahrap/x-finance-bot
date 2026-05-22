package market

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// IHSGFetcher fetches the Jakarta Composite Index (IHSG) data.
type IHSGFetcher struct {
	client HTTPClient
	apiURL string
}

// NewIHSGFetcher creates a fetcher for IHSG data.
func NewIHSGFetcher(apiURL string, client HTTPClient) *IHSGFetcher {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &IHSGFetcher{client: client, apiURL: apiURL}
}

func (f *IHSGFetcher) Name() string { return "market:ihsg" }

func (f *IHSGFetcher) FetchQuotes(ctx context.Context) ([]models.MarketSnapshot, error) {
	var resp struct {
		Value         float64 `json:"value"`
		ChangePercent float64 `json:"change_percent"`
	}

	if err := fetchJSON(ctx, f.client, f.apiURL, &resp); err != nil {
		return nil, fmt.Errorf("ihsg fetch: %w", err)
	}

	snap := newSnapshot("IHSG", fmt.Sprintf("%.2f", resp.Value), "idx", resp.ChangePercent)
	return []models.MarketSnapshot{snap}, nil
}

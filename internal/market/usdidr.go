package market

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// USDIDRFetcher fetches the USD/IDR exchange rate.
type USDIDRFetcher struct {
	client HTTPClient
	apiURL string
}

// NewUSDIDRFetcher creates a fetcher for USD/IDR rates.
// apiURL should be a free forex API endpoint (e.g., exchangerate-api.com).
func NewUSDIDRFetcher(apiURL string, client HTTPClient) *USDIDRFetcher {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &USDIDRFetcher{client: client, apiURL: apiURL}
}

func (f *USDIDRFetcher) Name() string { return "market:usdidr" }

func (f *USDIDRFetcher) FetchQuotes(ctx context.Context) ([]models.MarketSnapshot, error) {
	var resp struct {
		Rates map[string]float64 `json:"rates"`
		Base  string             `json:"base"`
	}

	if err := fetchJSON(ctx, f.client, f.apiURL, &resp); err != nil {
		return nil, fmt.Errorf("usdidr fetch: %w", err)
	}

	idr, ok := resp.Rates["IDR"]
	if !ok {
		return nil, fmt.Errorf("usdidr fetch: IDR rate not found in response")
	}

	snap := newSnapshot("USDIDR", fmt.Sprintf("%.2f", idr), "exchangerate-api", 0)
	return []models.MarketSnapshot{snap}, nil
}

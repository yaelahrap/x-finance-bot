package market

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// CryptoFetcher fetches cryptocurrency prices (BTC, ETH).
type CryptoFetcher struct {
	client HTTPClient
	apiURL string // CoinGecko-compatible API
}

// NewCryptoFetcher creates a fetcher for crypto prices.
// apiURL should point to a CoinGecko-compatible endpoint.
func NewCryptoFetcher(apiURL string, client HTTPClient) *CryptoFetcher {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if apiURL == "" {
		apiURL = "https://api.coingecko.com/api/v3/simple/price?ids=bitcoin,ethereum&vs_currencies=usd&include_24hr_change=true"
	}
	return &CryptoFetcher{client: client, apiURL: apiURL}
}

func (f *CryptoFetcher) Name() string { return "market:crypto" }

func (f *CryptoFetcher) FetchQuotes(ctx context.Context) ([]models.MarketSnapshot, error) {
	var resp map[string]struct {
		USD       float64 `json:"usd"`
		Change24h float64 `json:"usd_24h_change"`
	}

	if err := fetchJSON(ctx, f.client, f.apiURL, &resp); err != nil {
		return nil, fmt.Errorf("crypto fetch: %w", err)
	}

	var snapshots []models.MarketSnapshot

	if btc, ok := resp["bitcoin"]; ok {
		snapshots = append(snapshots, newSnapshot("BTC", fmt.Sprintf("%.2f", btc.USD), "coingecko", btc.Change24h))
	}
	if eth, ok := resp["ethereum"]; ok {
		snapshots = append(snapshots, newSnapshot("ETH", fmt.Sprintf("%.2f", eth.USD), "coingecko", eth.Change24h))
	}

	return snapshots, nil
}

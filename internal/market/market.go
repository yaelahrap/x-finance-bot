// Package market provides fetchers for financial market data including
// forex, crypto, commodities, and stock indices.
package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// Provider is the interface for market data providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string
	// FetchQuotes retrieves current market quotes for the configured symbols.
	FetchQuotes(ctx context.Context) ([]models.MarketSnapshot, error)
}

// HTTPClient is the minimal HTTP interface needed by market fetchers.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// fetchJSON is a helper that performs a GET request and unmarshals the response.
func fetchJSON(ctx context.Context, client HTTPClient, url string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "x-finance-bot/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return err
	}

	return json.Unmarshal(body, target)
}

// newSnapshot creates a MarketSnapshot with common fields populated.
func newSnapshot(symbol, value, source string, changePercent float64) models.MarketSnapshot {
	return models.MarketSnapshot{
		Symbol:        symbol,
		Value:         value,
		ChangePercent: changePercent,
		Source:        source,
		CapturedAt:    time.Now().UTC(),
	}
}

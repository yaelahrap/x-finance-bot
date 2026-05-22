package storage

import (
	"context"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// SeedSources inserts default sources if they do not exist.
func SeedSources(ctx context.Context, store Storage) error {
	defaultSources := []models.Source{
		{
			ID:               "bi-usdidr",
			Name:             "Bank Indonesia JISDOR",
			URL:              "https://www.bi.go.id/id/statistik/informasi-kurs/jisdor/default.aspx",
			Type:             "official",
			Category:         "market",
			ReliabilityScore: 10,
			Enabled:          true,
			CreatedAt:        time.Now().UTC(),
		},
		{
			ID:               "bmkg-gempa",
			Name:             "BMKG Auto Gempa",
			URL:              "https://data.bmkg.go.id/DataMKG/TEWS/autogempa.json",
			Type:             "official",
			Category:         "emergency",
			ReliabilityScore: 10,
			Enabled:          true,
			CreatedAt:        time.Now().UTC(),
		},
		{
			ID:               "cmc-crypto",
			Name:             "CoinMarketCap Crypto",
			URL:              "https://pro-api.coinmarketcap.com/v1/cryptocurrency/quotes/latest",
			Type:             "api",
			Category:         "crypto",
			ReliabilityScore: 10,
			Enabled:          true,
			CreatedAt:        time.Now().UTC(),
		},
		{
			ID:               "google-news-rupiah",
			Name:             "Google News - Indonesia Rupiah",
			URL:              "https://news.google.com/rss/search?q=Indonesia+rupiah&hl=en-US&gl=US&ceid=US:en",
			Type:             "aggregator",
			Category:         "news",
			ReliabilityScore: 5,
			Enabled:          true,
			CreatedAt:        time.Now().UTC(),
		},
		{
			ID:               "google-news-nickel",
			Name:             "Google News - Indonesia Nickel",
			URL:              "https://news.google.com/rss/search?q=Indonesia+nickel&hl=en-US&gl=US&ceid=US:en",
			Type:             "aggregator",
			Category:         "news",
			ReliabilityScore: 5,
			Enabled:          true,
			CreatedAt:        time.Now().UTC(),
		},
		{
			ID:               "cnbc-indonesia-rss",
			Name:             "CNBC Indonesia",
			URL:              "https://www.cnbcindonesia.com/market/rss",
			Type:             "rss",
			Category:         "news",
			ReliabilityScore: 8,
			Enabled:          true,
			CreatedAt:        time.Now().UTC(),
		},
		{
			ID:               "kontan-rss",
			Name:             "Kontan",
			URL:              "https://www.kontan.co.id/rss",
			Type:             "rss",
			Category:         "news",
			ReliabilityScore: 8,
			Enabled:          true,
			CreatedAt:        time.Now().UTC(),
		},
	}

	enabled, err := store.GetEnabledSources(ctx)
	if err != nil {
		return err
	}

	exists := make(map[string]bool)
	for _, src := range enabled {
		exists[src.ID] = true
	}

	for _, src := range defaultSources {
		if !exists[src.ID] {
			if err := store.SaveSource(ctx, src); err != nil {
				return err
			}
		}
	}

	return nil
}

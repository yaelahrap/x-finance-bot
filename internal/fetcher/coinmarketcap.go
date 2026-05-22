package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/raflyramadhan/x-finance-bot/internal/dedupe"
	"github.com/raflyramadhan/x-finance-bot/internal/models"
	"github.com/raflyramadhan/x-finance-bot/internal/storage"
)

type cmcQuote struct {
	Price            float64 `json:"price"`
	PercentChange24h float64 `json:"percent_change_24h"`
}

type cmcCoin struct {
	Symbol string              `json:"symbol"`
	Name   string              `json:"name"`
	Quote  map[string]cmcQuote `json:"quote"`
}

type cmcResponse struct {
	Data   map[string]cmcCoin `json:"data"`
	Status struct {
		ErrorCode int    `json:"error_code"`
		Message   string `json:"error_message"`
	} `json:"status"`
}

type CoinMarketCapFetcher struct {
	source models.Source
	client *http.Client
	apiKey string
	store  storage.Storage
}

func NewCoinMarketCapFetcher(source models.Source, client *http.Client, apiKey string, store storage.Storage) *CoinMarketCapFetcher {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &CoinMarketCapFetcher{
		source: source,
		client: client,
		apiKey: apiKey,
		store:  store,
	}
}

func (f *CoinMarketCapFetcher) Name() string {
	return f.source.Name
}

func (f *CoinMarketCapFetcher) Fetch(ctx context.Context) ([]models.Article, error) {
	if f.apiKey == "" {
		return nil, fmt.Errorf("coinmarketcap api key is missing")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.source.URL+"?symbol=BTC,ETH", nil)
	if err != nil {
		return nil, fmt.Errorf("cmc request: %w", err)
	}
	req.Header.Set("X-CMC_PRO_API_KEY", f.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cmc fetch do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cmc fetch status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("cmc fetch read: %w", err)
	}

	var res cmcResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("cmc json unmarshal: %w", err)
	}

	if res.Status.ErrorCode != 0 {
		return nil, fmt.Errorf("cmc api error: %s", res.Status.Message)
	}

	btc, hasBTC := res.Data["BTC"]
	eth, hasETH := res.Data["ETH"]
	if !hasBTC || !hasETH {
		return nil, fmt.Errorf("cmc response missing BTC or ETH data")
	}

	btcUSD, ok1 := btc.Quote["USD"]
	ethUSD, ok2 := eth.Quote["USD"]
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("cmc response missing USD quote data")
	}

	now := time.Now().UTC()

	if f.store != nil {
		btcSnap := models.MarketSnapshot{
			ID:            uuid.New().String(),
			Symbol:        "BTC",
			Value:         strconv.FormatFloat(btcUSD.Price, 'f', 2, 64),
			ChangePercent: btcUSD.PercentChange24h,
			Source:        "CoinMarketCap",
			CapturedAt:    now,
		}
		_ = f.store.SaveMarketSnapshot(ctx, btcSnap)

		ethSnap := models.MarketSnapshot{
			ID:            uuid.New().String(),
			Symbol:        "ETH",
			Value:         strconv.FormatFloat(ethUSD.Price, 'f', 2, 64),
			ChangePercent: ethUSD.PercentChange24h,
			Source:        "CoinMarketCap",
			CapturedAt:    now,
		}
		_ = f.store.SaveMarketSnapshot(ctx, ethSnap)
	}

	formatPrice := func(price float64) string {
		parts := strings.Split(fmt.Sprintf("%.2f", price), ".")
		intPart := parts[0]
		fracPart := parts[1]

		var result []string
		for len(intPart) > 3 {
			result = append([]string{intPart[len(intPart)-3:]}, result...)
			intPart = intPart[:len(intPart)-3]
		}
		if len(intPart) > 0 {
			result = append([]string{intPart}, result...)
		}
		return "$" + strings.Join(result, ",") + "." + fracPart
	}

	formatPercent := func(pct float64) string {
		sign := ""
		if pct > 0 {
			sign = "+"
		}
		return fmt.Sprintf("%s%.2f%%", sign, pct)
	}

	title := fmt.Sprintf("Crypto Update: BTC %s (%s), ETH %s (%s)",
		formatPrice(btcUSD.Price), formatPercent(btcUSD.PercentChange24h),
		formatPrice(ethUSD.Price), formatPercent(ethUSD.PercentChange24h),
	)

	content := fmt.Sprintf("Perkembangan harga aset kripto utama (24 jam terakhir):\n"+
		"- Bitcoin (BTC): %s (%s)\n"+
		"- Ethereum (ETH): %s (%s)\n\n"+
		"Data diperoleh dari CoinMarketCap.",
		formatPrice(btcUSD.Price), formatPercent(btcUSD.PercentChange24h),
		formatPrice(ethUSD.Price), formatPercent(ethUSD.PercentChange24h),
	)

	article := models.Article{
		ID:          uuid.New().String(),
		SourceID:    f.source.ID,
		Title:       title,
		URL:         "https://coinmarketcap.com",
		Content:     content,
		Summary:     content,
		PublishedAt: &now,
		FetchedAt:   now,
		Category:    "crypto",
		Status:      models.ArticleStatusFetched,
	}
	article.Hash = dedupe.ComputeHash(article.Title, article.URL, article.SourceID)

	return []models.Article{article}, nil
}

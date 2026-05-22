package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// APIFetcher fetches articles from a JSON API endpoint.
type APIFetcher struct {
	source    models.Source
	client    *http.Client
	parseFunc func(data []byte) ([]models.Article, error)
}

// NewAPIFetcher creates a new API fetcher. The parseFunc is responsible for
// converting the raw JSON response into articles.
func NewAPIFetcher(source models.Source, client *http.Client, parseFunc func([]byte) ([]models.Article, error)) *APIFetcher {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &APIFetcher{source: source, client: client, parseFunc: parseFunc}
}

// Name returns the fetcher's identifier.
func (f *APIFetcher) Name() string {
	return fmt.Sprintf("api:%s", f.source.Name)
}

// Fetch retrieves data from the API endpoint and parses it into articles.
func (f *APIFetcher) Fetch(ctx context.Context) ([]models.Article, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.source.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("api fetch %s: %w", f.source.Name, err)
	}
	req.Header.Set("User-Agent", "x-finance-bot/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api fetch %s: %w", f.source.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api fetch %s: status %d", f.source.Name, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("api fetch %s: read body: %w", f.source.Name, err)
	}

	if f.parseFunc != nil {
		return f.parseFunc(body)
	}

	// Default: try to parse as a generic news API response
	return parseGenericNewsAPI(body, f.source)
}

// genericNewsResponse is a common shape for news API responses.
type genericNewsResponse struct {
	Articles []struct {
		Title       string `json:"title"`
		URL         string `json:"url"`
		Description string `json:"description"`
		PublishedAt string `json:"publishedAt"`
		Source      struct {
			Name string `json:"name"`
		} `json:"source"`
	} `json:"articles"`
}

func parseGenericNewsAPI(data []byte, source models.Source) ([]models.Article, error) {
	var resp genericNewsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse generic news API: %w", err)
	}

	now := time.Now().UTC()
	var articles []models.Article
	for _, item := range resp.Articles {
		var pubTime *time.Time
		if t, err := time.Parse(time.RFC3339, item.PublishedAt); err == nil {
			t = t.UTC()
			pubTime = &t
		}

		articles = append(articles, models.Article{
			SourceID:    source.ID,
			Title:       item.Title,
			URL:         item.URL,
			Content:     item.Description,
			PublishedAt: pubTime,
			FetchedAt:   now,
			Category:    source.Category,
			Status:      models.ArticleStatusFetched,
		})
	}
	return articles, nil
}

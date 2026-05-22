package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/raflyramadhan/x-finance-bot/internal/dedupe"
	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

type GoogleNewsFetcher struct {
	source models.Source
	client *http.Client
}

func NewGoogleNewsFetcher(source models.Source, client *http.Client) *GoogleNewsFetcher {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &GoogleNewsFetcher{source: source, client: client}
}

func (f *GoogleNewsFetcher) Name() string {
	return fmt.Sprintf("google-news:%s", f.source.Name)
}

func (f *GoogleNewsFetcher) Fetch(ctx context.Context) ([]models.Article, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.source.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("google news request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google news fetch do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google news fetch status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("google news fetch read: %w", err)
	}

	feed, err := ParseRSS(body)
	if err != nil {
		return nil, fmt.Errorf("google news parse rss: %w", err)
	}

	now := time.Now().UTC()
	var wg sync.WaitGroup
	// Concurrency control: limit to 5 concurrent redirect resolutions
	sem := make(chan struct{}, 5)

	resolvedItems := make([]models.Article, len(feed.Items))

	for i, item := range feed.Items {
		pubTime := ParseRSSTime(item.PubDate)
		art := models.Article{
			ID:          uuid.New().String(),
			SourceID:    f.source.ID,
			Title:       item.Title,
			URL:         item.Link,
			Content:     item.Description,
			Summary:     item.Description,
			PublishedAt: pubTime,
			FetchedAt:   now,
			Category:    f.source.Category,
			Status:      models.ArticleStatusFetched,
		}

		resolvedItems[i] = art

		wg.Add(1)
		go func(idx int, targetURL string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			resolvedURL := f.resolveURL(ctx, targetURL)
			if resolvedURL != "" {
				resolvedItems[idx].URL = resolvedURL
			}
		}(i, item.Link)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Compute hashes after resolving URLs
	for i := range resolvedItems {
		resolvedItems[i].Hash = dedupe.ComputeHash(resolvedItems[i].Title, resolvedItems[i].URL, resolvedItems[i].SourceID)
	}

	return resolvedItems, nil
}

func (f *GoogleNewsFetcher) resolveURL(ctx context.Context, url string) string {
	resolveCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(resolveCtx, http.MethodGet, url, nil)
	if err != nil {
		return url
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	client := &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return url
	}
	defer resp.Body.Close()

	return resp.Request.URL.String()
}

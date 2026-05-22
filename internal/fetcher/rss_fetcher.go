package fetcher

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/raflyramadhan/x-finance-bot/internal/dedupe"
	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

// RSSFetcher fetches articles from an RSS/Atom feed.
type RSSFetcher struct {
	source models.Source
	client *http.Client
}

// NewRSSFetcher creates a new RSS fetcher for the given source.
func NewRSSFetcher(source models.Source, client *http.Client) *RSSFetcher {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &RSSFetcher{source: source, client: client}
}

// Name returns the fetcher's identifier.
func (f *RSSFetcher) Name() string {
	return fmt.Sprintf("rss:%s", f.source.Name)
}

// Fetch retrieves and parses the RSS feed, returning normalized articles.
func (f *RSSFetcher) Fetch(ctx context.Context) ([]models.Article, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.source.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("rss fetch %s: %w", f.source.Name, err)
	}
	req.Header.Set("User-Agent", "x-finance-bot/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rss fetch %s: %w", f.source.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rss fetch %s: status %d", f.source.Name, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit
	if err != nil {
		return nil, fmt.Errorf("rss fetch %s: read body: %w", f.source.Name, err)
	}

	feed, err := ParseRSS(body)
	if err != nil {
		return nil, fmt.Errorf("rss fetch %s: parse: %w", f.source.Name, err)
	}

	now := time.Now().UTC()
	var articles []models.Article
	for _, item := range feed.Items {
		pubTime := ParseRSSTime(item.PubDate)

		a := models.Article{
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
		a.Hash = dedupe.ComputeHash(a.Title, a.URL, a.SourceID)
		articles = append(articles, a)
	}

	return articles, nil
}

// RSS XML structures

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// atomFeed for Atom format support.
type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title   string   `xml:"title"`
	Link    atomLink `xml:"link"`
	Summary string   `xml:"summary"`
	Updated string   `xml:"updated"`
	ID      string   `xml:"id"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
}

func ParseRSS(data []byte) (*ParsedFeed, error) {
	// Try RSS 2.0 first
	var rss rssFeed
	if err := xml.Unmarshal(data, &rss); err == nil && len(rss.Channel.Items) > 0 {
		feed := &ParsedFeed{}
		for _, item := range rss.Channel.Items {
			feed.Items = append(feed.Items, ParsedItem{
				Title:       item.Title,
				Link:        item.Link,
				Description: item.Description,
				PubDate:     item.PubDate,
			})
		}
		return feed, nil
	}

	// Try Atom
	var atom atomFeed
	if err := xml.Unmarshal(data, &atom); err == nil && len(atom.Entries) > 0 {
		feed := &ParsedFeed{}
		for _, entry := range atom.Entries {
			feed.Items = append(feed.Items, ParsedItem{
				Title:       entry.Title,
				Link:        entry.Link.Href,
				Description: entry.Summary,
				PubDate:     entry.Updated,
			})
		}
		return feed, nil
	}

	return nil, fmt.Errorf("unable to parse feed as RSS or Atom")
}

type ParsedFeed struct {
	Items []ParsedItem
}

type ParsedItem struct {
	Title       string
	Link        string
	Description string
	PubDate     string
}

// ParseRSSTime attempts to parse common RSS date formats.
func ParseRSSTime(s string) *time.Time {
	if s == "" {
		return nil
	}

	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"2006-01-02 15:04:05",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

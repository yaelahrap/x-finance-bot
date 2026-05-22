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
)

type BMKGGempa struct {
	Tanggal     string `json:"Tanggal"`
	Jam         string `json:"Jam"`
	DateTime    string `json:"DateTime"`
	Coordinates string `json:"Coordinates"`
	Lintang     string `json:"Lintang"`
	Bujur       string `json:"Bujur"`
	Magnitude   string `json:"Magnitude"`
	Kedalaman   string `json:"Kedalaman"`
	Wilayah     string `json:"Wilayah"`
	Potensi     string `json:"Potensi"`
	Dirasakan   string `json:"Dirasakan"`
	Shakemap    string `json:"Shakemap"`
}

type BMKGResponse struct {
	Infogempa struct {
		Gempa BMKGGempa `json:"gempa"`
	} `json:"Infogempa"`
}

type BMKGFetcher struct {
	source models.Source
	client *http.Client
}

func NewBMKGFetcher(source models.Source, client *http.Client) *BMKGFetcher {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &BMKGFetcher{source: source, client: client}
}

func (f *BMKGFetcher) Name() string {
	return f.source.Name
}

func (f *BMKGFetcher) Fetch(ctx context.Context) ([]models.Article, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.source.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("bmkg request: %w", err)
	}
	req.Header.Set("User-Agent", "x-finance-bot/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bmkg fetch do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bmkg fetch status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("bmkg fetch read: %w", err)
	}

	var res BMKGResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("bmkg json unmarshal: %w", err)
	}

	gempa := res.Infogempa.Gempa
	if gempa.DateTime == "" {
		return nil, nil
	}

	mag, err := strconv.ParseFloat(gempa.Magnitude, 64)
	if err != nil {
		return nil, fmt.Errorf("bmkg parse magnitude %q: %w", gempa.Magnitude, err)
	}

	isFelt := strings.TrimSpace(gempa.Dirasakan) != "" && gempa.Dirasakan != "-"
	if mag < 5.0 && !isFelt {
		return nil, nil
	}

	publishedTime, err := time.Parse(time.RFC3339, gempa.DateTime)
	var publishedAt *time.Time
	if err == nil {
		publishedAt = &publishedTime
	}

	urgency := ""
	if mag >= 5.0 {
		urgency = "⚠️ [BREAKING] "
	}
	title := fmt.Sprintf("%sInfo Gempa Mag: %s - %s", urgency, gempa.Magnitude, gempa.Wilayah)

	var sb strings.Builder
	sb.WriteString("Info Gempa Terkini BMKG:\n")
	sb.WriteString(fmt.Sprintf("- Waktu: %s (%s)\n", gempa.Tanggal, gempa.Jam))
	sb.WriteString(fmt.Sprintf("- Magnitudo: %s\n", gempa.Magnitude))
	sb.WriteString(fmt.Sprintf("- Kedalaman: %s\n", gempa.Kedalaman))
	sb.WriteString(fmt.Sprintf("- Koordinat: %s, %s\n", gempa.Lintang, gempa.Bujur))
	sb.WriteString(fmt.Sprintf("- Wilayah: %s\n", gempa.Wilayah))
	sb.WriteString(fmt.Sprintf("- Potensi: %s\n", gempa.Potensi))
	if isFelt {
		sb.WriteString(fmt.Sprintf("- Dirasakan: %s\n", gempa.Dirasakan))
	}

	content := sb.String()

	article := models.Article{
		ID:          uuid.New().String(),
		SourceID:    f.source.ID,
		Title:       title,
		URL:         "https://www.bmkg.go.id",
		Content:     content,
		Summary:     content,
		PublishedAt: publishedAt,
		FetchedAt:   time.Now().UTC(),
		Category:    "emergency",
		Status:      models.ArticleStatusFetched,
	}
	article.Hash = dedupe.ComputeHash(article.Title, article.URL, article.SourceID)

	return []models.Article{article}, nil
}

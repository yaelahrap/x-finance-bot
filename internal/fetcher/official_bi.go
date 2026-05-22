package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/raflyramadhan/x-finance-bot/internal/dedupe"
	"github.com/raflyramadhan/x-finance-bot/internal/models"
	"github.com/raflyramadhan/x-finance-bot/internal/storage"
)

var (
	biRowRegex = regexp.MustCompile(`<tr>\s*<td[^>]*>([^<]+)</td>\s*<td[^>]*>([^<]+)</td>\s*</tr>`)
	biMonths   = map[string]string{
		"januari":   "January",
		"februari":  "February",
		"maret":     "March",
		"april":     "April",
		"mei":       "May",
		"juni":      "June",
		"juli":      "July",
		"agustus":   "August",
		"september": "September",
		"oktober":   "October",
		"november":  "November",
		"desember":  "December",
	}
)

type BIFetcher struct {
	source models.Source
	client *http.Client
	store  storage.Storage
}

func NewBIFetcher(source models.Source, client *http.Client, store storage.Storage) *BIFetcher {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &BIFetcher{
		source: source,
		client: client,
		store:  store,
	}
}

func (f *BIFetcher) Name() string {
	return f.source.Name
}

func (f *BIFetcher) Fetch(ctx context.Context) ([]models.Article, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.source.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("bi request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bi fetch do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bi fetch status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("bi fetch read: %w", err)
	}

	html := string(body)
	matches := biRowRegex.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("bi fetch: no exchange rate rows found in html")
	}

	// We only need the latest (first match in tbody)
	latestMatch := matches[0]
	rawDate := strings.TrimSpace(latestMatch[1])
	rawRate := strings.TrimSpace(latestMatch[2])

	// Parse date
	parsedTime, err := parseBIDate(rawDate)
	if err != nil {
		return nil, fmt.Errorf("bi parse date %q: %w", rawDate, err)
	}

	// Clean and parse rate
	// Rp17.717,00 -> 17717.00
	cleanedRate := strings.ReplaceAll(rawRate, "Rp", "")
	cleanedRate = strings.ReplaceAll(cleanedRate, ".", "")
	cleanedRate = strings.ReplaceAll(cleanedRate, ",", ".")
	cleanedRate = strings.TrimSpace(cleanedRate)

	rateVal, err := strconv.ParseFloat(cleanedRate, 64)
	if err != nil {
		return nil, fmt.Errorf("bi parse rate %q: %w", rawRate, err)
	}

	var changePercent float64
	var prevStr string
	if f.store != nil {
		prev, err := f.store.GetLatestSnapshot(ctx, "USDIDR")
		if err == nil && prev != nil && prev.Value != "" {
			prevVal, err := strconv.ParseFloat(prev.Value, 64)
			if err == nil && prevVal > 0 {
				changePercent = ((rateVal - prevVal) / prevVal) * 100
				prevStr = fmt.Sprintf(" (sebelumnya Rp%s)", formatIndonesianNumber(prevVal))
			}
		}

		// Save snapshot
		snap := models.MarketSnapshot{
			ID:            uuid.New().String(),
			Symbol:        "USDIDR",
			Value:         cleanedRate,
			ChangePercent: changePercent,
			Source:        "BI JISDOR",
			CapturedAt:    time.Now().UTC(),
		}
		_ = f.store.SaveMarketSnapshot(ctx, snap)
	}

	// Format output messages
	changeSign := ""
	if changePercent > 0 {
		changeSign = "+"
	}
	changeStr := ""
	if changePercent != 0 {
		changeStr = fmt.Sprintf(" (%s%.2f%%)", changeSign, changePercent)
	}

	title := fmt.Sprintf("Kurs JISDOR BI: USD/IDR di Level Rp%s%s - %s", formatIndonesianNumber(rateVal), changeStr, rawDate)
	content := fmt.Sprintf("Bank Indonesia menetapkan Jakarta Interbank Spot Dollar Rate (JISDOR) sebesar Rp%s per USD%s pada tanggal %s.",
		formatIndonesianNumber(rateVal), prevStr, rawDate)

	article := models.Article{
		ID:          uuid.New().String(),
		SourceID:    f.source.ID,
		Title:       title,
		URL:         f.source.URL,
		Content:     content,
		Summary:     content,
		PublishedAt: parsedTime,
		FetchedAt:   time.Now().UTC(),
		Category:    "market",
		Status:      models.ArticleStatusFetched,
	}
	article.Hash = dedupe.ComputeHash(article.Title, article.URL, article.SourceID)

	return []models.Article{article}, nil
}

func parseBIDate(rawDate string) (*time.Time, error) {
	parts := strings.Fields(strings.ToLower(rawDate))
	if len(parts) != 3 {
		return nil, fmt.Errorf("unexpected date format")
	}

	engMonth, ok := biMonths[parts[1]]
	if !ok {
		return nil, fmt.Errorf("unknown Indonesian month %q", parts[1])
	}

	engDateStr := fmt.Sprintf("%s %s %s", parts[0], engMonth, parts[2])
	t, err := time.Parse("2 January 2006", engDateStr)
	if err != nil {
		return nil, err
	}
	utcTime := t.UTC()
	return &utcTime, nil
}

func formatIndonesianNumber(val float64) string {
	intPart := int64(val)
	fracPart := int64((val - float64(intPart)) * 100)

	s := strconv.FormatInt(intPart, 10)
	var result []string
	for len(s) > 3 {
		result = append([]string{s[len(s)-3:]}, result...)
		s = s[:len(s)-3]
	}
	if len(s) > 0 {
		result = append([]string{s}, result...)
	}

	res := strings.Join(result, ".")
	if fracPart > 0 {
		res += fmt.Sprintf(",%02d", fracPart)
	}
	return res
}

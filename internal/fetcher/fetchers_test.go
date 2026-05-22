package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raflyramadhan/x-finance-bot/internal/models"
)

func TestBIFetcher_Fetch(t *testing.T) {
	mockHTML := `<html>
<body>
<div id="tableData">
    <table>
        <tbody>
            <tr>
                <td class="text-center">22 Mei 2026</td>
                <td class="text-center">Rp17.717,00</td>
            </tr>
            <tr>
                <td class="text-center">21 Mei 2026</td>
                <td class="text-center">Rp17.673,00</td>
            </tr>
        </tbody>
    </table>
</div>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockHTML))
	}))
	defer server.Close()

	src := models.Source{
		ID:       "bi-usdidr",
		Name:     "BI JISDOR",
		URL:      server.URL,
		Type:     "official",
		Category: "market",
	}

	fetcher := NewBIFetcher(src, server.Client(), nil)
	articles, err := fetcher.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}

	art := articles[0]
	if !strings.Contains(art.Title, "Rp17.717") {
		t.Errorf("expected title to contain Rp17.717, got %q", art.Title)
	}
	if !strings.Contains(art.Content, "22 Mei 2026") {
		t.Errorf("expected content to contain date, got %q", art.Content)
	}
}

func TestBMKGFetcher_Fetch(t *testing.T) {
	mockJSON := `{"Infogempa":{"gempa":{"Tanggal":"23 Mei 2026","Jam":"00:05:37 WIB","DateTime":"2026-05-22T17:05:37+00:00","Coordinates":"-2.29,140.49","Lintang":"2.29 LS","Bujur":"140.49 BT","Magnitude":"5.5","Kedalaman":"5 km","Wilayah":"Pusat gempa berada di laut 34 km Timur Laut Kab. Jayapura","Potensi":"Tidak berpotensi tsunami","Dirasakan":"III Jayapura"}}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockJSON))
	}))
	defer server.Close()

	src := models.Source{
		ID:       "bmkg-gempa",
		Name:     "BMKG Gempa",
		URL:      server.URL,
		Type:     "official",
		Category: "emergency",
	}

	fetcher := NewBMKGFetcher(src, server.Client())
	articles, err := fetcher.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}

	art := articles[0]
	if !strings.Contains(art.Title, "Mag: 5.5") {
		t.Errorf("expected title to contain Mag: 5.5, got %q", art.Title)
	}
	if !strings.Contains(art.Content, "III Jayapura") {
		t.Errorf("expected content to contain felt info, got %q", art.Content)
	}
}

func TestCoinMarketCapFetcher_Fetch(t *testing.T) {
	mockJSON := `{"data":{"BTC":{"symbol":"BTC","name":"Bitcoin","quote":{"USD":{"price":76757.45,"percent_change_24h":-1.41}}},"ETH":{"symbol":"ETH","name":"Ethereum","quote":{"USD":{"price":2123.85,"percent_change_24h":-0.97}}}},"status":{"error_code":0,"error_message":null}}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockJSON))
	}))
	defer server.Close()

	src := models.Source{
		ID:       "cmc-crypto",
		Name:     "CMC Crypto",
		URL:      server.URL,
		Type:     "api",
		Category: "crypto",
	}

	fetcher := NewCoinMarketCapFetcher(src, server.Client(), "mock-key", nil)
	articles, err := fetcher.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}

	art := articles[0]
	if !strings.Contains(art.Title, "BTC $76,757.45") {
		t.Errorf("expected title to contain BTC price, got %q", art.Title)
	}
}

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/raflyramadhan/x-finance-bot/internal/config"
	cardgen "github.com/raflyramadhan/x-finance-bot/internal/image"
	"github.com/raflyramadhan/x-finance-bot/internal/models"
	"github.com/raflyramadhan/x-finance-bot/internal/storage"
	_ "modernc.org/sqlite"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := sql.Open("sqlite", cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	r2Client := storage.NewR2Client(storage.R2Config{
		Endpoint:        cfg.Cloudflare.R2Endpoint,
		AccessKeyID:     cfg.Cloudflare.R2AccessKeyID,
		SecretAccessKey: cfg.Cloudflare.R2SecretAccessKey,
		BucketMedia:     cfg.Cloudflare.R2BucketMedia,
		PublicURL:       cfg.Cloudflare.R2PublicURLMedia,
	})

	ctx := context.Background()

	// Query drafts lacking media_url
	rows, err := db.QueryContext(ctx, `
		SELECT d.id, a.title, d.content, d.review_json, s.name, s.category
		FROM draft_posts d
		JOIN articles a ON d.article_id = a.id
		JOIN sources s ON a.source_id = s.id
		WHERE d.media_url IS NULL OR d.media_url = ''
	`)
	if err != nil {
		log.Fatalf("Failed to query drafts: %v", err)
	}
	defer rows.Close()

	type Item struct {
		ID         string
		Title      string
		Content    string
		ReviewJSON string
		SrcName    string
		SrcCat     string
	}

	var items []Item
	for rows.Next() {
		var it Item
		var reviewJSON sql.NullString
		if err := rows.Scan(&it.ID, &it.Title, &it.Content, &reviewJSON, &it.SrcName, &it.SrcCat); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}
		if reviewJSON.Valid {
			it.ReviewJSON = reviewJSON.String
		}
		items = append(items, it)
	}

	if len(items) == 0 {
		fmt.Println("No drafts found that need image generation.")
		return
	}

	fmt.Printf("Found %d drafts to backfill card images for.\n", len(items))

	for i, it := range items {
		// Parse review JSON if available
		var reviewOutput models.ReviewResult
		if it.ReviewJSON != "" {
			_ = json.Unmarshal([]byte(it.ReviewJSON), &reviewOutput)
		}

		category := it.SrcCat
		if category == "" {
			category = reviewOutput.Category
		}
		normCat := strings.ToLower(strings.TrimSpace(category))

		// Check if category is eligible
		if !(normCat == "emergency" || normCat == "alert" || normCat == "darurat" ||
			normCat == "market" || normCat == "jisdor" || normCat == "kurs" ||
			normCat == "crypto" || normCat == "bitcoin" || normCat == "news") {
			fmt.Printf("[%d/%d] Skipping draft %s: category %q is not eligible\n", i+1, len(items), it.ID, category)
			continue
		}

		cardDetails := reviewOutput.WhyItMatters
		if cardDetails == "" {
			cardDetails = reviewOutput.SuggestedPost
		}
		if cardDetails == "" {
			cardDetails = it.Content
		}

		fmt.Printf("[%d/%d] Generating card image for draft %s (Category: %s)... ", i+1, len(items), it.ID, category)
		pngBytes, err := cardgen.GenerateCard(category, it.Title, cardDetails, it.SrcName)
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			continue
		}

		key := fmt.Sprintf("cards/%s.png", it.ID)
		publicURL, err := r2Client.Upload(ctx, key, pngBytes, "image/png")
		if err != nil {
			fmt.Printf("UPLOAD FAILED: %v\n", err)
			continue
		}

		_, err = db.ExecContext(ctx, "UPDATE draft_posts SET media_url = ? WHERE id = ?", publicURL, it.ID)
		if err != nil {
			fmt.Printf("DB UPDATE FAILED: %v\n", err)
			continue
		}

		fmt.Printf("SUCCESS! URL: %s\n", publicURL)
	}

	fmt.Println("Backfill complete.")
}

// Package models defines the core domain types used across the X finance bot.
package models

import "time"

// Source represents a configured upstream provider of articles or market data.
// Sources cover RSS feeds, official APIs, and emergency alert channels.
type Source struct {
	// ID is the stable unique identifier (UUID) for the source.
	ID string `json:"id"`
	// Name is the human-readable name of the source.
	Name string `json:"name"`
	// URL is the endpoint or feed URL.
	URL string `json:"url"`
	// Type indicates how the source is consumed: "rss", "api", or "official".
	Type string `json:"type"`
	// Category classifies the source domain: "market", "news", "crypto", or "emergency".
	Category string `json:"category"`
	// ReliabilityScore is an integer score (typically 0-100) used in confidence weighting.
	ReliabilityScore int `json:"reliability_score"`
	// Enabled toggles whether the source is currently polled.
	Enabled bool `json:"enabled"`
	// CreatedAt is the timestamp when the source was registered.
	CreatedAt time.Time `json:"created_at"`
}

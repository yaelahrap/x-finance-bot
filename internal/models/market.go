package models

import "time"

// MarketSnapshot is a point-in-time observation of a market symbol.
// Symbols include currency pairs (e.g., "USDIDR"), commodities ("GOLD", "OIL"),
// indices ("IHSG"), and crypto assets ("BTC").
type MarketSnapshot struct {
	// ID is the stable unique identifier (UUID) for the snapshot.
	ID string `json:"id"`
	// Symbol is the ticker or instrument identifier.
	Symbol string `json:"symbol"`
	// Value is the captured price or level, stored as a string to preserve precision.
	Value string `json:"value"`
	// ChangePercent is the percent change versus a prior reference, when available.
	ChangePercent float64 `json:"change_percent,omitempty"`
	// Source is the upstream provider identifier or name.
	Source string `json:"source,omitempty"`
	// CapturedAt is when the snapshot was recorded.
	CapturedAt time.Time `json:"captured_at"`
}

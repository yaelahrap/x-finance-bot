// Package dedupe provides article deduplication via content hashing and
// text similarity scoring.
package dedupe

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ComputeHash generates a deterministic hash for an article based on its
// title, URL, and source ID. Used for exact duplicate detection.
func ComputeHash(title, url, sourceID string) string {
	normalized := strings.ToLower(strings.TrimSpace(title)) + "|" +
		strings.ToLower(strings.TrimSpace(url)) + "|" +
		strings.TrimSpace(sourceID)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// ComputeContentHash generates a SHA-256 hash from the article content body.
// Used as a secondary dedup signal for content-level matching.
func ComputeContentHash(content string) string {
	normalized := strings.ToLower(strings.TrimSpace(content))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

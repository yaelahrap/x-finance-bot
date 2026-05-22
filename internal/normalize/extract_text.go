package normalize

import (
	"strings"
	"unicode/utf8"
)

// ExtractText extracts meaningful text content from raw HTML or mixed content.
// It strips tags, decodes entities, and returns clean prose.
func ExtractText(raw string) string {
	if raw == "" {
		return ""
	}

	// Strip HTML
	text := StripHTML(raw)

	// Clean up whitespace
	text = CleanText(text)

	// Remove very short results (likely just tag artifacts)
	if utf8.RuneCountInString(text) < 10 {
		return ""
	}

	return text
}

// ExtractTitle cleans a title string: removes surrounding quotes,
// trims whitespace, and normalizes internal spacing.
func ExtractTitle(raw string) string {
	if raw == "" {
		return ""
	}

	title := CleanText(raw)

	// Remove surrounding quotes if present
	title = strings.TrimPrefix(title, `"`)
	title = strings.TrimSuffix(title, `"`)
	title = strings.TrimPrefix(title, "'")
	title = strings.TrimSuffix(title, "'")

	return strings.TrimSpace(title)
}

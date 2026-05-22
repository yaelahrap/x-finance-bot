// Package normalize provides text cleaning and extraction utilities for
// converting raw fetched content into clean, normalized article text.
package normalize

import (
	"strings"
	"unicode"
)

// Article normalizes an article's content fields in place: cleans HTML,
// trims whitespace, and generates a summary if missing.
func Article(title, content string) (cleanTitle, cleanContent, summary string) {
	cleanTitle = CleanText(title)
	cleanContent = StripHTML(content)
	cleanContent = CleanText(cleanContent)

	if cleanContent != "" {
		summary = Summarize(cleanContent, 280)
	}
	return cleanTitle, cleanContent, summary
}

// StripHTML removes HTML tags from text, converting common entities and
// stripping all markup.
func StripHTML(s string) string {
	if s == "" {
		return ""
	}

	// Replace common HTML entities first
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
		"&nbsp;", " ",
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n",
		"</div>", "\n",
		"</li>", "\n",
	)
	s = replacer.Replace(s)

	// Strip remaining HTML tags
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}

	return b.String()
}

// CleanText normalizes whitespace: collapses multiple spaces/newlines,
// trims leading/trailing whitespace.
func CleanText(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}

	return strings.TrimSpace(b.String())
}

// Summarize truncates text to maxLen characters at a word boundary,
// appending "..." if truncated.
func Summarize(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	// Find last space before maxLen-3 (room for "...")
	cutoff := maxLen - 3
	if cutoff <= 0 {
		return text[:maxLen]
	}

	lastSpace := strings.LastIndex(text[:cutoff], " ")
	if lastSpace <= 0 {
		return text[:cutoff] + "..."
	}

	return text[:lastSpace] + "..."
}

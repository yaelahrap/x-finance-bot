package dedupe

import (
	"strings"
	"unicode"
)

// Similarity computes a normalized similarity score between two texts using
// trigram-based Jaccard similarity. Returns a value between 0.0 (completely
// different) and 1.0 (identical).
func Similarity(a, b string) float64 {
	aNorm := normalize(a)
	bNorm := normalize(b)

	if aNorm == bNorm {
		return 1.0
	}
	if aNorm == "" || bNorm == "" {
		return 0.0
	}

	aGrams := trigrams(aNorm)
	bGrams := trigrams(bNorm)

	if len(aGrams) == 0 && len(bGrams) == 0 {
		return 1.0
	}
	if len(aGrams) == 0 || len(bGrams) == 0 {
		return 0.0
	}

	intersection := 0
	for gram := range aGrams {
		if _, ok := bGrams[gram]; ok {
			intersection++
		}
	}

	union := len(aGrams) + len(bGrams) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// IsDuplicate returns true if the similarity between two texts exceeds
// the given threshold. A threshold of 0.85 is recommended for news headline
// near-duplicate detection.
func IsDuplicate(a, b string, threshold float64) bool {
	return Similarity(a, b) >= threshold
}

// normalize lowercases text, removes punctuation, and collapses whitespace.
func normalize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
		} else if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// trigrams generates the set of character trigrams from a string.
func trigrams(s string) map[string]struct{} {
	runes := []rune(s)
	if len(runes) < 3 {
		result := make(map[string]struct{})
		if len(runes) > 0 {
			result[string(runes)] = struct{}{}
		}
		return result
	}

	result := make(map[string]struct{}, len(runes)-2)
	for i := 0; i <= len(runes)-3; i++ {
		result[string(runes[i:i+3])] = struct{}{}
	}
	return result
}

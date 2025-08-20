package strx

import (
	"strings"
	"unicode"
)

// HasPrefixAny returns true if s has any of the given prefixes.
func HasPrefixAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// HasSuffixAny returns true if s has any of the given suffixes.
func HasSuffixAny(s string, suffixes ...string) bool {
	for _, sf := range suffixes {
		if strings.HasSuffix(s, sf) {
			return true
		}
	}
	return false
}

// Truncate cuts the string in a rune-safe way; max<=0 returns an empty string.
// If ellipsis is provided and truncation occurs, append it to the end.
func Truncate(s string, max int, ellipsis string) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if ellipsis == "" {
		return string(runes[:max])
	}
	if max < len([]rune(ellipsis)) {
        // Ellipsis length exceeds max; return direct truncation
        return string(runes[:max])
    }
	cut := max - len([]rune(ellipsis))
	if cut < 0 {
		cut = 0
	}
	return string(runes[:cut]) + ellipsis
}

// TrimSpaceAndCompress trims leading/trailing spaces and compresses internal whitespace to single spaces.
func TrimSpaceAndCompress(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

// SplitAndTrim splits by sep, trims items, and removes empty parts.
func SplitAndTrim(s, sep string) []string {
	if sep == "" {
		return []string{strings.TrimSpace(s)}
	}
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

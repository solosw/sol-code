package websearch

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var stripTagsRe = regexp.MustCompile(`<[^>]*>`)

// NormalizeURL unquotes a URL and replaces spaces with '+'.
func NormalizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.QueryUnescape(rawURL)
	if err != nil {
		u = rawURL
	}
	return strings.ReplaceAll(u, " ", "+")
}

// NormalizeText strips HTML tags, unescapes entities, normalizes Unicode,
// removes control characters, and collapses whitespace.
func NormalizeText(raw string) string {
	if raw == "" {
		return ""
	}
	// 1. Strip HTML tags
	text := stripTagsRe.ReplaceAllString(raw, "")
	// 2. Unescape HTML entities
	text = html.UnescapeString(text)
	// 3. Unicode NFC normalization
	text = norm.NFC.String(text)
	// 4. Remove control characters
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
	// 5. Collapse whitespace
	return strings.Join(strings.Fields(text), " ")
}

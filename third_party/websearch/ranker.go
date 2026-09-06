package websearch

import (
	"regexp"
	"strings"
)

// Ranker sorts search results by relevance to the query.
// This is a direct port of ddgs SimpleFilterRanker.
type Ranker struct {
	minTokenLength int
	splitter       *regexp.Regexp
}

// NewRanker creates a new ranker with default settings.
func NewRanker() *Ranker {
	return &Ranker{
		minTokenLength: 3,
		splitter:       regexp.MustCompile(`\W+`),
	}
}

// Rank sorts results by query relevance:
// 1. Wikipedia results pinned to top
// 2. Both title & body contain query tokens
// 3. Title only
// 4. Body only
// 5. Neither
func (r *Ranker) Rank(results []SearchResult, query string) []SearchResult {
	tokens := r.extractTokens(query)

	var wiki, both, titleOnly, bodyOnly, neither []SearchResult

	for _, doc := range results {
		href := doc.Href()
		title := doc.Title()
		body := doc.Body()

		// Skip Wikimedia category pages
		if strings.Contains(title, "Category:") && strings.Contains(title, "Wikimedia") {
			continue
		}

		// Wikipedia results go to the top
		if strings.Contains(href, "wikipedia.org") {
			wiki = append(wiki, doc)
			continue
		}

		hitTitle := r.hasAnyToken(title, tokens)
		hitBody := r.hasAnyToken(body, tokens)

		switch {
		case hitTitle && hitBody:
			both = append(both, doc)
		case hitTitle:
			titleOnly = append(titleOnly, doc)
		case hitBody:
			bodyOnly = append(bodyOnly, doc)
		default:
			neither = append(neither, doc)
		}
	}

	result := make([]SearchResult, 0, len(results))
	result = append(result, wiki...)
	result = append(result, both...)
	result = append(result, titleOnly...)
	result = append(result, bodyOnly...)
	result = append(result, neither...)
	return result
}

func (r *Ranker) extractTokens(query string) []string {
	parts := r.splitter.Split(strings.ToLower(query), -1)
	var tokens []string
	for _, t := range parts {
		if len(t) >= r.minTokenLength {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

func (r *Ranker) hasAnyToken(text string, tokens []string) bool {
	lower := strings.ToLower(text)
	for _, tok := range tokens {
		if strings.Contains(lower, tok) {
			return true
		}
	}
	return false
}

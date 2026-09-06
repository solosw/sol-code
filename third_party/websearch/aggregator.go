package websearch

// ResultsAggregator deduplicates and counts search results by their primary key.
// Results with the same key but a longer body replace shorter ones.
// This mirrors ddgs ResultsAggregator.
type ResultsAggregator struct {
	cache   map[string]SearchResult
	counts  map[string]int
	order   []string
}

// NewAggregator creates a new results aggregator.
func NewAggregator() *ResultsAggregator {
	return &ResultsAggregator{
		cache:  make(map[string]SearchResult),
		counts: make(map[string]int),
	}
}

// Add appends a result to the aggregator, deduplicating by key.
func (a *ResultsAggregator) Add(result SearchResult) {
	key := result.DeduplicationKey()
	if key == "" {
		return
	}
	a.counts[key]++
	existing, exists := a.cache[key]
	if !exists || len(result.Body()) > len(existing.Body()) {
		a.cache[key] = result
		if !exists {
			a.order = append(a.order, key)
		}
	}
}

// AddAll adds multiple results.
func (a *ResultsAggregator) AddAll(results []SearchResult) {
	for _, r := range results {
		a.Add(r)
	}
}

// Len returns the number of unique results.
func (a *ResultsAggregator) Len() int {
	return len(a.cache)
}

// Results returns all results sorted by descending frequency count,
// preserving insertion order as a tiebreaker.
func (a *ResultsAggregator) Results() []SearchResult {
	// Sort by count descending, preserving insertion order
	type entry struct {
		key   string
		count int
		idx   int
	}
	entries := make([]entry, 0, len(a.order))
	for i, key := range a.order {
		entries = append(entries, entry{key: key, count: a.counts[key], idx: i})
	}
	// Stable sort: higher count first, then earlier insertion
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0; j-- {
			if entries[j].count > entries[j-1].count ||
				(entries[j].count == entries[j-1].count && entries[j].idx < entries[j-1].idx) {
				entries[j], entries[j-1] = entries[j-1], entries[j]
			} else {
				break
			}
		}
	}

	results := make([]SearchResult, 0, len(entries))
	for _, e := range entries {
		results = append(results, a.cache[e.key])
	}
	return results
}

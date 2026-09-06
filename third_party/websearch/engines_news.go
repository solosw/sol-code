package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// DuckDuckGoNewsEngine implements news search via DuckDuckGo's JSON API.
type DuckDuckGoNewsEngine struct{}

func NewDuckDuckGoNews() SearchEngine {
	return &DuckDuckGoNewsEngine{}
}

func (e *DuckDuckGoNewsEngine) Name() string       { return "duckduckgo" }
func (e *DuckDuckGoNewsEngine) Category() Category  { return CategoryNews }
func (e *DuckDuckGoNewsEngine) Provider() string    { return "bing" }
func (e *DuckDuckGoNewsEngine) Priority() float64   { return 1 }

func (e *DuckDuckGoNewsEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	client := httpClient(opts.Proxy, opts.Timeout)

	vqd, err := getVQD(ctx, client, query)
	if err != nil {
		return nil, fmt.Errorf("ddg news vqd: %w", err)
	}

	safesearchMap := map[string]string{"on": "1", "moderate": "-1", "off": "-2"}
	safesearch := safesearchMap[opts.SafeSearch]
	if safesearch == "" {
		safesearch = "-1"
	}

	params := url.Values{
		"l":     {opts.Region},
		"o":     {"json"},
		"noamp": {"1"},
		"q":     {query},
		"vqd":   {vqd},
		"p":     {safesearch},
	}

	if opts.TimeLimit != "" {
		params.Set("df", opts.TimeLimit)
	}
	if opts.Page > 1 {
		params.Set("s", itoa((opts.Page-1)*30))
	}

	body, err := doRequest(ctx, client, http.MethodGet, "https://duckduckgo.com/news.js", params, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("ddg news: %w", err)
	}

	var resp struct {
		Results []struct {
			Date    any    `json:"date"` // can be int or string
			Title   string `json:"title"`
			Excerpt string `json:"excerpt"`
			URL     string `json:"url"`
			Image   string `json:"image"`
			Source  string `json:"source"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ddg news parse: %w", err)
	}

	var results []SearchResult
	for _, item := range resp.Results {
		results = append(results, SearchResult{
			Category: CategoryNews,
			News: &NewsResult{
				Date:   normalizeDate(item.Date),
				Title:  NormalizeText(item.Title),
				Body:   NormalizeText(item.Excerpt),
				URL:    NormalizeURL(item.URL),
				Image:  NormalizeURL(item.Image),
				Source: NormalizeText(item.Source),
			},
		})
	}
	return results, nil
}

// normalizeDate converts an integer timestamp or string date to a string.
func normalizeDate(v any) string {
	switch t := v.(type) {
	case float64:
		// Unix timestamp — return as ISO string
		return fmt.Sprintf("%.0f", t)
	case int:
		return itoa(t)
	case string:
		return t
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(v)
	}
}

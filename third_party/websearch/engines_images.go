package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// DuckDuckGoImagesEngine implements image search via DuckDuckGo's JSON API.
type DuckDuckGoImagesEngine struct{}

func NewDuckDuckGoImages() SearchEngine {
	return &DuckDuckGoImagesEngine{}
}

func (e *DuckDuckGoImagesEngine) Name() string       { return "duckduckgo" }
func (e *DuckDuckGoImagesEngine) Category() Category  { return CategoryImages }
func (e *DuckDuckGoImagesEngine) Provider() string    { return "bing" }
func (e *DuckDuckGoImagesEngine) Priority() float64   { return 1 }

func (e *DuckDuckGoImagesEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	client := httpClient(opts.Proxy, opts.Timeout)

	// Step 1: get vqd token
	vqd, err := getVQD(ctx, client, query)
	if err != nil {
		return nil, fmt.Errorf("ddg images vqd: %w", err)
	}

	safesearchMap := map[string]string{"on": "1", "moderate": "1", "off": "-1"}
	safesearch := safesearchMap[opts.SafeSearch]
	if safesearch == "" {
		safesearch = "1"
	}

	params := url.Values{
		"o":   {"json"},
		"q":   {query},
		"l":   {opts.Region},
		"vqd": {vqd},
		"p":   {safesearch},
		"ct":  {"AT"},
	}

	if opts.TimeLimit != "" {
		tlMap := map[string]string{"d": "Day", "w": "Week", "m": "Month", "y": "Year"}
		if tl, ok := tlMap[opts.TimeLimit]; ok {
			params.Set("f", "time:"+tl+",,,,,")
		}
	}

	if opts.Page > 1 {
		params.Set("s", itoa((opts.Page-1)*100))
	}

	headers := map[string]string{
		"Accept":          "*/*",
		"Accept-Language": "en-US,en;q=0.5",
		"Referer":         "https://duckduckgo.com/",
		"Sec-GPC":         "1",
	}

	body, err := doRequest(ctx, client, http.MethodGet, "https://duckduckgo.com/i.js", params, nil, headers)
	if err != nil {
		return nil, fmt.Errorf("ddg images: %w", err)
	}

	var resp struct {
		Results []struct {
			Title     string `json:"title"`
			Image     string `json:"image"`
			Thumbnail string `json:"thumbnail"`
			URL       string `json:"url"`
			Height    any    `json:"height"` // can be int or string
			Width     any    `json:"width"`
			Source    string `json:"source"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ddg images parse: %w", err)
	}

	var results []SearchResult
	for _, item := range resp.Results {
		results = append(results, SearchResult{
			Category: CategoryImages,
			Image: &ImageResult{
				Title:     NormalizeText(item.Title),
				Image:     NormalizeURL(item.Image),
				Thumbnail: NormalizeURL(item.Thumbnail),
				URL:       NormalizeURL(item.URL),
				Height:    anyToString(item.Height),
				Width:     anyToString(item.Width),
				Source:    NormalizeText(item.Source),
			},
		})
	}
	return results, nil
}

// getVQD fetches the DuckDuckGo vqd token required for JSON API endpoints.
func getVQD(ctx context.Context, client *http.Client, query string) (string, error) {
	params := url.Values{"q": {query}}
	body, err := doRequest(ctx, client, http.MethodGet, "https://duckduckgo.com", params, nil, nil)
	if err != nil {
		return "", err
	}
	return extractVQD(body, query)
}

// extractVQD extracts the vqd token from DuckDuckGo HTML response.
func extractVQD(htmlBody []byte, query string) (string, error) {
	patterns := []struct {
		start []byte
		end   byte
	}{
		{[]byte(`vqd="`), '"'},
		{[]byte(`vqd=`), '&'},
		{[]byte(`vqd='`), '\''},
	}
	for _, p := range patterns {
		idx := bytes.Index(htmlBody, p.start)
		if idx < 0 {
			continue
		}
		start := idx + len(p.start)
		end := bytes.IndexByte(htmlBody[start:], p.end)
		if end < 0 {
			continue
		}
		return string(htmlBody[start : start+end]), nil
	}
	return "", fmt.Errorf("could not extract vqd for query %q", query)
}

func anyToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return itoa(int(t))
	case int:
		return itoa(t)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(v)
	}
}

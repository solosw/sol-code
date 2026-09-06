package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// DuckDuckGoVideosEngine implements video search via DuckDuckGo's JSON API.
type DuckDuckGoVideosEngine struct{}

func NewDuckDuckGoVideos() SearchEngine {
	return &DuckDuckGoVideosEngine{}
}

func (e *DuckDuckGoVideosEngine) Name() string       { return "duckduckgo" }
func (e *DuckDuckGoVideosEngine) Category() Category  { return CategoryVideos }
func (e *DuckDuckGoVideosEngine) Provider() string    { return "bing" }
func (e *DuckDuckGoVideosEngine) Priority() float64   { return 1 }

func (e *DuckDuckGoVideosEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	client := httpClient(opts.Proxy, opts.Timeout)

	vqd, err := getVQD(ctx, client, query)
	if err != nil {
		return nil, fmt.Errorf("ddg videos vqd: %w", err)
	}

	safesearchMap := map[string]string{"on": "1", "moderate": "-1", "off": "-2"}
	safesearch := safesearchMap[opts.SafeSearch]
	if safesearch == "" {
		safesearch = "-1"
	}

	filterParts := ""
	if opts.TimeLimit != "" {
		filterParts = "publishedAfter:" + opts.TimeLimit
	}

	params := url.Values{
		"l":   {opts.Region},
		"o":   {"json"},
		"q":   {query},
		"vqd": {vqd},
		"f":   {filterParts + ",,,"},
		"p":   {safesearch},
	}

	if opts.Page > 1 {
		params.Set("s", itoa((opts.Page-1)*60))
	}

	body, err := doRequest(ctx, client, http.MethodGet, "https://duckduckgo.com/v.js", params, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("ddg videos: %w", err)
	}

	var resp struct {
		Results []struct {
			Title       string            `json:"title"`
			Content     string            `json:"content"`
			Description string            `json:"description"`
			Duration    string            `json:"duration"`
			EmbedHTML   string            `json:"embed_html"`
			EmbedURL    string            `json:"embed_url"`
			ImageToken  string            `json:"image_token"`
			Images      map[string]string `json:"images"`
			Provider    string            `json:"provider"`
			Published   string            `json:"published"`
			Publisher   string            `json:"publisher"`
			Statistics  map[string]string `json:"statistics"`
			Uploader    string            `json:"uploader"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("ddg videos parse: %w", err)
	}

	var results []SearchResult
	for _, item := range resp.Results {
		results = append(results, SearchResult{
			Category: CategoryVideos,
			Video: &VideoResult{
				Title:       NormalizeText(item.Title),
				Content:     item.Content,
				Description: NormalizeText(item.Description),
				Duration:    item.Duration,
				EmbedHTML:   item.EmbedHTML,
				EmbedURL:    NormalizeURL(item.EmbedURL),
				ImageToken:  item.ImageToken,
				Images:      item.Images,
				Provider:    item.Provider,
				Published:   item.Published,
				Publisher:   NormalizeText(item.Publisher),
				Statistics:  item.Statistics,
				Uploader:    item.Uploader,
			},
		})
	}
	return results, nil
}

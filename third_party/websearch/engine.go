package websearch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// SearchEngine is the interface every backend must implement.
type SearchEngine interface {
	Name() string
	Category() Category
	Provider() string
	Priority() float64
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
}

// userAgents are rotated randomly for each request.
var userAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0",
}

var uaIdx int

func nextUserAgent() string {
	ua := userAgents[uaIdx%len(userAgents)]
	uaIdx++
	return ua
}

// httpClient builds an *http.Client with optional proxy and timeout.
func httpClient(proxy string, timeout int) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeout) * time.Second,
	}
}

// doRequest performs an HTTP request and returns the response body.
func doRequest(ctx context.Context, client *http.Client, method, rawURL string, params url.Values, data url.Values, extraHeaders map[string]string) ([]byte, error) {
	var body io.Reader
	if method == http.MethodPost && data != nil {
		body = strings.NewReader(data.Encode())
	}
	if method == http.MethodGet && params != nil {
		if strings.Contains(rawURL, "?") {
			rawURL += "&" + params.Encode()
		} else {
			rawURL += "?" + params.Encode()
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", nextUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	if method == http.MethodPost && data != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}
	return io.ReadAll(resp.Body)
}

// XPathEngine is the base implementation for HTML-scraping engines.
// It fetches a URL, parses HTML, runs XPath to extract items, then maps
// element XPaths to result fields. This mirrors ddgs BaseSearchEngine.
type XPathEngine struct {
	name     string
	category Category
	provider string
	priority float64

	SearchURL    string
	SearchMethod string // "GET" or "POST"
	ItemsXPath   string
	ElementsMap  map[string]string // field -> xpath
	PreProcess   func([]byte) []byte

	// BuildPayload returns (params, data, cookies) for the request.
	BuildPayload func(query string, opts SearchOptions) (params url.Values, data url.Values)
	// PostProcess filters/transforms results after extraction.
	PostProcess func(results []SearchResult) []SearchResult
	// ExtraHeaders returns additional headers for the request.
	ExtraHeaders map[string]string
	// SetCookies are set on the request.
	CookieBuilder func(opts SearchOptions) []*http.Cookie
}

func (e *XPathEngine) Name() string       { return e.name }
func (e *XPathEngine) Category() Category { return e.category }
func (e *XPathEngine) Provider() string   { return e.provider }
func (e *XPathEngine) Priority() float64  { return e.priority }

func (e *XPathEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	client := httpClient(opts.Proxy, opts.Timeout)

	var params, data url.Values
	if e.BuildPayload != nil {
		params, data = e.BuildPayload(query, opts)
	}

	body, err := doRequest(ctx, client, e.SearchMethod, e.SearchURL, params, data, e.ExtraHeaders)
	if err != nil {
		return nil, fmt.Errorf("engine %s: %w", e.name, err)
	}
	if e.PreProcess != nil {
		body = e.PreProcess(body)
	}

	results, err := e.extractResults(body)
	if err != nil {
		return nil, fmt.Errorf("engine %s extract: %w", e.name, err)
	}

	if e.PostProcess != nil {
		results = e.PostProcess(results)
	}
	return results, nil
}

func (e *XPathEngine) extractResults(htmlBody []byte) ([]SearchResult, error) {
	doc, err := html.Parse(bytes.NewReader(htmlBody))
	if err != nil {
		return nil, err
	}

	items := htmlquery.Find(doc, e.ItemsXPath)
	var results []SearchResult
	for _, item := range items {
		result := e.extractOneItem(item)
		if result != nil {
			results = append(results, *result)
		}
	}
	return results, nil
}

func (e *XPathEngine) extractOneItem(item *html.Node) *SearchResult {
	switch e.category {
	case CategoryText:
		r := &TextResult{}
		for field, xpath := range e.ElementsMap {
			val := xpathText(item, xpath)
			switch field {
			case "title":
				r.Title = NormalizeText(val)
			case "href":
				r.Href = NormalizeURL(val)
			case "body":
				r.Body = NormalizeText(val)
			}
		}
		if r.Title == "" && r.Href == "" {
			return nil
		}
		return &SearchResult{Category: CategoryText, Text: r}
	case CategoryImages:
		r := &ImageResult{}
		for field, xpath := range e.ElementsMap {
			val := xpathText(item, xpath)
			switch field {
			case "title":
				r.Title = NormalizeText(val)
			case "image":
				r.Image = NormalizeURL(val)
			case "thumbnail":
				r.Thumbnail = NormalizeURL(val)
			case "url":
				r.URL = NormalizeURL(val)
			case "height":
				r.Height = val
			case "width":
				r.Width = val
			case "source":
				r.Source = NormalizeText(val)
			}
		}
		if r.Image == "" {
			return nil
		}
		return &SearchResult{Category: CategoryImages, Image: r}
	case CategoryNews:
		r := &NewsResult{}
		for field, xpath := range e.ElementsMap {
			val := xpathText(item, xpath)
			switch field {
			case "date":
				r.Date = val
			case "title":
				r.Title = NormalizeText(val)
			case "body":
				r.Body = NormalizeText(val)
			case "url":
				r.URL = NormalizeURL(val)
			case "image":
				r.Image = NormalizeURL(val)
			case "source":
				r.Source = NormalizeText(val)
			}
		}
		if r.Title == "" && r.URL == "" {
			return nil
		}
		return &SearchResult{Category: CategoryNews, News: r}
	case CategoryBooks:
		r := &BooksResult{}
		for field, xpath := range e.ElementsMap {
			val := xpathText(item, xpath)
			switch field {
			case "title":
				r.Title = NormalizeText(val)
			case "author":
				r.Author = NormalizeText(val)
			case "publisher":
				r.Publisher = NormalizeText(val)
			case "info":
				r.Info = NormalizeText(val)
			case "url":
				r.URL = NormalizeURL(val)
			case "thumbnail":
				r.Thumbnail = NormalizeURL(val)
			case "year":
				r.Year = val
			case "language":
				r.Language = val
			case "pages":
				r.Pages = val
			case "size":
				r.Size = val
			case "extension":
				r.Extension = val
			}
		}
		if r.Title == "" && r.URL == "" {
			return nil
		}
		return &SearchResult{Category: CategoryBooks, Books: r}
	default:
		return nil
	}
}

// xpathText evaluates an XPath expression on a node and concatenates all text content.
func xpathText(node *html.Node, expr string) string {
	nodes := htmlquery.Find(node, expr)
	if len(nodes) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, n := range nodes {
		if n == nil {
			continue
		}
		sb.WriteString(htmlquery.InnerText(n))
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

// safeInnerText returns the node text, or "" when node is nil.
// htmlquery.InnerText panics on nil nodes.
func safeInnerText(node *html.Node) string {
	if node == nil {
		return ""
	}
	return htmlquery.InnerText(node)
}

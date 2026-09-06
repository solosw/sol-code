package websearch

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

var annasArchiveDomains = []string{"gl", "gd", "pk", "li"}

// NewAnnasArchive returns the Anna's Archive search engine.
func NewAnnasArchive() SearchEngine {
	domain := annasArchiveDomains[0]
	if n, err := rand.Int(rand.Reader, big.NewInt(int64(len(annasArchiveDomains)))); err == nil {
		domain = annasArchiveDomains[int(n.Int64())]
	}
	baseURL := fmt.Sprintf("https://annas-archive.%s", domain)

	return &XPathEngine{
		name:         "annasarchive",
		category:     CategoryBooks,
		provider:     "annasarchive",
		priority:     1,
		SearchURL:    baseURL + "/search",
		SearchMethod: http.MethodGet,
		ItemsXPath:   "//div[contains(@class, 'record-list-outer')]/div",
		ElementsMap: map[string]string{
			"title":     ".//a[contains(@class, 'text-lg')]//text()",
			"author":    ".//a[span[contains(@class, 'user')]]//text()",
			"publisher": ".//a[span[contains(@class, 'company')]]//text()",
			"info":      ".//div[contains(@class, 'text-gray-800')]/text()",
			"url":       "./a/@href",
			"thumbnail": ".//img/@src",
		},
		PreProcess: func(body []byte) []byte {
			body = bytes.ReplaceAll(body, []byte("<!--"), nil)
			return bytes.ReplaceAll(body, []byte("-->"), nil)
		},
		BuildPayload: func(query string, opts SearchOptions) (url.Values, url.Values) {
			params := url.Values{
				"q":    {query},
				"page": {itoa(opts.Page)},
			}
			return params, nil
		},
		PostProcess: func(results []SearchResult) []SearchResult {
			for i := range results {
				if results[i].Books != nil {
					if !strings.HasPrefix(results[i].Books.URL, "http") {
						results[i].Books.URL = baseURL + results[i].Books.URL
					}
				}
			}
			return results
		},
	}
}
// NewLibgen returns the Libgen search engine.
func NewLibgen() SearchEngine {
	return &LibgenEngine{priority: 1.5}
}

type LibgenEngine struct {
	priority float64
}

func (e *LibgenEngine) Name() string       { return "libgen" }
func (e *LibgenEngine) Category() Category  { return CategoryBooks }
func (e *LibgenEngine) Provider() string    { return "libgen" }
func (e *LibgenEngine) Priority() float64   { return e.priority }

func (e *LibgenEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	client := httpClient(opts.Proxy, opts.Timeout)

	mirror := "https://libgen.li"
	searchURL := fmt.Sprintf("%s/index.php", mirror)
	params := url.Values{
		"req":       {query},
		"columns[]": {"t", "a", "s", "y", "p", "i"}, // title, author, series, year, publisher, isbn
		"topics[]":  {"l", "c", "f", "a", "m", "r", "s"},
		"res":       {"25"},
		"filesuns":  {"all"},
	}

	body, err := doRequest(ctx, client, http.MethodGet, searchURL, params, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("libgen search: %w", err)
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	table := htmlquery.FindOne(doc, "//table[@id='tablelibgen']")
	if table == nil {
		return nil, nil
	}

	rows := htmlquery.Find(table, ".//tr[td]")
	var results []SearchResult

	for _, row := range rows {
		tds := htmlquery.Find(row, "./td")
		if len(tds) < 9 {
			continue
		}

		offset := 0
		img := htmlquery.FindOne(tds[0], ".//img")
		if img != nil {
			src := htmlquery.SelectAttr(img, "src")
			if strings.Contains(src, "covers") {
				offset = 1
			}
		}

		titleLinks := htmlquery.Find(tds[offset], ".//a[@href]")
		if len(titleLinks) == 0 {
			continue
		}

		title := ""
		for _, link := range titleLinks {
			title += htmlquery.InnerText(link)
		}
		title = strings.TrimSpace(title)

		r := &BooksResult{
			Title:     NormalizeText(title),
			Author:    NormalizeText(htmlquery.InnerText(tds[offset+1])),
			Publisher: NormalizeText(htmlquery.InnerText(tds[offset+2])),
			Year:      strings.TrimSpace(htmlquery.InnerText(tds[offset+3])),
			Language:  strings.TrimSpace(htmlquery.InnerText(tds[offset+4])),
			Pages:     strings.TrimSpace(htmlquery.InnerText(tds[offset+5])),
			Size:      strings.TrimSpace(htmlquery.InnerText(tds[offset+6])),
			Extension: strings.TrimSpace(htmlquery.InnerText(tds[offset+7])),
		}

		mirrorLinks := htmlquery.Find(tds[offset+8], ".//a[@href]")
		for _, link := range mirrorLinks {
			href := htmlquery.SelectAttr(link, "href")
			if !strings.HasPrefix(href, "http") {
				href = mirror + "/" + strings.TrimPrefix(href, "/")
			}
			r.Mirrors = append(r.Mirrors, href)
		}

		results = append(results, SearchResult{
			Category: CategoryBooks,
			Books:    r,
		})

		if len(results) >= opts.MaxResults {
			break
		}
	}

	// Concurrent resolution of direct download links
	var wg sync.WaitGroup
	for i := range results {
		if results[i].Books == nil || len(results[i].Books.Mirrors) == 0 {
			continue
		}
		wg.Add(1)
		go func(r *BooksResult) {
			defer wg.Done()
			if direct, err := resolveLibgenDirectLink(ctx, client, r.URL); err == nil {
				r.DirectDownloadURL = direct
			}
		}(results[i].Books)
	}
	wg.Wait()

	return results, nil
}

func resolveLibgenDirectLink(ctx context.Context, client *http.Client, mirrorURL string) (string, error) {
	parsed, err := url.Parse(mirrorURL)
	if err != nil {
		return "", err
	}
	rootURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	body, err := doRequest(ctx, client, http.MethodGet, mirrorURL, nil, nil, nil)
	if err != nil {
		return "", err
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	// Look for "GET" link
	getLinks := htmlquery.Find(doc, "//a[contains(translate(text(), 'get', 'GET'), 'GET')]")
	for _, link := range getLinks {
		href := htmlquery.SelectAttr(link, "href")
		if href == "" {
			continue
		}

		fullURL := href
		if !strings.HasPrefix(fullURL, "http") {
			fullURL = rootURL + "/" + strings.TrimPrefix(href, "/")
		}

		u, err := url.Parse(fullURL)
		if err != nil {
			continue
		}

		q := u.Query()
		key := q.Get("key")
		md5 := q.Get("md5")

		if key != "" && md5 != "" {
			return fmt.Sprintf("%s/get.php?md5=%s&key=%s", rootURL, md5, key), nil
		}
	}

	return "", fmt.Errorf("direct link not found")
}

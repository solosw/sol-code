package websearch

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// NewArxiv returns the arXiv search engine.
func NewArxiv() SearchEngine {
	return &ArxivEngine{priority: 2.0}
}

type ArxivEngine struct {
	priority float64
}

func (e *ArxivEngine) Name() string       { return "arxiv" }
func (e *ArxivEngine) Category() Category  { return CategoryResearch }
func (e *ArxivEngine) Provider() string    { return "arxiv" }
func (e *ArxivEngine) Priority() float64   { return e.priority }

func (e *ArxivEngine) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	client := httpClient(opts.Proxy, opts.Timeout)

	searchURL := "https://export.arxiv.org/api/query"
	params := url.Values{
		"search_query": {query},
		"start":        {itoa((opts.Page - 1) * opts.MaxResults)},
		"max_results":  {itoa(opts.MaxResults)},
	}

	body, err := doRequest(ctx, client, http.MethodGet, searchURL, params, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("arxiv search: %w", err)
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	entries := htmlquery.Find(doc, "//entry")
	var results []SearchResult

	for _, entry := range entries {
		r := &ResearchResult{
			Title:   NormalizeText(safeInnerText(htmlquery.FindOne(entry, "./title"))),
			Summary: NormalizeText(safeInnerText(htmlquery.FindOne(entry, "./summary"))),
			Date:    safeInnerText(htmlquery.FindOne(entry, "./published")),
			URL:     safeInnerText(htmlquery.FindOne(entry, "./id")),
		}

		// Authors
		authorNodes := htmlquery.Find(entry, "./author/name")
		for _, node := range authorNodes {
			if node == nil {
				continue
			}
			r.Authors = append(r.Authors, NormalizeText(htmlquery.InnerText(node)))
		}

		// Categories
		catNodes := htmlquery.Find(entry, "./category")
		for _, node := range catNodes {
			term := htmlquery.SelectAttr(node, "term")
			if term != "" {
				r.Categories = append(r.Categories, term)
			}
		}

		// Links (PDF, DOI)
		linkNodes := htmlquery.Find(entry, "./link")
		for _, node := range linkNodes {
			title := htmlquery.SelectAttr(node, "title")
			href := htmlquery.SelectAttr(node, "href")
			rel := htmlquery.SelectAttr(node, "rel")

			if title == "pdf" || (rel == "related" && strings.Contains(href, "pdf")) {
				r.PDFURL = href
			} else if title == "doi" {
				r.DOI = href
			}
		}
		
		// If PDF URL is not found in links, try to construct it from ID
		if r.PDFURL == "" && r.URL != "" {
			parts := strings.Split(r.URL, "/abs/")
			if len(parts) == 2 {
				r.PDFURL = "https://arxiv.org/pdf/" + parts[1] + ".pdf"
			}
		}

		// Journal Ref
		if journalNode := htmlquery.FindOne(entry, "./journal_ref"); journalNode != nil {
			r.Journal = htmlquery.InnerText(journalNode)
		}

		results = append(results, SearchResult{
			Category: CategoryResearch,
			Research: r,
		})
	}

	return results, nil
}

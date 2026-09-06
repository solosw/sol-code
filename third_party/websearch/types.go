// Package websearch implements a metasearch library that aggregates results
// from multiple web search engines, inspired by the ddgs Python library.
package websearch

// Category represents a search category.
type Category string

const (
	CategoryText   Category = "text"
	CategoryImages Category = "images"
	CategoryNews   Category = "news"
	CategoryVideos   Category = "videos"
	CategoryBooks    Category = "books"
	CategoryResearch Category = "research"
)

// TextResult represents a web text search result.
type TextResult struct {
	Title string `json:"title"`
	Href  string `json:"href"`
	Body  string `json:"body"`
}

// ImageResult represents an image search result.
type ImageResult struct {
	Title     string `json:"title"`
	Image     string `json:"image"`
	Thumbnail string `json:"thumbnail"`
	URL       string `json:"url"`
	Height    string `json:"height"`
	Width     string `json:"width"`
	Source    string `json:"source"`
}

// NewsResult represents a news search result.
type NewsResult struct {
	Date   string `json:"date"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
	Image  string `json:"image"`
	Source string `json:"source"`
}

// VideoResult represents a video search result.
type VideoResult struct {
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Description string            `json:"description"`
	Duration    string            `json:"duration"`
	EmbedHTML   string            `json:"embed_html"`
	EmbedURL    string            `json:"embed_url"`
	ImageToken  string            `json:"image_token"`
	Images      map[string]string `json:"images,omitempty"`
	Provider    string            `json:"provider"`
	Published   string            `json:"published"`
	Publisher   string            `json:"publisher"`
	Statistics  map[string]string `json:"statistics,omitempty"`
	Uploader    string            `json:"uploader"`
}

// BooksResult represents a book search result.
type BooksResult struct {
	Title             string   `json:"title"`
	Author            string   `json:"author"`
	Publisher         string   `json:"publisher"`
	Info              string   `json:"info"`
	URL               string   `json:"url"`
	Thumbnail         string   `json:"thumbnail"`
	Year              string   `json:"year,omitempty"`
	Language          string   `json:"language,omitempty"`
	Pages             string   `json:"pages,omitempty"`
	Size              string   `json:"size,omitempty"`
	Extension         string   `json:"extension,omitempty"`
	Mirrors           []string `json:"mirrors,omitempty"`
	DirectDownloadURL string   `json:"direct_download_url,omitempty"`
}

// ResearchResult represents an academic research result (e.g. arXiv).
type ResearchResult struct {
	Title      string   `json:"title"`
	Authors    []string `json:"authors"`
	Summary    string   `json:"summary"`
	Date       string   `json:"date"`
	URL        string   `json:"url"`
	PDFURL     string   `json:"pdf_url,omitempty"`
	DOI        string   `json:"doi,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Journal    string   `json:"journal,omitempty"`
}

// SearchResult is a union result type returned by the top-level Search function.
// Exactly one of the typed fields will be populated depending on the category.
type SearchResult struct {
	Category Category        `json:"category"`
	Text     *TextResult     `json:"text,omitempty"`
	Image    *ImageResult    `json:"image,omitempty"`
	News     *NewsResult     `json:"news,omitempty"`
	Video    *VideoResult    `json:"video,omitempty"`
	Books    *BooksResult    `json:"books,omitempty"`
	Research *ResearchResult `json:"research,omitempty"`
}

// DeduplicationKey returns the key used for result deduplication.
func (r SearchResult) DeduplicationKey() string {
	switch {
	case r.Text != nil:
		return r.Text.Href
	case r.Image != nil:
		return r.Image.Image
	case r.News != nil:
		return r.News.URL
	case r.Video != nil:
		return r.Video.EmbedURL
	case r.Books != nil:
		return r.Books.URL
	case r.Research != nil:
		return r.Research.URL
	default:
		return ""
	}
}

// Title returns the title of the result regardless of type.
func (r SearchResult) Title() string {
	switch {
	case r.Text != nil:
		return r.Text.Title
	case r.Image != nil:
		return r.Image.Title
	case r.News != nil:
		return r.News.Title
	case r.Video != nil:
		return r.Video.Title
	case r.Books != nil:
		return r.Books.Title
	case r.Research != nil:
		return r.Research.Title
	default:
		return ""
	}
}

// Body returns the body/description of the result regardless of type.
func (r SearchResult) Body() string {
	switch {
	case r.Text != nil:
		return r.Text.Body
	case r.News != nil:
		return r.News.Body
	case r.Video != nil:
		return r.Video.Description
	case r.Books != nil:
		return r.Books.Info
	case r.Research != nil:
		return r.Research.Summary
	default:
		return ""
	}
}

// Href returns the primary URL of the result regardless of type.
func (r SearchResult) Href() string {
	switch {
	case r.Text != nil:
		return r.Text.Href
	case r.Image != nil:
		return r.Image.URL
	case r.News != nil:
		return r.News.URL
	case r.Video != nil:
		return r.Video.EmbedURL
	case r.Books != nil:
		return r.Books.URL
	case r.Research != nil:
		return r.Research.URL
	default:
		return ""
	}
}

// SearchOptions controls search behavior.
type SearchOptions struct {
	Region     string   `json:"region,omitempty"`     // e.g. "us-en"
	SafeSearch string   `json:"safesearch,omitempty"` // "on", "moderate", "off"
	TimeLimit  string   `json:"timelimit,omitempty"`  // "d", "w", "m", "y"
	MaxResults int      `json:"max_results,omitempty"`
	Page       int      `json:"page,omitempty"`
	Backend    string   `json:"backend,omitempty"` // "auto", "duckduckgo", "brave", etc.
	Proxy      string   `json:"proxy,omitempty"`
	Timeout    int      `json:"timeout,omitempty"` // seconds
	Category   Category `json:"category,omitempty"`
}

func (o SearchOptions) withDefaults() SearchOptions {
	if o.Region == "" {
		o.Region = "us-en"
	}
	if o.SafeSearch == "" {
		o.SafeSearch = "moderate"
	}
	if o.MaxResults <= 0 {
		o.MaxResults = 10
	}
	if o.Page <= 0 {
		o.Page = 1
	}
	if o.Backend == "" {
		o.Backend = "auto"
	}
	if o.Timeout <= 0 {
		o.Timeout = 10
	}
	if o.Category == "" {
		o.Category = CategoryText
	}
	return o
}

// Package httpproxy provides a process-wide HTTP(S) proxy configuration used by
// solcode outbound clients (API, Fetch, MCP HTTP, Baidu fallback).
package httpproxy

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Config is the durable proxy settings loaded from settings JSON.
type Config struct {
	// URL is an absolute proxy endpoint, e.g. http://127.0.0.1:7890 or socks5://127.0.0.1:1080.
	URL string `json:"url,omitempty"`
	// Enabled controls whether the configured URL is applied. When false, clients
	// use environment-based proxy resolution (or direct when none is set).
	Enabled bool `json:"enabled,omitempty"`
}

var (
	mu     sync.RWMutex
	active Config
)

// Get returns a copy of the currently active proxy settings.
func Get() Config {
	mu.RLock()
	defer mu.RUnlock()
	return active
}

// Set replaces the active proxy settings. An empty URL is allowed so callers can
// keep a previously configured endpoint while Enabled is false.
func Set(cfg Config) {
	mu.Lock()
	defer mu.Unlock()
	active = Config{
		URL:     strings.TrimSpace(cfg.URL),
		Enabled: cfg.Enabled && strings.TrimSpace(cfg.URL) != "",
	}
	if active.URL == "" {
		active.Enabled = false
	}
}

// Apply stores cfg and returns the normalized active copy.
func Apply(cfg Config) Config {
	Set(cfg)
	return Get()
}

// Clear disables the proxy and drops the stored URL.
func Clear() Config {
	Set(Config{})
	return Get()
}

// Enable turns the proxy on when a URL is already stored.
func Enable() (Config, error) {
	mu.Lock()
	defer mu.Unlock()
	if strings.TrimSpace(active.URL) == "" {
		return active, fmt.Errorf("no proxy URL configured; set one with /proxy <url>")
	}
	active.Enabled = true
	return active, nil
}

// Disable turns the proxy off while keeping the stored URL for later re-enable.
func Disable() Config {
	mu.Lock()
	defer mu.Unlock()
	active.Enabled = false
	return active
}

// ParseURL validates a user-supplied proxy URL.
// Accepted schemes: http, https, socks5, socks5h.
// Bare host:port is treated as http://host:port.
func ParseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("proxy URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q (use http, https, socks5, or socks5h)", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("proxy URL must include a host")
	}
	return u, nil
}

// StatusLine returns a short human-readable summary for /proxy and /status.
func StatusLine(cfg Config) string {
	proxyURL := strings.TrimSpace(cfg.URL)
	switch {
	case proxyURL == "":
		return "Proxy: off (no URL configured)"
	case cfg.Enabled:
		return fmt.Sprintf("Proxy: on (%s)", proxyURL)
	default:
		return fmt.Sprintf("Proxy: off (saved %s)", proxyURL)
	}
}

// Transport returns an *http.Transport that honors the active proxy when enabled.
// The returned transport is a clone; callers may set timeouts on a wrapping client.
func Transport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok || base == nil {
		base = &http.Transport{}
	}
	t := base.Clone()
	t.Proxy = proxyFunc
	return t
}

// NewClient returns an *http.Client that uses the shared proxy-aware transport.
// timeout=0 means no overall client timeout (suitable for long-lived streams).
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: Transport(),
	}
}

func proxyFunc(req *http.Request) (*url.URL, error) {
	cfg := Get()
	if !cfg.Enabled || strings.TrimSpace(cfg.URL) == "" {
		return http.ProxyFromEnvironment(req)
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}
	return u, nil
}

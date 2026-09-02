package httpproxy

import (
	"net/http"
	"strings"
	"testing"
)

func TestParseURLAcceptsSchemesAndShorthand(t *testing.T) {
	u, err := ParseURL("127.0.0.1:7890")
	if err != nil {
		t.Fatalf("ParseURL shorthand: %v", err)
	}
	if u.Scheme != "http" || u.Host != "127.0.0.1:7890" {
		t.Fatalf("shorthand = %s://%s", u.Scheme, u.Host)
	}
	for _, raw := range []string{
		"http://127.0.0.1:7890",
		"https://proxy.example:443",
		"socks5://127.0.0.1:1080",
		"socks5h://127.0.0.1:1080",
	} {
		if _, err := ParseURL(raw); err != nil {
			t.Fatalf("ParseURL(%q)=%v", raw, err)
		}
	}
}

func TestParseURLRejectsBadInput(t *testing.T) {
	for _, raw := range []string{"", "ftp://x", "://missing-host", "http://"} {
		if _, err := ParseURL(raw); err == nil {
			t.Fatalf("ParseURL(%q) expected error", raw)
		}
	}
}

func TestSetEnableDisableClear(t *testing.T) {
	t.Cleanup(func() { Clear() })
	Clear()

	if Get().Enabled {
		t.Fatal("expected disabled after Clear")
	}
	if _, err := Enable(); err == nil {
		t.Fatal("Enable without URL should fail")
	}

	Apply(Config{URL: "http://127.0.0.1:7890", Enabled: true})
	got := Get()
	if !got.Enabled || got.URL != "http://127.0.0.1:7890" {
		t.Fatalf("after Apply: %+v", got)
	}

	Disable()
	got = Get()
	if got.Enabled || got.URL != "http://127.0.0.1:7890" {
		t.Fatalf("after Disable: %+v", got)
	}

	if _, err := Enable(); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !Get().Enabled {
		t.Fatal("expected enabled after Enable")
	}

	Clear()
	if Get().URL != "" || Get().Enabled {
		t.Fatalf("after Clear: %+v", Get())
	}
}

func TestProxyFuncUsesActiveURL(t *testing.T) {
	t.Cleanup(func() { Clear() })
	Apply(Config{URL: "http://proxy.test:9", Enabled: true})
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := proxyFunc(req)
	if err != nil {
		t.Fatal(err)
	}
	if u.String() != "http://proxy.test:9" {
		t.Fatalf("proxy = %s", u)
	}

	Disable()
	// With no env proxy, ProxyFromEnvironment returns nil.
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("https_proxy", "")
	u, err = proxyFunc(req)
	if err != nil {
		t.Fatal(err)
	}
	if u != nil {
		t.Fatalf("expected env/direct nil proxy, got %s", u)
	}
}

func TestStatusLine(t *testing.T) {
	if !strings.Contains(StatusLine(Config{}), "off") {
		t.Fatal("empty should be off")
	}
	if !strings.Contains(StatusLine(Config{URL: "http://x", Enabled: true}), "on") {
		t.Fatal("enabled should be on")
	}
	if got := StatusLine(Config{URL: "http://x", Enabled: false}); !strings.Contains(got, "off") || !strings.Contains(got, "saved") {
		t.Fatalf("disabled with URL: %s", got)
	}
}

func TestTransportClone(t *testing.T) {
	t.Cleanup(func() { Clear() })
	Apply(Config{URL: "http://127.0.0.1:1", Enabled: true})
	tr := Transport()
	if tr == nil || tr.Proxy == nil {
		t.Fatal("expected transport with Proxy func")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	u, err := tr.Proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if u == nil || u.Host != "127.0.0.1:1" {
		t.Fatalf("proxy host = %v", u)
	}
}

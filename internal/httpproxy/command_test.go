package httpproxy

import (
	"strings"
	"testing"
)

func TestApplyCommandSetOnOffClear(t *testing.T) {
	t.Cleanup(func() { Clear() })
	Clear()

	cfg, msg, err := ApplyCommand("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled || !strings.Contains(msg, "Usage:") {
		t.Fatalf("empty status: cfg=%+v msg=%q", cfg, msg)
	}

	cfg, msg, err = ApplyCommand("127.0.0.1:7890")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.URL != "http://127.0.0.1:7890" {
		t.Fatalf("set: %+v", cfg)
	}
	if !strings.Contains(msg, "enabled") {
		t.Fatalf("msg=%q", msg)
	}

	cfg, msg, err = ApplyCommand("off")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled || cfg.URL != "http://127.0.0.1:7890" {
		t.Fatalf("off: %+v", cfg)
	}
	if !strings.Contains(msg, "off") {
		t.Fatalf("msg=%q", msg)
	}

	cfg, msg, err = ApplyCommand("on")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled {
		t.Fatalf("on: %+v msg=%q", cfg, msg)
	}

	cfg, msg, err = ApplyCommand("clear")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled || cfg.URL != "" {
		t.Fatalf("clear: %+v msg=%q", cfg, msg)
	}
}

func TestPersistMap(t *testing.T) {
	m := PersistMap(Config{URL: "http://x", Enabled: true})
	proxy, ok := m["proxy"].(map[string]any)
	if !ok {
		t.Fatalf("persist map: %#v", m)
	}
	if proxy["url"] != "http://x" || proxy["enabled"] != true {
		t.Fatalf("proxy payload: %#v", proxy)
	}
}

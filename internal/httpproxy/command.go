package httpproxy

import (
	"fmt"
	"strings"
)

// ApplyCommand interprets /proxy arguments and updates the process-wide settings.
//
//	(no args)           → status only
//	on | enable         → enable saved URL
//	off | disable       → disable, keep URL
//	clear | none | reset→ clear URL and disable
//	<url>               → set URL and enable
func ApplyCommand(args string) (Config, string, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		cfg := Get()
		return cfg, StatusLine(cfg) + "\nUsage: /proxy <url|on|off|clear>", nil
	}

	switch strings.ToLower(args) {
	case "on", "enable", "enabled":
		cfg, err := Enable()
		if err != nil {
			return Get(), "", err
		}
		return cfg, StatusLine(cfg), nil
	case "off", "disable", "disabled":
		cfg := Disable()
		return cfg, StatusLine(cfg), nil
	case "clear", "none", "reset", "unset":
		cfg := Clear()
		return cfg, StatusLine(cfg), nil
	}

	u, err := ParseURL(args)
	if err != nil {
		return Get(), "", err
	}
	cfg := Apply(Config{URL: u.String(), Enabled: true})
	return cfg, fmt.Sprintf("Proxy enabled: %s", cfg.URL), nil
}

// PersistMap returns the map suitable for config.SaveLocalOverrides.
func PersistMap(cfg Config) map[string]any {
	return map[string]any{
		"proxy": map[string]any{
			"url":     cfg.URL,
			"enabled": cfg.Enabled,
		},
	}
}

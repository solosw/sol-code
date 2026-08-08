package workflowui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/solosw/solcode/internal/config"
	"github.com/solosw/solcode/internal/workflow"
)

func TestSettingsUpdateCarriesModelMCPAndSkillState(t *testing.T) {
	maxTurns := 12
	stored := config.Config{
		Provider: "test",
		Model:    "model-id",
		Effort:   "high",
		Providers: []config.ProviderConfig{{
			Name: "test",
			Models: []config.ModelConfig{{
				Name:             "model-name",
				ID:               "model-id",
				MaxContextTokens: 128_000,
				MaxTurns:         &maxTurns,
				Effort:           "high",
			}},
		}},
		MCP:    config.MCPConfig{Servers: []config.MCPServerConfig{{Name: "enabled"}, {Name: "disabled", Disabled: true}}},
		Skills: config.SkillsConfig{Enabled: []string{"keep", "disable"}, Disabled: []string{"old-disabled"}},
	}
	applied := stored

	srv, url, err := Start(Config{
		Addr: "127.0.0.1:0",
		List: func() []workflow.Definition {
			return nil
		},
		Save: func(workflow.Definition, workflow.SaveScope, *workflow.Layout) (string, error) {
			return "", nil
		},
		Settings: func() config.Config {
			return stored
		},
		ApplySettings: func(next config.Config) error {
			applied = next
			stored = next
			return nil
		},
		Skills: func() []SkillInfo {
			return []SkillInfo{{Name: "keep", Enabled: true}, {Name: "disable", Enabled: true}, {Name: "old-disabled", Enabled: false}}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	payload := map[string]any{
		"provider":           "test",
		"model":              "model-name",
		"effort":             "low",
		"max_turns":          42,
		"max_context_tokens": 256000,
		"mcp_disabled":       map[string]bool{"enabled": true, "disabled": false},
		"skills_enabled":     map[string]bool{"keep": true, "old-disabled": true},
		"skills_disabled":    map[string]bool{"disable": true},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url+"api/settings", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/settings = %d", resp.StatusCode)
	}

	model := applied.Providers[0].Models[0]
	if model.MaxContextTokens != 256000 {
		t.Fatalf("model context = %d, want 256000", model.MaxContextTokens)
	}
	if model.MaxTurns == nil || *model.MaxTurns != 42 {
		t.Fatalf("model max turns = %#v, want 42", model.MaxTurns)
	}
	if model.Effort != "low" {
		t.Fatalf("model effort = %q, want low", model.Effort)
	}
	if applied.MaxContextTokens != 256000 || applied.MaxTurns != 42 || applied.Effort != "low" {
		t.Fatalf("runtime settings = context %d, turns %d, effort %q", applied.MaxContextTokens, applied.MaxTurns, applied.Effort)
	}
	if !applied.MCP.Servers[0].Disabled || applied.MCP.Servers[1].Disabled {
		t.Fatalf("MCP disabled state = %#v", applied.MCP.Servers)
	}
	if containsSetting(applied.Skills.Disabled, "keep") || containsSetting(applied.Skills.Disabled, "old-disabled") || !containsSetting(applied.Skills.Disabled, "disable") {
		t.Fatalf("skill disabled state = %#v", applied.Skills.Disabled)
	}
	if !containsSetting(applied.Skills.Enabled, "keep") || !containsSetting(applied.Skills.Enabled, "old-disabled") || containsSetting(applied.Skills.Enabled, "disable") {
		t.Fatalf("skill enabled state = %#v", applied.Skills.Enabled)
	}

	getResp, err := http.Get(url + "api/settings")
	if err != nil {
		t.Fatal(err)
	}
	defer getResp.Body.Close()
	var settings settingsResponse
	if err := json.NewDecoder(getResp.Body).Decode(&settings); err != nil {
		t.Fatal(err)
	}
	if settings.Model != "model-name" || settings.MaxContext != 256000 || settings.MaxTurns != 42 || settings.Effort != "low" {
		t.Fatalf("GET /api/settings = %#v", settings)
	}
}

func containsSetting(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

package anthropic

import (
	"encoding/json"
	"strings"
	"testing"

	sdk "github.com/anthropics/anthropic-sdk-go"
)

func TestToSDKParamsEnablesPromptCacheMarkers(t *testing.T) {
	tools := []sdk.ToolUnionParam{
		{OfTool: &sdk.ToolParam{Name: "A", Description: sdk.String("a"), InputSchema: sdk.ToolInputSchemaParam{}}},
		{OfTool: &sdk.ToolParam{Name: "B", Description: sdk.String("b"), InputSchema: sdk.ToolInputSchemaParam{}}},
	}
	params := (MessageRequest{
		Model:     "claude-opus-4-8",
		MaxTokens: 1024,
		System:    strings.Repeat("stable system prompt. ", 40),
		Messages:  []sdk.MessageParam{sdk.NewUserMessage(sdk.NewTextBlock("hello"))},
		Tools:     tools,
	}).ToSDKParams()

	if params.CacheControl.Type != "ephemeral" {
		t.Fatalf("top-level CacheControl.Type = %q, want ephemeral", params.CacheControl.Type)
	}
	if params.CacheControl.TTL != sdk.CacheControlEphemeralTTLTTL1h {
		t.Fatalf("top-level CacheControl.TTL = %q, want 1h", params.CacheControl.TTL)
	}
	if len(params.System) != 1 || params.System[0].CacheControl.Type != "ephemeral" {
		t.Fatalf("system CacheControl = %#v, want ephemeral", params.System)
	}
	if params.Tools[0].OfTool.CacheControl.Type != "" {
		t.Fatal("first tool should not be marked")
	}
	if params.Tools[1].OfTool.CacheControl.Type != "ephemeral" {
		t.Fatalf("last tool CacheControl = %#v, want ephemeral", params.Tools[1].OfTool.CacheControl)
	}
	if tools[1].OfTool.CacheControl.Type != "" {
		t.Fatal("original tools slice must not be mutated")
	}

	raw, err := params.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, `"cache_control"`) || !strings.Contains(body, `"ephemeral"`) {
		t.Fatalf("serialized body missing cache_control: %s", body)
	}
	// Ensure markers appear on tools/system, not only top-level.
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sys, _ := decoded["system"].([]any)
	if len(sys) == 0 {
		t.Fatal("missing system in JSON")
	}
	sys0, _ := sys[0].(map[string]any)
	if sys0["cache_control"] == nil {
		t.Fatalf("system block missing cache_control in JSON: %s", body)
	}
	tls, _ := decoded["tools"].([]any)
	last, _ := tls[len(tls)-1].(map[string]any)
	if last["cache_control"] == nil {
		t.Fatalf("last tool missing cache_control in JSON: %s", body)
	}
}

package tool

import (
	"context"
	"encoding/json"
	"testing"
)

func TestModeSwitchToolInvokesConfiguredTransition(t *testing.T) {
	var requested string
	modeTool := NewModeSwitchTool(func(_ context.Context, mode string) error {
		requested = mode
		return nil
	})
	result, err := modeTool.Invoke(context.Background(), nil, json.RawMessage(`{"mode":"plan"}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.IsError || requested != "plan" {
		t.Fatalf("result = %#v, requested = %q", result, requested)
	}
}

func TestModeSwitchToolRejectsUnknownMode(t *testing.T) {
	modeTool := NewModeSwitchTool(func(context.Context, string) error { return nil })
	result, err := modeTool.Invoke(context.Background(), nil, json.RawMessage(`{"mode":"auto"}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !result.IsError {
		t.Fatalf("result = %#v, want validation error", result)
	}
}

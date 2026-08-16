package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const ModeSwitchToolName = "ModeSwitch"

type ModeSwitchRequest struct {
	Mode string `json:"mode"`
}

type modeSwitchTool struct {
	BaseTool
	switchMode func(ctx context.Context, mode string) error
}

func NewModeSwitchTool(switchMode func(ctx context.Context, mode string) error) Tool {
	return &modeSwitchTool{switchMode: switchMode}
}

func (t *modeSwitchTool) Name() string { return ModeSwitchToolName }

func (t *modeSwitchTool) Description() string {
	return "Request a permission-mode transition. Entering plan from another mode requires user approval. Bypass and goal are only available from plan and also require user approval."
}

func (t *modeSwitchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"plan", "bypass", "goal"},
				"description": "Requested permission mode.",
			},
		},
		"required": []string{"mode"},
	}
}

func (t *modeSwitchTool) IsDestructive(json.RawMessage) bool { return false }

func (t *modeSwitchTool) Invoke(ctx context.Context, _ *UseContext, input json.RawMessage) (*ContentBlock, error) {
	var req ModeSwitchRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return ErrorResult("invalid mode switch request: " + err.Error()), nil
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode != "plan" && mode != "bypass" && mode != "goal" {
		return ErrorResult("mode must be plan, bypass, or goal"), nil
	}
	if t.switchMode == nil {
		return nil, fmt.Errorf("mode switching is not configured")
	}
	if err := t.switchMode(ctx, mode); err != nil {
		return ErrorResult(err.Error()), nil
	}
	return Result("Permission mode switched to " + mode + "."), nil
}

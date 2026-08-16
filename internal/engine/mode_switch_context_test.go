package engine

import (
	"context"
	"strings"
	"testing"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/solosw/solcode/internal/agent"
	cpanthropic "github.com/solosw/solcode/internal/anthropic"
	"github.com/solosw/solcode/internal/permission"
)

type modeSwitchClient struct {
	requests []string
	calls    int
	mode     *permission.Service
}

func (c *modeSwitchClient) Create(_ context.Context, req cpanthropic.MessageRequest) (*sdk.Message, error) {
	c.requests = append(c.requests, req.System)
	c.calls++
	if c.calls == 1 {
		c.mode.SetMode(permission.ModePlan)
		return &sdk.Message{}, nil
	}
	return &sdk.Message{}, nil
}

func TestEngineRefreshesPlanContextAfterModeSwitch(t *testing.T) {
	permissions := permission.NewService(permission.ModeAuto)
	client := &modeSwitchClient{mode: permissions}
	engine := NewEngine(Config{Client: client, Permissions: permissions, MaxTurns: 1, ModelName: "test"})

	first := engine.Run(context.Background(), agent.AgentConfig{ID: "main", Role: agent.AgentRoleMain, Prompt: "first", MaxTurns: 1})
	if first.Error != "" {
		t.Fatalf("first Run error = %q", first.Error)
	}
	second := engine.Run(context.Background(), agent.AgentConfig{ID: "main", Role: agent.AgentRoleMain, Prompt: "second", MaxTurns: 1})
	if second.Error != "" {
		t.Fatalf("second Run error = %q", second.Error)
	}
	if len(client.requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(client.requests))
	}
	if client.requests[0] == permission.PlanModeInstructions {
		t.Fatalf("first request unexpectedly had plan instructions")
	}
	if !strings.Contains(client.requests[1], permission.PlanModePromptMarker) {
		t.Fatalf("second request did not refresh plan instructions:\n%s", client.requests[1])
	}
}

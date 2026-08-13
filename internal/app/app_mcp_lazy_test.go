package app

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/solosw/solcode/internal/config"
	"github.com/solosw/solcode/internal/mcp"
	"github.com/solosw/solcode/internal/tool"
)

type startupMCPClient struct {
	mu         sync.Mutex
	startCalls int
	listCalls  int
}

func (c *startupMCPClient) Start(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.startCalls++
	return nil
}

func (c *startupMCPClient) ListTools(context.Context) ([]tool.MCPToolInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listCalls++
	return []tool.MCPToolInfo{{ServerName: "slow", ToolName: "lookup", Description: "lookup"}}, nil
}

func (c *startupMCPClient) CallTool(context.Context, string, json.RawMessage) (*tool.ContentBlock, error) {
	return nil, nil
}

func (c *startupMCPClient) Close() error { return nil }

func (c *startupMCPClient) calls() (start, list int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.startCalls, c.listCalls
}

func TestAppDefersMCPConnectionUntilEnsureMCPTools(t *testing.T) {
	cfg := config.Default()
	cfg.Memory.Enabled = false
	cfg.Session.Enabled = false
	cfg.KnowledgeGraph.Enabled = false
	cfg.MCP.Servers = []config.MCPServerConfig{{Name: "slow", Transport: "stdio", Command: "fake"}}

	client := &startupMCPClient{}
	application, err := New(cfg, WithMCPClientFactory(func(config.MCPServerConfig) mcp.Client { return client }))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer application.Close()

	if start, list := client.calls(); start != 0 || list != 0 {
		t.Fatalf("New() connected MCP server: Start=%d ListTools=%d", start, list)
	}
	if application.Tools.Find("mcp__slow__lookup") != nil {
		t.Fatal("MCP tool registered before EnsureMCPTools")
	}

	if err := application.EnsureMCPTools(context.Background()); err != nil {
		t.Fatalf("EnsureMCPTools() error = %v", err)
	}
	if start, list := client.calls(); start != 1 || list != 1 {
		t.Fatalf("EnsureMCPTools() calls: Start=%d ListTools=%d, want 1 each", start, list)
	}
	if application.Tools.Find("mcp__slow__lookup") == nil {
		t.Fatal("MCP tool not registered after EnsureMCPTools")
	}

	if err := application.EnsureMCPTools(context.Background()); err != nil {
		t.Fatalf("second EnsureMCPTools() error = %v", err)
	}
	if start, list := client.calls(); start != 1 || list != 1 {
		t.Fatalf("second EnsureMCPTools() reconnected MCP server: Start=%d ListTools=%d", start, list)
	}
}

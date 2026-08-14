package app

import (
	"context"
	"testing"
	"time"

	"github.com/solosw/solcode/internal/agent"
)

func TestRunMainAgentRetriesAndReportsStatus(t *testing.T) {
	originalDelay := mainAgentRetryDelay
	mainAgentRetryDelay = time.Millisecond
	t.Cleanup(func() { mainAgentRetryDelay = originalDelay })

	var attempts int
	var statuses []string
	application := &App{onStatus: func(status string) { statuses = append(statuses, status) }}
	result := application.runMainAgent(context.Background(), func() agent.AgentResult {
		attempts++
		if attempts < 3 {
			return agent.AgentResult{AgentID: "main", Error: "temporary task failure"}
		}
		return agent.AgentResult{AgentID: "main", Output: "completed"}
	})

	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if result.Error != "" || result.Output != "completed" {
		t.Fatalf("result = %#v", result)
	}
	if len(statuses) != 2 || statuses[0] != "retry 1/5" || statuses[1] != "retry 2/5" {
		t.Fatalf("statuses = %v, want retry 1/5 and retry 2/5", statuses)
	}
}

func TestRunMainAgentDoesNotRetryCancellation(t *testing.T) {
	originalDelay := mainAgentRetryDelay
	mainAgentRetryDelay = time.Millisecond
	t.Cleanup(func() { mainAgentRetryDelay = originalDelay })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0
	result := (&App{}).runMainAgent(ctx, func() agent.AgentResult {
		attempts++
		return agent.AgentResult{AgentID: "main", Error: context.Canceled.Error()}
	})

	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if result.Error != context.Canceled.Error() {
		t.Fatalf("error = %q, want %q", result.Error, context.Canceled.Error())
	}
}

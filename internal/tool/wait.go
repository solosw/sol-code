package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	WaitToolName = "Wait"
	// DefaultWaitTimeoutMs is used when timeout_ms is omitted (internal/tests).
	DefaultWaitTimeoutMs = 600_000 // 10 minutes
	// MaxWaitTimeoutMs caps a single Wait invocation (24h; matches Bash MaxTimeout).
	// Production long runs wait inside Bash; Wait is not model-visible.
	MaxWaitTimeoutMs = 86_400_000 // 24 hours
)

// WaitParams controls waiting on background Bash jobs.
// Wait is an internal helper (tests / registry execution); the model uses Bash
// timeout > AutoWaitThresholdMs which auto-waits for the same duration.
type WaitParams struct {
	// TaskID is a background bash job id.
	// Empty waits for all currently running background jobs.
	TaskID string `json:"task_id,omitempty"`
	// TimeoutMs is how long this Wait call may block (milliseconds).
	// Production Bash auto-wait uses the Bash timeout value (max 24h).
	TimeoutMs int `json:"timeout_ms,omitempty"`
}

type waitTool struct {
	BaseTool
	jobs *BackgroundJobs
}

// NewWaitTool creates the Wait tool bound to the process-wide job registry.
func NewWaitTool() Tool {
	return NewWaitToolWithJobs(DefaultBackgroundJobs())
}

// NewWaitToolWithJobs creates Wait bound to a specific registry (tests).
func NewWaitToolWithJobs(jobs *BackgroundJobs) Tool {
	if jobs == nil {
		jobs = DefaultBackgroundJobs()
	}
	return &waitTool{jobs: jobs}
}

func (w *waitTool) Name() string { return WaitToolName }

func (w *waitTool) Description() string {
	// Not shown to the model (excluded from tool selection). Kept for tests
	// and any direct registry invocation.
	return `Internal: suspend until a background Bash job finishes (or wait timeout elapses).
Long-running shell work should set Bash timeout > 3 minutes; Bash auto-waits up to 24h.
- task_id: background job id. Omit to wait for all running background jobs.
- timeout_ms: max block time (default 600000, max 86400000 = 24h).`
}

func (w *waitTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": map[string]any{
				"type":        "string",
				"description": "Background task id. Omit to wait for all running background jobs.",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"description": "Maximum time to wait in milliseconds (default 600000, max 86400000 = 24h)",
			},
		},
	}
}

func (w *waitTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (w *waitTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (w *waitTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (w *waitTool) Invoke(ctx context.Context, _ *UseContext, input json.RawMessage) (*ContentBlock, error) {
	var params WaitParams
	if len(input) > 0 && string(input) != "null" {
		if err := json.Unmarshal(input, &params); err != nil {
			return ErrorResult("invalid parameters: " + err.Error()), nil
		}
	}

	timeoutMs := params.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = DefaultWaitTimeoutMs
	}
	if timeoutMs > MaxWaitTimeoutMs {
		timeoutMs = MaxWaitTimeoutMs
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond

	taskID := strings.TrimSpace(params.TaskID)
	if taskID == "" {
		snaps, err := w.jobs.WaitAny(ctx, timeout)
		running := 0
		for _, s := range snaps {
			if s.Status == JobRunning {
				running++
			}
		}
		if len(snaps) == 0 {
			return Result("No background Bash jobs."), nil
		}
		var b strings.Builder
		if err == errWaitTimeout {
			fmt.Fprintf(&b, "Wait timed out after %s; %d job(s) still running.\n\n", timeout.Round(time.Second), running)
		} else if err != nil && ctx.Err() != nil {
			fmt.Fprintf(&b, "Wait canceled (%v).\n\n", err)
		} else if running == 0 {
			fmt.Fprintf(&b, "All background jobs finished (%d).\n\n", len(snaps))
		} else {
			fmt.Fprintf(&b, "Background jobs update (%d total, %d running).\n\n", len(snaps), running)
		}
		// Prefer showing still-running and most recent finished jobs.
		for _, snap := range snaps {
			b.WriteString(formatJobSnapshot(snap))
			b.WriteString("\n\n")
		}
		return Result(strings.TrimSpace(b.String())), nil
	}

	snap, err := w.jobs.Wait(ctx, taskID, timeout)
	if err != nil && err != errWaitTimeout && ctx.Err() == nil && !isUnknownJob(err) {
		// Unexpected error path.
		return ErrorResult(err.Error()), nil
	}
	if isUnknownJob(err) {
		return ErrorResult(err.Error()), nil
	}

	text := formatJobSnapshot(snap)
	if err == errWaitTimeout {
		text = fmt.Sprintf("Wait timed out after %s; job still running.\n\n%s", timeout.Round(time.Second), text)
	} else if ctx.Err() != nil {
		text = fmt.Sprintf("Wait canceled.\n\n%s", text)
	}
	isErr := snap.Status == JobFailed || snap.Status == JobTimedOut || snap.Status == JobCanceled
	return &ContentBlock{Type: "text", Text: text, IsError: isErr}, nil
}

func isUnknownJob(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unknown background task_id")
}

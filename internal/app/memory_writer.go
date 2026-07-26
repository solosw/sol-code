package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/solosw/solcode/internal/memory"
	"github.com/solosw/solcode/internal/tool"
)

// WriteMemory implements tool.MemoryWriter so the model can decide, mid-task,
// that something is worth remembering across sessions.
func (a *App) WriteMemory(ctx context.Context, req tool.MemoryWriteRequest) (tool.MemoryWriteResult, error) {
	if a == nil || a.MemoryManager == nil || !a.Config.Memory.Enabled {
		return tool.MemoryWriteResult{}, fmt.Errorf("memory is not enabled")
	}

	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = a.Config.Session.DefaultSession
	}

	outcome, err := a.MemoryManager.RememberDirect(ctx, memory.DirectInput{
		Text:            req.Text,
		Kind:            memory.Kind(req.Kind),
		Scope:           memory.Scope(req.Scope),
		Tier:            memoryTierForWrite(req.Kind),
		Tags:            req.Tags,
		Importance:      req.Importance,
		Reason:          req.Reason,
		SourceSessionID: sessionID,
	})
	if err != nil {
		return tool.MemoryWriteResult{}, err
	}

	result := tool.MemoryWriteResult{
		Stored: outcome.Stored,
		Merged: outcome.Merged,
		Reason: outcome.Reason,
	}
	if outcome.Stored {
		result.ID = outcome.Item.ID
		result.Text = outcome.Item.Text
		result.Tier = string(outcome.Item.Tier)
		result.Kind = string(outcome.Item.Kind)
		result.Scope = string(outcome.Item.Scope)
	}
	return result, nil
}

// memoryTierForWrite picks a starting tier from the declared kind: durable
// knowledge lands in M4, procedural workflows in M5, and task notes stay in M3
// so lifecycle decay can retire them.
func memoryTierForWrite(kind string) memory.Tier {
	switch memory.Kind(strings.ToLower(strings.TrimSpace(kind))) {
	case memory.KindWorkflow:
		return memory.TierProcedural
	case memory.KindTask:
		return memory.TierShortTerm
	default:
		return memory.TierLongTerm
	}
}

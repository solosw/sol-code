package acp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/solosw/solcode/internal/engine"
	"github.com/solosw/solcode/internal/session"
)

func promptToText(blocks []ContentBlock, workDir string) string {
	var parts []string
	for _, block := range blocks {
		switch strings.ToLower(strings.TrimSpace(block.Type)) {
		case "", "text":
			if text := strings.TrimSpace(block.Text); text != "" {
				parts = append(parts, text)
			}
		case "image":
			path, err := writePromptImage(block, workDir)
			if err != nil {
				parts = append(parts, fmt.Sprintf("[failed to attach image: %v]", err))
				continue
			}
			parts = append(parts, "@"+quotePath(path))
		case "resource_link":
			label := strings.TrimSpace(block.Name)
			uri := strings.TrimSpace(block.URI)
			switch {
			case label != "" && uri != "":
				parts = append(parts, fmt.Sprintf("%s (%s)", label, uri))
			case uri != "":
				parts = append(parts, uri)
			case label != "":
				parts = append(parts, label)
			}
			if text := strings.TrimSpace(block.Text); text != "" {
				parts = append(parts, text)
			}
		case "resource":
			if text := strings.TrimSpace(block.Text); text != "" {
				parts = append(parts, text)
				continue
			}
			uri := resourceURI(block)
			if uri != "" {
				parts = append(parts, uri)
			}
			if name := strings.TrimSpace(block.Name); name != "" && name != uri {
				parts = append(parts, name)
			}
		default:
			if text := strings.TrimSpace(block.Text); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func writePromptImage(block ContentBlock, workDir string) (string, error) {
	data := strings.TrimSpace(block.Data)
	if data == "" {
		return "", fmt.Errorf("image data is empty")
	}
	mime := strings.ToLower(strings.TrimSpace(block.MimeType))
	ext := ".png"
	switch mime {
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	case "image/png", "":
		ext = ".png"
	default:
		if strings.HasPrefix(mime, "image/") {
			ext = "." + strings.TrimPrefix(mime, "image/")
		}
	}
	dir := filepath.Join(workDir, ".solcode", "acp-uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := strings.TrimSpace(block.Name)
	if name == "" {
		name = "image" + ext
	} else if filepath.Ext(name) == "" {
		name += ext
	}
	path := filepath.Join(dir, sanitizeFileName(name))
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, decoded, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func quotePath(path string) string {
	if strings.ContainsAny(path, " \t") {
		return `"` + path + `"`
	}
	return path
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		return "image.png"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "image.png"
	}
	return out
}

func resourceURI(block ContentBlock) string {
	if uri := strings.TrimSpace(block.URI); uri != "" {
		return uri
	}
	if len(block.Resource) == 0 {
		return ""
	}
	var payload struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(block.Resource, &payload); err != nil {
		return ""
	}
	if payload.Text != "" {
		return payload.Text
	}
	if payload.URI != "" {
		return payload.URI
	}
	return payload.Name
}

func sessionHistoryUpdates(s *session.Session) []SessionUpdate {
	if s == nil {
		return nil
	}
	var updates []SessionUpdate
	for _, msg := range s.Messages {
		role := strings.ToLower(string(msg.Role))
		for _, block := range msg.Content {
			switch {
			case block.OfText != nil:
				text := strings.TrimSpace(block.OfText.Text)
				if text == "" {
					continue
				}
				kind := "agent_message_chunk"
				if role == "user" {
					kind = "user_message_chunk"
				}
				content := ContentBlock{Type: "text", Text: text}
				updates = append(updates, SessionUpdate{SessionUpdate: kind, Content: &content})
			case block.OfToolUse != nil:
				input, _ := json.Marshal(block.OfToolUse.Input)
				updates = append(updates, SessionUpdate{
					SessionUpdate: "tool_call",
					ToolCallID:    block.OfToolUse.ID,
					Title:         block.OfToolUse.Name,
					Kind:          toolKind(block.OfToolUse.Name),
					Status:        ToolCallCompleted,
					RawInput:      input,
				})
			case block.OfToolResult != nil:
				text := toolResultText(block.OfToolResult)
				output, _ := json.Marshal(map[string]string{"output": text})
				status := ToolCallCompleted
				if toolResultIsError(block.OfToolResult) {
					status = ToolCallFailed
				}
				updates = append(updates, SessionUpdate{
					SessionUpdate: "tool_call_update",
					ToolCallID:    block.OfToolResult.ToolUseID,
					Status:        status,
					RawOutput:     output,
					ToolContent: []ToolCallContent{{
						Type:    "content",
						Content: &ContentBlock{Type: "text", Text: text},
					}},
				})
			}
		}
	}
	return updates
}

func toolResultText(result *sdk.ToolResultBlockParam) string {
	if result == nil {
		return ""
	}
	var parts []string
	for _, content := range result.Content {
		if content.OfText != nil {
			parts = append(parts, content.OfText.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func toolResultIsError(result *sdk.ToolResultBlockParam) bool {
	if result == nil {
		return false
	}
	return result.IsError.Or(false)
}

func toolKind(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "read", "view", "glob", "grep", "ls":
		return "read"
	case "edit", "multiedit", "write", "multiwrite", "patch":
		return "edit"
	case "bash":
		return "execute"
	case "websearch", "fetch":
		return "fetch"
	default:
		return "other"
	}
}

func usageUpdate(usage engine.Usage) *UsageUpdate {
	return &UsageUpdate{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		MaxContextTokens:         usage.MaxContextTokens,
		EstimatedContextTokens:   usage.EstimatedContextTokens,
	}
}

func availableCommands() []AvailableCommand {
	return []AvailableCommand{
		{Name: "help", Description: "Show available commands"},
		{Name: "model", Description: "Select a model from the current provider"},
		{Name: "provider", Description: "Select a provider"},
		{Name: "effort", Description: "Select thinking effort"},
		{Name: "sessions", Description: "List or switch saved sessions"},
		{Name: "compact", Description: "Compact the current session now"},
		{Name: "fix-session", Description: "Repair invalid tool-use chains in the current session"},
		{Name: "new-session", Description: "Create and switch to a new session", Input: &AvailableCommandInput{Hint: "optional name"}},
		{Name: "skills", Description: "Browse skills and toggle enabled/disabled"},
		{Name: "mcp", Description: "Browse MCP servers and toggle enabled/disabled"},
		{Name: "goal", Description: "Work from goal.md until complete", Input: &AvailableCommandInput{Hint: "optional description"}},
		{Name: "workflows", Description: "List loaded workflows"},
	}
}

package session

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	sdk "github.com/anthropics/anthropic-sdk-go"
)

const (
	ObservationMaskMarker      = "[observation-masked]"
	defaultRecentUnmasked      = 2
	defaultMaskMinChars        = 400
	observationStoreDirName = "observations"
)

// ObservationStore persists masked tool observations so later turns can
// retrieve the original payload from the placeholder text.
type ObservationStore interface {
	Save(id, content string) (path string, err error)
	Load(id string) (content string, err error)
}

// FileObservationStore writes one file per masked observation under dir.
type FileObservationStore struct {
	Dir string
}

func NewFileObservationStore(dir string) *FileObservationStore {
	return &FileObservationStore{Dir: dir}
}

func ObservationStoreDir(sessionDir string, id SessionID) string {
	return filepath.Join(sessionDir, observationStoreDirName, string(id))
}

func (s *FileObservationStore) pathFor(id string) string {
	safe := sanitizeObservationID(id)
	return filepath.Join(s.Dir, safe+".txt")
}

func (s *FileObservationStore) Save(id, content string) (string, error) {
	if s == nil || strings.TrimSpace(s.Dir) == "" {
		return "", fmt.Errorf("observation store dir is empty")
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", fmt.Errorf("create observation store: %w", err)
	}
	path := s.pathFor(id)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write observation %q: %w", id, err)
	}
	return path, nil
}

func (s *FileObservationStore) Load(id string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("observation store is nil")
	}
	data, err := os.ReadFile(s.pathFor(id))
	if err != nil {
		return "", fmt.Errorf("read observation %q: %w", id, err)
	}
	return string(data), nil
}

type ObservationMaskOptions struct {
	// RecentUnmaskedTurns keeps the newest N user turns fully unmasked.
	RecentUnmaskedTurns int
	// MinChars only masks tool results at least this long.
	MinChars int
}

type ObservationMaskResult struct {
	Messages []sdk.MessageParam
	Masked   int
	Changed  bool
}

// MaskObservations replaces older verbose tool results with placeholders that
// include a lookup id/path. Reasoning, tool-use actions, and recent turns stay
// intact, including Edit/Write calls; only their older result payloads are
// masked. Original payloads are written to store when provided.
func MaskObservations(messages []sdk.MessageParam, store ObservationStore, opts ObservationMaskOptions) (ObservationMaskResult, error) {
	keep := opts.RecentUnmaskedTurns
	if keep <= 0 {
		keep = defaultRecentUnmasked
	}
	minChars := opts.MinChars
	if minChars <= 0 {
		minChars = defaultMaskMinChars
	}
	protectFrom := compactCutIndex(messages, keep)
	if protectFrom <= 0 {
		return ObservationMaskResult{Messages: append([]sdk.MessageParam(nil), messages...)}, nil
	}
	out := make([]sdk.MessageParam, 0, len(messages))
	toolNamesByID := map[string]string{}
	masked := 0
	changed := false
	for i, message := range messages {
		blocks := make([]sdk.ContentBlockParamUnion, 0, len(message.Content))
		for _, block := range message.Content {
			if block.OfToolUse != nil {
				id := strings.TrimSpace(block.OfToolUse.ID)
				if id != "" {
					toolNamesByID[id] = block.OfToolUse.Name
				}
			}
			if i < protectFrom && block.OfToolResult != nil && shouldMaskObservation(block.OfToolResult, minChars) {
				replaced, did, err := maskToolResultBlock(block, store, toolNamesByID[block.OfToolResult.ToolUseID])
				if err != nil {
					return ObservationMaskResult{}, err
				}
				if did {
					masked++
					changed = true
					block = replaced
				}
			}
			blocks = append(blocks, block)
		}
		out = append(out, sdk.MessageParam{Role: message.Role, Content: blocks})
	}
	return ObservationMaskResult{Messages: out, Masked: masked, Changed: changed}, nil
}

func shouldMaskObservation(block *sdk.ToolResultBlockParam, minChars int) bool {
	if block == nil {
		return false
	}
	text := strings.TrimSpace(observationContent(block))
	if text == "" || isMaskedObservationText(text) {
		return false
	}
	return utf8.RuneCountInString(text) >= minChars
}

func maskToolResultBlock(block sdk.ContentBlockParamUnion, store ObservationStore, toolName string) (sdk.ContentBlockParamUnion, bool, error) {
	if block.OfToolResult == nil {
		return block, false, nil
	}
	original := strings.TrimSpace(observationContent(block.OfToolResult))
	id := observationID(block.OfToolResult.ToolUseID, original)
	if store != nil {
		if _, err := store.Save(id, original); err != nil {
			return block, false, err
		}
	}
	placeholder := formatObservationPlaceholder(toolName, id)
	toolResult := *block.OfToolResult
	toolResult.Content = []sdk.ToolResultBlockParamContentUnion{
		{OfText: &sdk.TextBlockParam{Text: placeholder}},
	}
	block.OfToolResult = &toolResult
	return block, true, nil
}

func formatObservationPlaceholder(toolName, id string) string {
	if strings.TrimSpace(toolName) == "" {
		toolName = "tool"
	}
	return fmt.Sprintf("%s tool=%s observation_id=%s", ObservationMaskMarker, toolName, id)
}

func observationID(toolUseID, content string) string {
	toolUseID = strings.TrimSpace(toolUseID)
	sum := sha1.Sum([]byte(toolUseID + "\n" + content))
	short := hex.EncodeToString(sum[:8])
	if toolUseID == "" {
		return short
	}
	return sanitizeObservationID(toolUseID) + "-" + short
}

func sanitizeObservationID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "observation"
	}
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "observation"
	}
	return out
}

func isMaskedObservationText(text string) bool {
	return strings.Contains(text, ObservationMaskMarker)
}

func observationContent(block *sdk.ToolResultBlockParam) string {
	if block == nil {
		return ""
	}
	parts := make([]string, 0, len(block.Content))
	for _, content := range block.Content {
		if content.OfText != nil {
			parts = append(parts, content.OfText.Text)
			continue
		}
		if text := content.GetText(); text != nil {
			parts = append(parts, *text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// ParseObservationRef extracts a stored observation id/path from placeholder text.
func ParseObservationRef(text string) (id, path string) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "path=") {
			path = strings.TrimSpace(strings.TrimPrefix(line, "path="))
		}
		if strings.Contains(line, "observation_id=") {
			for _, field := range strings.Fields(line) {
				if strings.HasPrefix(field, "observation_id=") {
					id = strings.TrimPrefix(field, "observation_id=")
				}
			}
		}
	}
	return id, path
}

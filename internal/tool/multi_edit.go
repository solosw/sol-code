package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MultiEditParams applies ordered exact replacements. Edits targeting the same
// file see the result of preceding edits, and no file is written until every
// replacement has been validated.
type MultiEditParams struct {
	Edits []EditParams `json:"edits"`
}

const MultiEditToolName = "MultiEdit"

type multiEditTool struct{ BaseTool }

// NewMultiEditTool creates a transactional batch exact-replacement tool.
func NewMultiEditTool() Tool { return &multiEditTool{} }

func (t *multiEditTool) Name() string                         { return MultiEditToolName }
func (t *multiEditTool) IsDestructive(_ json.RawMessage) bool { return true }
func (t *multiEditTool) IsReadOnly(_ json.RawMessage) bool    { return false }

func (t *multiEditTool) Description() string {
	return `Applies multiple exact text replacements atomically across one or more files.
Use this when one file has multiple changes or several files need targeted edits.

Input: edits is an ordered list of file_path, old_string, new_string, and optional desc.
Each old_string must occur exactly once in the file state produced by preceding edits.
All edits are validated before any file is written; if validation fails, no files change.`
}

func (t *multiEditTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"edits": map[string]any{
				"type":        "array",
				"description": "Ordered exact replacements to apply together.",
				"minItems":    1,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_path":  map[string]any{"type": "string"},
						"old_string": map[string]any{"type": "string"},
						"new_string": map[string]any{"type": "string"},
						"desc":       map[string]any{"type": "string"},
					},
					"required": []string{"file_path", "old_string", "new_string"},
				},
			},
		},
		"required": []string{"edits"},
	}
}

func (t *multiEditTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var params MultiEditParams
	if err := json.Unmarshal(input, &params); err != nil {
		return err
	}
	if len(params.Edits) == 0 {
		return fmt.Errorf("at least one edit is required")
	}
	for i, edit := range params.Edits {
		if strings.TrimSpace(edit.FilePath) == "" {
			return fmt.Errorf("edits[%d].file_path is required", i)
		}
		if _, errText := validateChangeDescription(edit.Desc); errText != "" {
			return fmt.Errorf("edits[%d]: %s", i, errText)
		}
	}
	return nil
}

type stagedEditFile struct {
	path    string
	before  string
	after   string
	existed bool
	descs   []string
}

func (t *multiEditTool) Invoke(ctx context.Context, uctx *UseContext, input json.RawMessage) (*ContentBlock, error) {
	var params MultiEditParams
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorResult("invalid parameters: " + err.Error()), nil
	}
	if err := t.ValidateInput(ctx, input); err != nil {
		return ErrorResult(err.Error()), nil
	}

	files := make([]*stagedEditFile, 0, len(params.Edits))
	byPath := make(map[string]*stagedEditFile, len(params.Edits))
	for index, edit := range params.Edits {
		path := ResolvePath(uctx, edit.FilePath)
		if err := CheckAllowedPath(uctx, path); err != nil {
			return ErrorResult(fmt.Sprintf("edits[%d]: %v", index, err)), nil
		}
		file := byPath[path]
		if file == nil {
			data, err := os.ReadFile(path)
			existed := err == nil
			if err != nil && !os.IsNotExist(err) {
				return ErrorResult(fmt.Sprintf("edits[%d]: read %s: %v", index, path, err)), nil
			}
			file = &stagedEditFile{path: path, before: string(data), after: string(data), existed: existed}
			byPath[path] = file
			files = append(files, file)
		}
		if err := applyStagedEdit(file, edit); err != nil {
			return ErrorResult(fmt.Sprintf("edits[%d] (%s): %v", index, edit.FilePath, err)), nil
		}
		if desc, _ := validateChangeDescription(edit.Desc); desc != "" {
			file.descs = append(file.descs, desc)
		}
	}

	if err := commitStagedEdits(files); err != nil {
		return ErrorResult(err.Error()), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Applied %d edits across %d files.\n", len(params.Edits), len(files)))
	for _, file := range files {
		desc := strings.Join(file.descs, "; ")
		if desc == "" {
			desc = "batch edit"
		}
		recordFileChange(ctx, uctx, MultiEditToolName, file.path, desc, file.before, file.after)
		additions, removals := CountDiffChanges(GenerateSimpleDiff(file.before, file.after, file.path))
		fmt.Fprintf(&result, "- %s (+%d -%d)\n", file.path, additions, removals)
	}
	return Result(strings.TrimSpace(result.String())), nil
}

func applyStagedEdit(file *stagedEditFile, edit EditParams) error {
	if edit.OldString == "" {
		if file.existed || file.after != "" {
			return fmt.Errorf("old_string may be empty only when creating a new empty file")
		}
		file.after = edit.NewString
		return nil
	}
	index := strings.Index(file.after, edit.OldString)
	if index < 0 {
		return fmt.Errorf("old_string not found; it must match exactly")
	}
	if index != strings.LastIndex(file.after, edit.OldString) {
		return fmt.Errorf("old_string appears multiple times; include more context for a unique match")
	}
	file.after = file.after[:index] + edit.NewString + file.after[index+len(edit.OldString):]
	return nil
}

func commitStagedEdits(files []*stagedEditFile) error {
	committed := make([]*stagedEditFile, 0, len(files))
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file.path), 0o755); err != nil {
			rollbackStagedEdits(committed)
			return fmt.Errorf("create parent directory for %s: %w", file.path, err)
		}
		if err := os.WriteFile(file.path, []byte(file.after), 0o644); err != nil {
			rollbackStagedEdits(committed)
			return fmt.Errorf("write %s: %w", file.path, err)
		}
		committed = append(committed, file)
	}
	return nil
}

func rollbackStagedEdits(files []*stagedEditFile) {
	for index := len(files) - 1; index >= 0; index-- {
		file := files[index]
		if file.existed {
			_ = os.WriteFile(file.path, []byte(file.before), 0o644)
		} else {
			_ = os.Remove(file.path)
		}
	}
}

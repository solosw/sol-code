package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// MultiWriteParams writes complete contents to multiple files together.
type MultiWriteParams struct {
	Files []WriteParams `json:"files"`
}

const MultiWriteToolName = "MultiWrite"

type multiWriteTool struct{ BaseTool }

// NewMultiWriteTool creates a transactional batch full-file writer.
func NewMultiWriteTool() Tool { return &multiWriteTool{} }

func (t *multiWriteTool) Name() string                         { return MultiWriteToolName }
func (t *multiWriteTool) IsDestructive(_ json.RawMessage) bool { return true }
func (t *multiWriteTool) IsReadOnly(_ json.RawMessage) bool    { return false }

func (t *multiWriteTool) Description() string {
	return `Writes complete contents to multiple files atomically.
Use this when creating or replacing several files in one coordinated change.

Input: files is a list of file_path, content, and optional desc. All paths are
validated before writing. If a write fails, files already written by this call
are restored to their prior contents or removed when newly created.`
}

func (t *multiWriteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"files": map[string]any{
				"type":        "array",
				"description": "Files and complete contents to write together.",
				"minItems":    1,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file_path": map[string]any{"type": "string"},
						"content":   map[string]any{"type": "string"},
						"desc":      map[string]any{"type": "string"},
					},
					"required": []string{"file_path", "content"},
				},
			},
		},
		"required": []string{"files"},
	}
}

func (t *multiWriteTool) ValidateInput(_ context.Context, input json.RawMessage) error {
	var params MultiWriteParams
	if err := json.Unmarshal(input, &params); err != nil {
		return err
	}
	if len(params.Files) == 0 {
		return fmt.Errorf("at least one file is required")
	}
	for i, file := range params.Files {
		if strings.TrimSpace(file.FilePath) == "" {
			return fmt.Errorf("files[%d].file_path is required", i)
		}
		if _, errText := validateChangeDescription(file.Desc); errText != "" {
			return fmt.Errorf("files[%d]: %s", i, errText)
		}
	}
	return nil
}

func (t *multiWriteTool) Invoke(ctx context.Context, uctx *UseContext, input json.RawMessage) (*ContentBlock, error) {
	var params MultiWriteParams
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorResult("invalid parameters: " + err.Error()), nil
	}
	if err := t.ValidateInput(ctx, input); err != nil {
		return ErrorResult(err.Error()), nil
	}

	files := make([]*stagedEditFile, 0, len(params.Files))
	seen := make(map[string]struct{}, len(params.Files))
	for index, write := range params.Files {
		path := ResolvePath(uctx, write.FilePath)
		if err := CheckAllowedPath(uctx, path); err != nil {
			return ErrorResult(fmt.Sprintf("files[%d]: %v", index, err)), nil
		}
		if _, exists := seen[path]; exists {
			return ErrorResult(fmt.Sprintf("files[%d]: duplicate file_path %s", index, write.FilePath)), nil
		}
		seen[path] = struct{}{}
		data, err := ReadTextFileContent(ctx, uctx, path)
		existed := err == nil
		if err != nil && !os.IsNotExist(err) {
			return ErrorResult(fmt.Sprintf("files[%d]: read %s: %v", index, path, err)), nil
		}
		desc, _ := validateChangeDescription(write.Desc)
		files = append(files, &stagedEditFile{path: path, before: string(data), after: write.Content, existed: existed, descs: []string{desc}})
	}

	if err := commitStagedEdits(ctx, uctx, files); err != nil {
		return ErrorResult(err.Error()), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Wrote %d files.\n", len(files)))
	for _, file := range files {
		desc := strings.TrimSpace(strings.Join(file.descs, "; "))
		if desc == "" {
			desc = "batch write"
		}
		recordFileChange(ctx, uctx, MultiWriteToolName, file.path, desc, file.before, file.after)
		additions, removals := CountDiffChanges(GenerateSimpleDiff(file.before, file.after, file.path))
		fmt.Fprintf(&result, "- %s (+%d -%d)\n", file.path, additions, removals)
	}
	return Result(strings.TrimSpace(result.String())), nil
}

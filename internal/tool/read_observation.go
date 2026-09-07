package tool

import (
	"context"
	"encoding/json"
	"strings"
)

const ReadObservationToolName = "ReadObservation"

// ObservationReadRequest looks up a compacted tool observation by the
// placeholder text, observation_id, or stored path.
type ObservationReadRequest struct {
	Ref       string
	ID        string
	Path      string
	SessionID string
	WorkDir   string
}

// ObservationReader retrieves original payloads that compaction masked.
type ObservationReader interface {
	ReadObservation(ctx context.Context, req ObservationReadRequest) (string, error)
}

// ReadObservationParams is the input schema for the ReadObservation tool.
type ReadObservationParams struct {
	Ref  string `json:"ref,omitempty"`
	ID   string `json:"observation_id,omitempty"`
	Path string `json:"path,omitempty"`
}

type readObservationTool struct {
	BaseTool
	reader ObservationReader
}

// NewReadObservationTool creates the model-driven observation lookup tool.
func NewReadObservationTool(reader ObservationReader) Tool {
	return &readObservationTool{reader: reader}
}

func (t *readObservationTool) Name() string { return ReadObservationToolName }

func (t *readObservationTool) Description() string {
	return `Retrieve the original payload of a compacted tool result.

Use this when history contains [observation-masked] placeholders and you need
the full observation. Pass the placeholder text as ref, or the observation_id /
path fields printed in that placeholder. Do not guess filenames; copy the
values from the masked tool result.`
}

func (t *readObservationTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ref": map[string]any{
				"type":        "string",
				"description": "The [observation-masked] placeholder text, or a raw observation_id.",
			},
			"observation_id": map[string]any{
				"type":        "string",
				"description": "Stored observation_id from the placeholder.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Stored path= value from the placeholder.",
			},
		},
	}
}

func (t *readObservationTool) IsDestructive(_ json.RawMessage) bool     { return false }
func (t *readObservationTool) IsReadOnly(_ json.RawMessage) bool        { return true }
func (t *readObservationTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *readObservationTool) Invoke(ctx context.Context, uctx *UseContext, input json.RawMessage) (*ContentBlock, error) {
	var params ReadObservationParams
	if len(strings.TrimSpace(string(input))) > 0 {
		if err := json.Unmarshal(input, &params); err != nil {
			return ErrorResult("invalid parameters: " + err.Error()), nil
		}
	}
	if t.reader == nil {
		return ErrorResult("observation store is not enabled for this session"), nil
	}

	req := ObservationReadRequest{
		Ref:  strings.TrimSpace(params.Ref),
		ID:   strings.TrimSpace(params.ID),
		Path: strings.TrimSpace(params.Path),
	}
	if uctx != nil {
		req.SessionID = uctx.SessionID
		req.WorkDir = uctx.WorkDir
	}
	if req.Ref == "" && req.ID == "" && req.Path == "" {
		return ErrorResult("ref, observation_id, or path is required"), nil
	}

	content, err := t.reader.ReadObservation(ctx, req)
	if err != nil {
		return ErrorResult("failed to read observation: " + err.Error()), nil
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ErrorResult("observation is empty"), nil
	}
	return Result(content), nil
}

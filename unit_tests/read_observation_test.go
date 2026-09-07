package unit_tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/solosw/solcode/internal/tool"
)

type fakeObservationReader struct {
	got    tool.ObservationReadRequest
	calls  int
	result string
	err    error
}

func (f *fakeObservationReader) ReadObservation(_ context.Context, req tool.ObservationReadRequest) (string, error) {
	f.calls++
	f.got = req
	return f.result, f.err
}

func invokeReadObservation(t *testing.T, reader tool.ObservationReader, params map[string]any) *tool.ContentBlock {
	t.Helper()
	ro := tool.NewReadObservationTool(reader)
	input, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	uctx := &tool.UseContext{SessionID: "s1", WorkDir: t.TempDir()}
	result, err := ro.Invoke(context.Background(), uctx, input)
	if err != nil {
		t.Fatalf("Invoke() = %v", err)
	}
	return result
}

func TestReadObservationToolReturnsOriginal(t *testing.T) {
	reader := &fakeObservationReader{result: "full fetch body"}
	placeholder := "[observation-masked] tool=Fetch observation_id=toolu_old-abc"
	result := invokeReadObservation(t, reader, map[string]any{"ref": placeholder})
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Text)
	}
	if result.Text != "full fetch body" {
		t.Fatalf("result = %q", result.Text)
	}
	if reader.got.Ref != placeholder || reader.got.SessionID != "s1" {
		t.Fatalf("request = %#v", reader.got)
	}
}

func TestReadObservationToolRequiresRef(t *testing.T) {
	reader := &fakeObservationReader{result: "unused"}
	result := invokeReadObservation(t, reader, map[string]any{})
	if !result.IsError || !strings.Contains(result.Text, "required") {
		t.Fatalf("result = %q (isError=%v), want required-field error", result.Text, result.IsError)
	}
	if reader.calls != 0 {
		t.Fatalf("reader should not be called, got %d", reader.calls)
	}
}

func TestReadObservationToolWithoutReader(t *testing.T) {
	result := invokeReadObservation(t, nil, map[string]any{"observation_id": "toolu_old-abc"})
	if !result.IsError || !strings.Contains(result.Text, "not enabled") {
		t.Fatalf("result = %q (isError=%v), want disabled-store error", result.Text, result.IsError)
	}
}

func TestReadObservationToolSafetyFlags(t *testing.T) {
	ro := tool.NewReadObservationTool(&fakeObservationReader{})
	if ro.Name() != "ReadObservation" {
		t.Fatalf("name = %q, want ReadObservation", ro.Name())
	}
	if !ro.IsReadOnly(nil) {
		t.Fatal("ReadObservation only reads; it must be read-only so plan mode allows it")
	}
	if ro.IsDestructive(nil) {
		t.Fatal("ReadObservation must not be destructive")
	}
	if !ro.IsConcurrencySafe(nil) {
		t.Fatal("ReadObservation should be concurrency safe")
	}
}

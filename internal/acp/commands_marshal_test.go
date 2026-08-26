package acp

import (
	"encoding/json"
	"testing"
)

func TestAvailableCommandsUpdateMarshal(t *testing.T) {
	u := SessionUpdate{SessionUpdate: "available_commands_update", AvailableCommands: availableCommands()}
	raw, err := json.Marshal(SessionUpdateParams{SessionID: "s1", Update: u})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", raw)
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	update := payload["update"].(map[string]any)
	if update["sessionUpdate"] != "available_commands_update" {
		t.Fatalf("sessionUpdate=%v", update["sessionUpdate"])
	}
	cmds, ok := update["availableCommands"].([]any)
	if !ok || len(cmds) == 0 {
		t.Fatalf("availableCommands missing: %s", raw)
	}
}

func TestAvailableCommandsUpdateMarshalEmptyList(t *testing.T) {
	// Empty command catalogs must still serialize the field so clients do not
	// drop the update type and lose slash completion state.
	u := SessionUpdate{SessionUpdate: "available_commands_update", AvailableCommands: []AvailableCommand{}}
	raw, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["sessionUpdate"] != "available_commands_update" {
		t.Fatalf("payload=%s", raw)
	}
	cmds, ok := payload["availableCommands"].([]any)
	if !ok {
		t.Fatalf("availableCommands omitted for empty update: %s", raw)
	}
	if len(cmds) != 0 {
		t.Fatalf("expected empty list, got %#v", cmds)
	}
}

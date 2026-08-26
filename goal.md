# Goal

ACP p0+p1+p2: Agent Client Protocol stdio server so editors can drive the same agent loop as the TUI.

## P0 — Core stdio agent

- [x] JSON-RPC 2.0 newline-delimited stdio (`internal/acp/conn.go`).
- [x] `initialize` advertises loadSession, image, and embeddedContext.
- [x] `session/new` creates a session and returns sessionId + modes.
- [x] `session/prompt` streams thought, message, tool, and usage updates.
- [x] CLI: `solcode --acp` and `solcode acp` (mutually exclusive with `--prompt`).
- [x] README features, first-run, and `-acp` flag document ACP.

## P1 — Permissions, cancel, modes

- [x] `session/request_permission` with allow/reject once/always.
- [x] `session/cancel` stops an in-flight prompt (`stopReason=cancelled`).
- [x] `session/set_mode` applies permission modes and emits `current_mode_update`.

## P2 — Session load and multimodal prompts

- [x] `session/load` replays saved user/assistant history.
- [x] Image blocks are written under `.solcode/acp-uploads` and attached with `@`.
- [x] `resource_link` / embedded context is folded into the prompt text.
- [x] Session bootstrap emits available commands and current mode.

## P3 — Tool call diffs for client UI

- [x] Emit ACP `type: "diff"` content on Edit/Write/MultiEdit/MultiWrite tool calls.
- [x] Include `locations` for follow-along.
- [x] Attach the same diff preview on `session/request_permission` when input is available.

## P4 — ACP plan updates and client filesystem

- [x] Map successful TodoWrite calls to complete `agent_plan_update` entries.
- [x] Add per-session, capability-gated `fs/read_text_file` / `fs/write_text_file` client filesystem access.
- [x] Route text file tools through the client filesystem when the matching capability is advertised, with local fallback otherwise.

## Validation

- [x] `go test ./internal/acp/ ./internal/session/` — ok.

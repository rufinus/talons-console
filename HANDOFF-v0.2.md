# talons-console v0.2 HANDOFF.md

## Branch: feature_commands
## Git Attribution: ALL commits must use `Ludwig Ruderstaller <lr@cwd.at>`
## Set before any commit: `git config user.name "Ludwig Ruderstaller" && git config user.email "lr@cwd.at"`

---

## Project Context

This is v0.2 of talons-console (OpenClaw Gateway CLI), built in Go with Bubble Tea TUI.
- **Repo:** `~/workspace/projects/talons-console`
- **Branch:** `feature_commands`
- **Docs:** `concepts/v0.2/` — contains PRD, architecture, design docs, and acceptance criteria

### Key existing packages
- `internal/config/` — config loading, validation (TASK-001/002 modify this)
- `internal/gateway/` — WebSocket, auth, protocol, client
- `internal/tui/` — Bubble Tea TUI model + components
- `internal/version/` — version/update check
- `cmd/talons/` — CLI entry point (Cobra commands)

### New package for v0.2
- `internal/commands/` — slash command framework (created in TASK-003/004)

---

## Completed Tasks

(Updated as workers finish)

---

## Worker Notes

(Populated as workers report back)

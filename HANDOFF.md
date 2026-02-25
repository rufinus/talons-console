# talons-console — Worker Handoff Notes

Passed to each Daedalus worker as context. Keep slim.

## Repo
- Path: `~/workspace/projects/talons-console/`
- Branch: `feature/mvp` (always work here)
- Module: `github.com/rufinus/talons-console`
- Commit and push after EACH task: `git pull --rebase origin feature/mvp && git push origin feature/mvp`

## Design Docs
- `concepts/mvp/design.md` — architecture, interfaces, directory structure
- `concepts/mvp/protocol-research.md` — OpenClaw WebSocket protocol
- `concepts/mvp/prd.md` — requirements
- `concepts/go-best-practices.md` — Go coding standards

## Completed Tasks
- ✅ TASK-001: Scaffolding done. All dirs, deps, Makefile in place. `go build ./...` passes.
- ✅ TASK-003: Core interfaces — `GatewayClient`, `WebSocketConn`, `ConnectionState` (5 states), 6 sentinel errors, `OutboundMessage`, `InboundEvent`, `InboundKind` (8 kinds, KindUnknown=0).
- ✅ TASK-016: Version info + update check — `version.String()`, `version.CheckUpdate(ctx)`, `githubReleasesURL` is a `var` (not const) for test override. 91.4% coverage.
- ✅ TASK-012: Markdown rendering + terminal detection. `RenderMarkdown(content, width)` singleton glamour renderer (width locked at first call — MVP). `DetectTerminal()` uses golang.org/x/term for size. Removed glamour blank import from tools.go.

## Key Notes
- `tools.go` (build tag `//go:build tools`) keeps all deps alive. Remove a blank import when your package imports that dep for real.
- `cmd/talons/root.go` already exists — Cobra root command pre-defined. TASK-015 builds on it.
- lipgloss is a prerelease version (glamour dependency) — don't upgrade it.
- `OutboundMessage.Payload` is `any` — callers marshal into it; client JSON-encodes the full payload.
- `InboundKind` starts at `KindUnknown = 0` — guard against zero-value events being misread as real ones.
- `GatewayClient` is the only interface TUI should depend on — never import the concrete `Client` struct directly.
- `ErrShutdown` message: `"client is shutting down"` (present continuous).

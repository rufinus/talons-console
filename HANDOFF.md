# talons-console — Worker Handoff Notes

Passed to each Daedalus worker as context. Keep slim.

## Repo
- Path: `~/workspace/projects/talons-console/`
- Branch: `feature/mvp` (always work here)
- Module: `github.com/rufinus/talons-console`
- Commit and push after EACH task: `git pull --rebase origin feature/mvp && git push origin feature/mvp`

## ⚠️ CRITICAL: Git Author — HARD REQUIREMENT
Every commit MUST use Ludwig's identity. Run this FIRST before any commit:
```bash
git -C ~/workspace/projects/talons-console config user.name "Ludwig Ruderstaller"
git -C ~/workspace/projects/talons-console config user.email "lr@cwd.at"
```
Or use `--author` on every commit:
```bash
git commit --author="Ludwig Ruderstaller <lr@cwd.at>" -m "..."
```
- NO "Co-Authored-By" trailers
- NO agent attribution of any kind
- Violation = removal from project

## Design Docs
- `concepts/mvp/design.md` — architecture, interfaces, directory structure
- `concepts/mvp/protocol-research.md` — OpenClaw WebSocket protocol
- `concepts/mvp/prd.md` — requirements
- `concepts/go-best-practices.md` — Go coding standards

## Completed Tasks
- ✅ TASK-001: Scaffolding done. All dirs, deps, Makefile in place. `go build ./...` passes.
- ✅ TASK-003: Core interfaces — `GatewayClient`, `WebSocketConn`, `ConnectionState` (5 states), 6 sentinel errors. Types moved to protocol.go.
- ✅ TASK-002: Linting (`.golangci.yml` v2), CI workflow (`.github/workflows/ci.yml`), Makefile updated. Also produced TASK-004 and TASK-005 work (see below).
- ✅ TASK-004 (done by TASK-002): Full protocol types in `internal/gateway/protocol.go` — `OutboundFrame`, `ChatSendParams`, `ConnectParams`, `HelloOKPayload`, `ConnectChallengePayload`, `ChatEventPayload`, all inbound types, `ParseInbound()`. 83.1% coverage.
- ✅ TASK-005 (done by TASK-002): `wsConn` wrapper + `MockWebSocketConn` in `internal/gateway/websocket.go`. Mock has `EnqueueRead`, `SentMessages()`, `SetWriteError`.
- ✅ TASK-016: Version info + update check — `version.String()`, `version.CheckUpdate(ctx)`, `githubReleasesURL` is a `var` for test override. 91.4% coverage.
- ✅ TASK-012: Markdown rendering + terminal detection. `RenderMarkdown(content, width)` singleton glamour renderer. `DetectTerminal()` uses golang.org/x/term.
- ✅ TASK-010: Config system — `Load(v *viper.Viper)`, `Validate()`, `CheckFilePermissions()`. Uses explicit `BindEnv` (AutomaticEnv alone doesn't hydrate via Unmarshal). 91.8% coverage.
- ✅ TASK-016: Version info + update check — `version.String()`, `version.CheckUpdate(ctx)`, `githubReleasesURL` is a `var` (not const) for test override. 91.4% coverage.
- ✅ TASK-012: Markdown rendering + terminal detection. `RenderMarkdown(content, width)` singleton glamour renderer (width locked at first call — MVP). `DetectTerminal()` uses golang.org/x/term for size. Removed glamour blank import from tools.go.
- ✅ TASK-010: Config system — `Load(v *viper.Viper)`, `Validate()`, `CheckFilePermissions()`. Uses explicit `BindEnv` for all keys (AutomaticEnv alone doesn't hydrate via Unmarshal). 91.8% coverage. Removed viper blank import from tools.go.

## Key Notes
- `tools.go` (build tag `//go:build tools`) keeps all deps alive. Remove a blank import when your package imports that dep for real.
- `cmd/talons/root.go` already exists — Cobra root command pre-defined. TASK-015 builds on it.
- lipgloss is a prerelease version (glamour dependency) — don't upgrade it.
- `OutboundMessage.Payload` is `any` — callers marshal into it; client JSON-encodes the full payload.
- `InboundKind` starts at `KindUnknown = 0` — guard against zero-value events being misread as real ones.
- `GatewayClient` is the only interface TUI should depend on — never import the concrete `Client` struct directly.
- `ErrShutdown` message: `"client is shutting down"` (present continuous).
- Config: `Load(v)` must be called AFTER all `v.BindPFlag(...)` cobra flag bindings. Call `CheckFilePermissions(path)` separately at CLI layer — Load doesn't call it.
- Config env key replacer maps `.` and `-` → `_`: `timeout_ms` → `TALONS_TIMEOUT_MS`.
- Session PID files (TASK-011): `os.UserConfigDir()/talons/sessions/<agent>-<session>.pid`
- `RenderMarkdown` singleton: width locked at first call. Not a problem for MVP.

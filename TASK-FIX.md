# talons-console Fix Task

## Branch: feature_commands
## Git: ALL commits must use `Ludwig Ruderstaller <lr@cwd.at>` (git config already set)

---

## How OpenClaw's own TUI implements these commands

This is the reference implementation. Replicate it exactly.

### `/model <name>` → `sessions.patch` with `model` field
```js
const result = await client.patchSession({
    key: state.currentSessionKey,
    model: args
});
chatLog.addSystem(`model set to ${args}`);
applySessionInfoFromPatch(result);
```

### `/thinking <level>` → `sessions.patch` with `thinkingLevel` field
```js
const result = await client.patchSession({
    key: state.currentSessionKey,
    thinkingLevel: args
});
chatLog.addSystem(`thinking set to ${args}`);
```

### `SessionsPatchParams` (from Gateway schema):
```
key            string   (required) — the session key
model          string?  (optional) — model override, null to clear
thinkingLevel  string?  (optional) — thinking level override
verboseLevel   string?  (optional)
reasoningLevel string?  (optional)
... (other fields not needed now)
```

The `sessions.patch` request is fire-and-forget (no response handling needed for now).

### `/agent <name>` — change `currentAgentId`, rebuild session key, reload history
The session key format is `agent:<agentId>:<sessionName>`. When agent changes, reconstruct the session key. This is already partially implemented via `ApplyToSendParams` — verify it works correctly.

---

## Fix 1: CI Lint — gofmt (TRIVIAL)

**File:** `internal/tui/state_test.go`
**Problem:** Two extra blank lines at line ~131-132 fail gofmt check.

**Fix:** `gofmt -w internal/tui/state_test.go`

---

## Fix 2: CI Integration Tests

**File:** `test/integration/gateway_test.go`
**Problem:** Uses struct fields that don't exist:
- `Content` → correct field is `Message`
- `AgentID` → doesn't exist on ChatSendParams (agent is via SessionKey)
- `Model` → doesn't exist on ChatSendParams (model is via sessions.patch)

**Fix:** Update each test to use valid `ChatSendParams` fields:
- Replace `Content:` with `Message:`
- For agent tests: use `SessionKey: "agent:integration-test-2:main"` instead of `AgentID:`
- For model tests: remove `Model:` from ChatSendParams; test model via a separate `sessions.patch` call or simply verify the sessions.patch roundtrip

---

## Fix 3: `input.go` — Reset() Value Receiver Bug (CRITICAL)

**File:** `internal/tui/input.go`
**Problem:** `Reset()` is a VALUE receiver — `m.input.Reset()` in app.go is a no-op. Input never clears after pressing Enter.

**Fix:**
```go
// Change from:
func (m InputModel) Reset() {
    m.textarea.Reset()
}

// To:
func (m *InputModel) Reset() {
    m.textarea.Reset()
}
```

---

## Fix 4: `/model` and `/thinking` — Wire to `sessions.patch`

### 4a. Add `SessionsPatchParams` to `internal/gateway/protocol.go`:
```go
// SessionsPatchParams is the params block for the sessions.patch method.
type SessionsPatchParams struct {
    Key           string  `json:"key"`
    Model         *string `json:"model,omitempty"`         // null to clear, string to set
    ThinkingLevel *string `json:"thinkingLevel,omitempty"` // null to clear, string to set
}
```

### 4b. Add `PatchSession` to the GatewayClient interface (`internal/gateway/interfaces.go`):
```go
PatchSession(params SessionsPatchParams) error
```

### 4c. Implement `PatchSession` in `internal/gateway/client.go`:
```go
func (c *Client) PatchSession(params SessionsPatchParams) error {
    return c.Send(OutboundMessage{
        Type:    "sessions.patch",
        Payload: params,
    })
}
```

### 4d. Add `PatchSession` to `HandlerContext` interface (`internal/commands/handler.go`):
```go
GetSessionKey() string
PatchSession(params gateway.SessionsPatchParams) error
```

Wait — `internal/commands` should NOT import `internal/gateway` (circular risk). Instead, define a minimal patch struct in `internal/commands`:

```go
// SessionPatch holds the fields for a sessions.patch call.
// Only includes fields used by command handlers.
type SessionPatch struct {
    Model         *string
    ThinkingLevel *string
}
```

And in HandlerContext:
```go
GetSessionKey() string
PatchSession(patch SessionPatch) error
```

### 4e. Implement in `internal/tui/app.go`:
```go
func (m *Model) GetSessionKey() string {
    agent := m.state.GetAgent()
    session := m.state.GetSession()
    return fmt.Sprintf("agent:%s:%s", agent, session)
}

func (m *Model) PatchSession(patch commands.SessionPatch) error {
    params := gateway.SessionsPatchParams{
        Key:           m.GetSessionKey(),
        Model:         patch.Model,
        ThinkingLevel: patch.ThinkingLevel,
    }
    return m.client.PatchSession(params)
}
```

### 4f. Update `HandleModel` in `internal/commands/handlers_state.go`:

```go
func HandleModel(ctx HandlerContext, args []string) tea.Cmd {
    if len(args) == 0 {
        cur := ctx.GetModel()
        display := cur
        if display == "" {
            display = "(default)"
        }
        ctx.AppendSystemMessage(fmt.Sprintf("Current model: %s\nUsage: /model <model-id>", display))
        return nil
    }

    id := strings.TrimPrefix(args[0], "/")
    // ... existing validation ...

    if ctx.GetModel() == id {
        ctx.AppendSystemMessage(fmt.Sprintf("Already using model '%s'", id))
        return nil
    }

    return func() tea.Msg {
        if err := ctx.PatchSession(SessionPatch{Model: &id}); err != nil {
            ctx.AppendSystemMessage(CmdError("model", fmt.Sprintf("patch failed: %s", err)))
            return nil
        }
        ctx.SetModel(id)
        ctx.AppendSystemMessage(fmt.Sprintf("Model set to '%s'", id))
        return nil
    }
}
```

### 4g. Update `HandleThinking` in `internal/commands/handlers_state.go`:

Currently sends `thinking` as per-message ChatSendParams field. Change to `sessions.patch` with `thinkingLevel`:

```go
func HandleThinking(ctx HandlerContext, args []string) tea.Cmd {
    if len(args) == 0 {
        cur := ctx.GetThinking()
        display := cur
        if display == "" {
            display = "(default)"
        }
        ctx.AppendSystemMessage(fmt.Sprintf("Current thinking: %s\nUsage: /thinking <off|minimal|low|medium|high>", display))
        return nil
    }

    level := strings.ToLower(args[0])
    if _, ok := validThinkingLevels[level]; !ok {
        ctx.AppendSystemMessage(CmdError("thinking", "must be one of: off, minimal, low, medium, high"))
        return nil
    }

    return func() tea.Msg {
        if err := ctx.PatchSession(SessionPatch{ThinkingLevel: &level}); err != nil {
            ctx.AppendSystemMessage(CmdError("thinking", fmt.Sprintf("patch failed: %s", err)))
            return nil
        }
        ctx.SetThinking(level)
        ctx.AppendSystemMessage(fmt.Sprintf("Thinking set to '%s'", level))
        return nil
    }
}
```

### 4h. Remove `thinking` from `ApplyToSendParams` in `internal/tui/state.go`

Since thinking is now set via sessions.patch (session-level), it should NOT be re-sent on every chat.send. Remove `params.Thinking = thinking` from `ApplyToSendParams`.

---

## Fix 5: UI Improvements

The UI needs to be significantly better. Here's the target design:

### Color palette (Catppuccin Mocha — consistent with OpenClaw):
```go
var (
    colorBase    = lipgloss.Color("#1e1e2e") // background
    colorText    = lipgloss.Color("#cdd6f4") // text
    colorGreen   = lipgloss.Color("#a6e3a1")
    colorBlue    = lipgloss.Color("#89b4fa")
    colorYellow  = lipgloss.Color("#f9e2af")
    colorRed     = lipgloss.Color("#f38ba8")
    colorMauve   = lipgloss.Color("#cba6f7")
    colorOverlay = lipgloss.Color("#313244") // subtle background
    colorMuted   = lipgloss.Color("#585b70")
    colorPeach   = lipgloss.Color("#fab387")
)
```

### Header (`internal/tui/header.go`):
Full-width styled bar showing:
```
● Connected  main / main                        talons v0.2.0
```
- Background: `colorBase` or `colorOverlay`
- Connection dot: green (●) connected, yellow (◌) connecting/authenticating, red (○) disconnected
- Bold connection state
- Right-aligned version

```go
func (m HeaderModel) View() string {
    dot, dotStyle := m.statusDot()
    left := dotStyle.Render(dot) + " " + m.connectionLabel()
    center := m.agent + " / " + m.session
    right := "talons " + m.version

    // Full width layout: left | center | right
    // Use lipgloss.JoinHorizontal or manual padding
    available := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
    if available < 0 { available = 0 }
    pad := strings.Repeat(" ", available/2)

    content := headerStyle.Width(m.width).Render(
        left + pad + center + pad + right,
    )
    return content
}
```

Use a background style:
```go
headerStyle = lipgloss.NewStyle().
    Background(colorBase).
    Foreground(colorText).
    Padding(0, 1)
```

### Messages Area:
- User messages: right-side styled with blue prefix `▶ you`
- Assistant messages: green prefix `◀ assistant` (or agent name)
- System messages: muted italic, centered `─── system ───`
- Timestamps: small muted suffix

Example:
```
▶ you                                          12:34
hello world

◀ assistant                                    12:34
Hi there! How can I help?

─── model set to 'anthropic/claude-opus-4-6' ───
```

### Input Area:
- Border around the textarea
- Status footer below: `agent: main  |  session: main  |  model: default  |  think: off`

```go
inputBorderStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(colorMuted).
    Padding(0, 1)

statusLineStyle = lipgloss.NewStyle().
    Foreground(colorMuted).
    Italic(true)
```

The input `View()` should return:
```
inputBorderStyle.Render(m.textarea.View()) + "\n" + statusLine
```

The status line is passed in from the parent model (it has agent/session/model/thinking state). Add a `SetStatus(s string)` method to InputModel.

### Centralize styles:
Create `internal/tui/styles.go` with all style variables. Remove inline style definitions from other files.

---

## Test Requirements

After all fixes:
- `gofmt -l ./...` → no output
- `go vet ./...` → clean  
- `go test ./...` → all pass (integration tests excluded since they need real gateway)
- `golangci-lint run` → clean
- Build: `go build ./cmd/talons/` → succeeds

---

## Commit Strategy

1. `fix: pointer receiver on InputModel.Reset — input clears after command`
2. `fix: gofmt state_test.go`
3. `fix: integration tests use correct ChatSendParams fields`
4. `feat: wire /model and /thinking to sessions.patch (matches OpenClaw TUI)`
5. `feat: UI overhaul — header bar, message styling, input border, status footer`

All commits: author Ludwig Ruderstaller <lr@cwd.at>

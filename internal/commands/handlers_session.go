package commands

import (
	"fmt"
	"regexp"

	tea "github.com/charmbracelet/bubbletea"
)

// agentNameRe validates agent names: alphanumeric, hyphens, underscores, dots; max 64 chars.
// Compiled once at package level per quality requirements.
var agentNameRe = regexp.MustCompile(`^[a-zA-Z0-9\-_.]{1,64}$`)

// sessionKeyRe validates session keys: alphanumeric, hyphens, underscores; max 64 chars.
// Note: dots are intentionally excluded from session keys (they are allowed in agent names).
// Compiled once at package level per quality requirements.
var sessionKeyRe = regexp.MustCompile(`^[a-zA-Z0-9\-_]{1,64}$`)

// HandleAgent implements the /agent soft-switch command.
//
// Soft-switch design (see architecture.md §3): the Gateway protocol attaches
// agent/session identity to each chat.send message via AgentID in ChatSendParams,
// not to the connection handshake. Switching agent therefore requires only an
// in-memory state update — no reconnection. History is session-scoped, so
// requesting history after an agent switch would surface the old session's
// messages regardless of agent, which is confusing and incorrect; therefore
// RequestHistory is NOT called here.
func HandleAgent(ctx HandlerContext, args []string) tea.Cmd {
	if len(args) == 0 {
		name := ctx.GetAgent()
		display := name
		if display == "" {
			display = "(default)"
		}
		ctx.AppendSystemMessage(fmt.Sprintf("Current agent: %s\nUsage: /agent <name>", display))
		return nil
	}

	name := args[0]

	// Validate before any state mutation.
	if !agentNameRe.MatchString(name) {
		ctx.AppendSystemMessage(fmt.Sprintf(
			"⚠ /agent: invalid name %q — only alphanumeric characters, hyphens (-), "+
				"underscores (_), and dots (.) are permitted; max 64 characters",
			name,
		))
		return nil
	}

	// No-op if already using this agent.
	if name == ctx.GetAgent() {
		ctx.AppendSystemMessage(fmt.Sprintf("Already using agent '%s'", name))
		return nil
	}

	// Apply state changes.
	ctx.SetAgent(name)
	ctx.ClearMessages()
	ctx.UpdateHeader()
	ctx.AppendSystemMessage(fmt.Sprintf("Switched to agent '%s'", name))

	// Soft switch: no Reconnect, no RequestHistory (see architecture.md §3).
	return nil
}

// HandleSession implements the /session soft-switch command.
//
// Soft-switch design (see architecture.md §3): the Gateway protocol attaches
// agent/session identity to each chat.send message via SessionKey in ChatSendParams,
// not to the connection handshake. Switching session requires only an in-memory
// state update — no reconnection. An async tea.Cmd is returned to fetch the new
// session's history; history arrives asynchronously as KindHistory events handled
// by the existing TUI machinery.
func HandleSession(ctx HandlerContext, args []string) tea.Cmd {
	if len(args) == 0 {
		key := ctx.GetSession()
		display := key
		if display == "" {
			display = "(none)"
		}
		ctx.AppendSystemMessage(fmt.Sprintf("Current session: %s\nUsage: /session <key>", display))
		return nil
	}

	key := args[0]

	// Validate before any state mutation.
	if !sessionKeyRe.MatchString(key) {
		ctx.AppendSystemMessage(fmt.Sprintf(
			"⚠ /session: invalid key %q — only alphanumeric characters, hyphens (-), "+
				"and underscores (_) are permitted; max 64 characters",
			key,
		))
		return nil
	}

	// No-op if already in this session.
	if key == ctx.GetSession() {
		ctx.AppendSystemMessage(fmt.Sprintf("Already in session '%s'", key))
		return nil
	}

	// Apply state changes.
	ctx.SetSession(key)
	ctx.ClearMessages()
	ctx.UpdateHeader()
	ctx.AppendSystemMessage(fmt.Sprintf("Switched to session '%s'", key))

	// Soft switch: no Reconnect (see architecture.md §3).
	// Return async cmd to load history for the new session.
	return func() tea.Msg {
		if err := ctx.RequestHistory(key); err != nil {
			return systemErrorMsg{Err: fmt.Errorf("history request failed: %w", err)}
		}
		return nil
	}
}

// systemErrorMsg is a local stand-in for tui.SystemErrorMsg (defined in
// internal/tui/events.go by TASK-009). The TUI integration (TASK-010) will
// wire the real type; this local type is used only within this package's tests.
type systemErrorMsg struct {
	Err error
}

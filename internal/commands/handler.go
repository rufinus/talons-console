// Package commands provides the slash-command framework for talons-console.
// It is completely decoupled from the TUI layer and Bubble Tea model types;
// the TUI wires up the HandlerContext interface in a later phase.
package commands

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// HandlerFunc is the signature for all slash-command handler functions.
// ctx is implemented by the TUI Model in Phase 5; args are the whitespace-split
// arguments that followed the command name on the input line.
type HandlerFunc func(ctx HandlerContext, args []string) tea.Cmd

// HandlerContext is the interface that the TUI Model implements so that
// command handlers can interact with application state without importing
// internal/tui.
type HandlerContext interface {
	// Message display
	AppendSystemMessage(msg string)
	ClearMessages()

	// Session state
	GetAgent() string
	SetAgent(name string)
	GetSession() string
	SetSession(key string)
	GetModel() string
	SetModel(model string)
	GetThinking() string
	SetThinking(level string)
	GetTimeoutMs() int
	SetTimeoutMs(ms int)

	// Gateway info
	GetGatewayURL() string
	IsConnected() bool

	// Runtime info
	GetVersion() string
	GetUptime() time.Duration
	GetMsgSent() int
	GetMsgRecv() int

	// Gateway operations
	RequestHistory(sessionKey string) error
	Reconnect(ctx context.Context) error
	CloseGateway() error

	// Display helpers
	UpdateHeader()
	GetWidth() int
}

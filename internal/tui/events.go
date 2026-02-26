package tui

import (
	"github.com/rufinus/talons-console/internal/commands"
	"github.com/rufinus/talons-console/internal/gateway"
)

// GatewayEventMsg is sent when a message is received from the Gateway.
type GatewayEventMsg struct {
	Event gateway.InboundEvent
}

// ConnectionStateMsg is sent when connection state changes.
type ConnectionStateMsg struct {
	State gateway.ConnectionState
}

// SendRequestMsg is sent when user wants to send a message.
type SendRequestMsg struct {
	Text string
}

// TerminalResizeMsg indicates terminal dimensions changed.
type TerminalResizeMsg struct {
	Width  int
	Height int
}

// QuitMsg signals application shutdown.
type QuitMsg struct{}

// ReconnectedMsg is sent when a user-initiated /reconnect succeeds.
// Aliased from commands package to avoid import cycles.
type ReconnectedMsg = commands.ReconnectedMsg

// SystemErrorMsg is sent when a command-level operation fails and the error
// should be surfaced to the user via the viewport.
// Aliased from commands package to avoid import cycles.
type SystemErrorMsg = commands.SystemErrorMsg

// UpdateHeaderMsg signals the TUI to refresh the header component.
type UpdateHeaderMsg struct{}

// ClearMessagesMsg signals the TUI to clear the message viewport.
type ClearMessagesMsg struct{}

// firstLaunchHintMsg is an internal message that triggers the one-time hint.
type firstLaunchHintMsg struct{}

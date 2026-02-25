package tui

import (
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

package gateway

import (
	"context"
	"time"
)

// GatewayClient abstracts all Gateway WebSocket operations.
// The TUI layer depends only on this interface, never on concrete WebSocket types.
type GatewayClient interface {
	// Connect establishes the WebSocket connection and authenticates.
	// It blocks until authentication succeeds or an error occurs.
	Connect(ctx context.Context) error

	// Send queues an outbound message for delivery to the Gateway.
	// If the client is disconnected, the message is buffered for reconnect.
	Send(msg OutboundMessage) error

	// Messages returns a read-only channel of inbound events.
	// The channel is closed when the client is permanently shut down.
	Messages() <-chan InboundEvent

	// Close shuts down the connection and all background goroutines.
	Close() error

	// State returns the current connection state.
	State() ConnectionState
}

// WebSocketConn abstracts the gorilla/websocket connection for testing.
// Inject a mock implementation to test gateway logic without a real network.
type WebSocketConn interface {
	// ReadMessage reads the next message from the WebSocket connection.
	ReadMessage() (messageType int, p []byte, err error)

	// WriteMessage writes a message to the WebSocket connection.
	WriteMessage(messageType int, data []byte) error

	// Close closes the WebSocket connection.
	Close() error

	// SetReadDeadline sets the deadline for future ReadMessage calls.
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline sets the deadline for future WriteMessage calls.
	SetWriteDeadline(t time.Time) error
}

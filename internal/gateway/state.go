package gateway

// ConnectionState represents the current state of the gateway connection.
type ConnectionState int

const (
	StateDisconnected   ConnectionState = iota // not connected
	StateConnecting                            // TCP/WebSocket handshake in progress
	StateAuthenticating                        // auth challenge/response in progress
	StateConnected                             // fully connected and authenticated
	StateReconnecting                          // connection lost, attempting reconnection
)

// String returns a human-readable name for the ConnectionState.
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateAuthenticating:
		return "authenticating"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

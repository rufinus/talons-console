package gateway

import "errors"

// Sentinel errors returned by the gateway package.
var (
	// ErrAuthFailed is returned when authentication is rejected by the Gateway.
	ErrAuthFailed = errors.New("authentication failed")

	// ErrAuthTimeout is returned when the auth handshake exceeds the deadline.
	ErrAuthTimeout = errors.New("authentication timed out")

	// ErrConnectionClosed is returned when an operation is attempted on a closed connection.
	ErrConnectionClosed = errors.New("connection closed")

	// ErrQueueFull is returned when the outbound message queue is at capacity.
	ErrQueueFull = errors.New("message queue full")

	// ErrInvalidConfig is returned when the supplied configuration fails validation.
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrShutdown is returned when an operation is attempted after Close() has been called.
	ErrShutdown = errors.New("client is shutting down")
)

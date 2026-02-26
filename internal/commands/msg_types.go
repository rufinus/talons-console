package commands

import "time"

// ReconnectedMsg is returned by handleReconnect on success.
// It is defined here (not in internal/tui) so that command handlers can
// return it without creating an import cycle.
type ReconnectedMsg struct {
	At time.Time
}

// SystemErrorMsg is returned by handlers when a command-level operation fails
// and the error should be surfaced to the user via the viewport.
type SystemErrorMsg struct {
	Err error
}

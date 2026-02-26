package commands

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rufinus/talons-console/internal/tui"
)

// reconnectTimeout is the maximum time allowed for a user-initiated reconnect.
const reconnectTimeout = 30 * time.Second

// handleReconnect implements the /reconnect command.
// It appends a status message immediately, then returns an async tea.Cmd that
// closes the current WebSocket and re-dials the Gateway with a 30-second
// timeout. On success it returns tui.ReconnectedMsg; on failure tui.SystemErrorMsg.
func handleReconnect(ctx HandlerContext, _ []string) tea.Cmd {
	ctx.AppendSystemMessage("Reconnecting...")

	return func() tea.Msg {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), reconnectTimeout)
		defer cancel()

		if err := ctx.Reconnect(timeoutCtx); err != nil {
			return tui.SystemErrorMsg{Err: fmt.Errorf("reconnect failed: %w", err)}
		}

		return tui.ReconnectedMsg{At: time.Now()}
	}
}

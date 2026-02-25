package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/rufinus/talons-console/internal/gateway"
	"github.com/rufinus/talons-console/internal/version"
)

// HeaderModel displays connection status, agent, and session information.
type HeaderModel struct {
	connectionState gateway.ConnectionState
	agent           string
	session         string
	version         string
	width           int
}

// NewHeaderModel creates a new HeaderModel.
func NewHeaderModel(agent, session string) HeaderModel {
	return HeaderModel{
		connectionState: gateway.StateDisconnected,
		agent:           agent,
		session:         session,
		version:         version.Version,
	}
}

// SetConnectionState updates the connection state display.
func (m *HeaderModel) SetConnectionState(state gateway.ConnectionState) {
	m.connectionState = state
}

// SetSize updates the header width.
func (m *HeaderModel) SetSize(width int) {
	m.width = width
}

// View renders the header.
func (m HeaderModel) View() string {
	stateStr := m.stateLabel()
	stateStyle := m.stateStyle()

	left := stateStyle.Render(stateStr)
	right := fmt.Sprintf("talons %s", m.version)

	// Build status string
	var status strings.Builder
	status.WriteString(m.agent)
	status.WriteString(" / ")
	status.WriteString(m.session)

	// Layout: [state] agent/session | talons vX.X.X
	// Use spaces to pad to width
	content := fmt.Sprintf("%s %s | %s", left, status.String(), right)
	if len(content) < m.width {
		content += strings.Repeat(" ", m.width-len(content))
	}

	return content
}

func (m HeaderModel) stateLabel() string {
	switch m.connectionState {
	case gateway.StateConnected:
		return "Connected"
	case gateway.StateConnecting:
		return "Connecting..."
	case gateway.StateAuthenticating:
		return "Authenticating..."
	case gateway.StateReconnecting:
		return "Reconnecting..."
	default:
		return "Disconnected"
	}
}

func (m HeaderModel) stateStyle() lipgloss.Style {
	switch m.connectionState {
	case gateway.StateConnected:
		return headerConnected
	default:
		return headerDefault
	}
}

var (
	headerConnected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")). // green
			Bold(true)
	headerDefault = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9E2AF")). // yellow
			Bold(true)
)

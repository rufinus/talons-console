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

// SetAgent updates the agent field displayed in the header.
func (m *HeaderModel) SetAgent(name string) {
	m.agent = name
}

// SetSession updates the session field displayed in the header.
func (m *HeaderModel) SetSession(key string) {
	m.session = key
}

// SetSize updates the header width.
func (m *HeaderModel) SetSize(width int) {
	m.width = width
}

// View renders the header as a full-width styled bar:
//
//	● Connected  main / main                        talons v0.2.0
func (m HeaderModel) View() string {
	dot, dotStyle := m.statusDot()
	connLabel := m.connectionLabel()
	left := dotStyle.Render(dot) + " " + connLabel

	center := m.agent + " / " + m.session
	right := fmt.Sprintf("talons %s", m.version)

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	centerW := lipgloss.Width(center)

	// Total inner width minus padding (2 chars from Padding(0,1) on each side = 2)
	inner := m.width - 2
	if inner < 0 {
		inner = 0
	}

	// Space available for centering after left and right sections
	sideGap := inner - leftW - rightW - centerW
	if sideGap < 2 {
		sideGap = 2
	}
	leftPad := sideGap / 2
	rightPad := sideGap - leftPad

	content := left +
		strings.Repeat(" ", leftPad) +
		center +
		strings.Repeat(" ", rightPad) +
		right

	return headerStyle.Width(m.width).Render(content)
}

func (m HeaderModel) connectionLabel() string {
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

func (m HeaderModel) statusDot() (string, lipgloss.Style) {
	switch m.connectionState {
	case gateway.StateConnected:
		return "●", headerConnectedStyle
	case gateway.StateConnecting, gateway.StateAuthenticating, gateway.StateReconnecting:
		return "◌", headerConnectingStyle
	default:
		return "○", headerDisconnectedStyle
	}
}

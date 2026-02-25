package tui

import (
	"testing"

	"github.com/rufinus/talons-console/internal/gateway"
)

func TestHeaderModel_View_Connected(t *testing.T) {
	m := NewHeaderModel("test-agent", "test-session")
	m.SetConnectionState(gateway.StateConnected)
	m.SetSize(80)

	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestHeaderModel_View_Disconnected(t *testing.T) {
	m := NewHeaderModel("test-agent", "test-session")
	m.SetConnectionState(gateway.StateDisconnected)
	m.SetSize(80)

	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestHeaderModel_View_Connecting(t *testing.T) {
	m := NewHeaderModel("test-agent", "test-session")
	m.SetConnectionState(gateway.StateConnecting)
	m.SetSize(80)

	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestHeaderModel_View_Reconnecting(t *testing.T) {
	m := NewHeaderModel("test-agent", "test-session")
	m.SetConnectionState(gateway.StateReconnecting)
	m.SetSize(80)

	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestHeaderModel_View_Authenticating(t *testing.T) {
	m := NewHeaderModel("test-agent", "test-session")
	m.SetConnectionState(gateway.StateAuthenticating)
	m.SetSize(80)

	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestHeaderModel_SetConnectionState(t *testing.T) {
	m := NewHeaderModel("test-agent", "test-session")

	m.SetConnectionState(gateway.StateConnecting)
	if m.connectionState != gateway.StateConnecting {
		t.Errorf("expected StateConnecting, got %v", m.connectionState)
	}

	m.SetConnectionState(gateway.StateConnected)
	if m.connectionState != gateway.StateConnected {
		t.Errorf("expected StateConnected, got %v", m.connectionState)
	}
}

func TestHeaderModel_SetSize(t *testing.T) {
	m := NewHeaderModel("test-agent", "test-session")
	m.SetSize(100)

	if m.width != 100 {
		t.Errorf("expected width 100, got %d", m.width)
	}
}

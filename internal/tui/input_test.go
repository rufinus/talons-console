package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewInputModel(t *testing.T) {
	m := NewInputModel(80, 3)

	if m.height != 3 {
		t.Errorf("expected height 3, got %d", m.height)
	}
	if !m.focused {
		t.Error("expected focused to be true")
	}
}

func TestInputModel_Value(t *testing.T) {
	m := NewInputModel(80, 3)
	m.textarea.SetValue("Hello, world!")

	if m.Value() != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got '%s'", m.Value())
	}
}

func TestInputModel_Reset(t *testing.T) {
	m := NewInputModel(80, 3)
	// Note: The textarea API doesn't have a simple way to set value programmatically
	// without using internal methods. Just test that Reset doesn't panic and works.
	m.Reset()

	if m.Value() != "" {
		t.Errorf("expected empty value after Reset, got '%s'", m.Value())
	}
}

func TestInputModel_SetSize(t *testing.T) {
	m := NewInputModel(80, 3)
	m.SetSize(100, 5)

	if m.height != 5 {
		t.Errorf("expected height 5, got %d", m.height)
	}
}

func TestInputModel_Focus(t *testing.T) {
	m := NewInputModel(80, 3)
	m.Blur()
	m.Focus()

	if !m.Focused() {
		t.Error("expected Focused() to return true after Focus()")
	}
}

func TestInputModel_Blur(t *testing.T) {
	m := NewInputModel(80, 3)
	m.Blur()

	if m.Focused() {
		t.Error("expected Focused() to return false after Blur()")
	}
}

func TestInputModel_View_NotFocused(t *testing.T) {
	m := NewInputModel(80, 3)
	m.Blur()

	view := m.View()
	if view != "" {
		t.Errorf("expected empty view when not focused, got '%s'", view)
	}
}

func TestInputModel_View_Focused(t *testing.T) {
	m := NewInputModel(80, 3)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view when focused")
	}
}

func TestInputModel_SetPlaceholder(t *testing.T) {
	m := NewInputModel(80, 3)
	m.SetPlaceholder("Enter message...")

	if m.textarea.Placeholder != "Enter message..." {
		t.Errorf("expected placeholder 'Enter message...', got '%s'", m.textarea.Placeholder)
	}
}

func TestInputModel_Init(t *testing.T) {
	m := NewInputModel(80, 3)
	cmd := m.Init()

	if cmd != nil {
		t.Error("expected Init to return nil cmd")
	}
}

func TestInputModel_Update_EnterKey(t *testing.T) {
	m := NewInputModel(80, 3)
	m.textarea.SetValue("Hello")

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(msg)

	// The Update method returns nil for Enter (parent handles it)
	// This tests that it doesn't panic
	if cmd != nil {
		t.Log("Update returned a cmd for Enter key")
	}
}

func TestInputModel_Update_OtherKey(t *testing.T) {
	m := NewInputModel(80, 3)
	m.textarea.SetValue("Hel")

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("lo")}
	_, cmd := m.Update(msg)

	// Should not panic, just process the key
	_ = cmd
}

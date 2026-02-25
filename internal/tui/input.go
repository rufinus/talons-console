package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// InputModel handles text input for user messages.
type InputModel struct {
	textarea textarea.Model
	height   int
	focused  bool
}

// NewInputModel creates a new InputModel.
func NewInputModel(width, height int) InputModel {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.SetWidth(width)
	ta.SetHeight(height)
	ta.Prompt = "> "
	ta.Focus()

	return InputModel{
		textarea: ta,
		height:   height,
		focused:  true,
	}
}

// Init implements tea.Model.
func (m InputModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	// Handle Enter key for sending
	if k, ok := msg.(tea.KeyMsg); ok {
		if k.Type == tea.KeyEnter {
			// Check if textarea has content - if yes, let parent handle it
			// The parent will call Value() and Reset()
			return m, nil
		}
	}

	ta, cmd := m.textarea.Update(msg)
	m.textarea = ta
	return m, cmd
}

// View implements tea.Model.
func (m InputModel) View() string {
	if !m.focused {
		return ""
	}
	return m.textarea.View()
}

// Value returns the current input value.
func (m InputModel) Value() string {
	return m.textarea.Value()
}

// Reset clears the input.
func (m InputModel) Reset() {
	m.textarea.Reset()
}

// SetSize updates the input dimensions.
func (m *InputModel) SetSize(width, height int) {
	m.height = height
	m.textarea.SetWidth(width)
	if height > 1 {
		m.textarea.SetHeight(height)
	}
}

// Focus makes the input focused.
func (m *InputModel) Focus() {
	m.focused = true
	m.textarea.Focus()
}

// Blur unfocuses the input.
func (m *InputModel) Blur() {
	m.focused = false
	m.textarea.Blur()
}

// Focused returns whether the input is focused.
func (m InputModel) Focused() bool {
	return m.focused
}

// SetPlaceholder sets the placeholder text.
func (m *InputModel) SetPlaceholder(p string) {
	m.textarea.Placeholder = p
}

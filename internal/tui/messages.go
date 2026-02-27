package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rufinus/talons-console/internal/gateway"
)

// ChatMessage represents a single message in the chat.
type ChatMessage struct {
	Role      string // "user", "assistant", "system", "tool"
	Content   string // Raw content
	Rendered  string // Cached rendered output (markdown)
	ToolName  string // For tool messages
	Timestamp time.Time
	Streaming bool // True while receiving tokens
}

// MessagesModel manages the chat viewport.
type MessagesModel struct {
	viewport        viewport.Model
	messages        []ChatMessage
	spinner         spinner.Model
	showSpinner     bool
	currentStreamID string
	width           int
	height          int
}

// NewMessagesModel creates a new MessagesModel.
func NewMessagesModel(width, height int) MessagesModel {
	vp := viewport.New(width, height-3)
	vp.Style = viewportStyle

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	return MessagesModel{
		viewport:    vp,
		messages:    []ChatMessage{},
		spinner:     s,
		showSpinner: false,
		width:       width,
		height:      height,
	}
}

// Init implements tea.Model.
func (m MessagesModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m MessagesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3

	case spinner.TickMsg:
		if m.showSpinner {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	m.viewport, _ = m.viewport.Update(msg)
	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m MessagesModel) View() string {
	var sb strings.Builder

	// Render messages
	for _, msg := range m.messages {
		sb.WriteString(renderMessage(msg))
		sb.WriteString("\n")
	}

	// Show spinner while waiting
	if m.showSpinner {
		sb.WriteString(m.spinner.View())
	}

	m.viewport.SetContent(sb.String())
	if !m.showSpinner {
		m.viewport.GotoBottom()
	}

	return m.viewport.View()
}

// AppendUserMessage adds a user message.
func (m *MessagesModel) AppendUserMessage(content string) {
	m.messages = append(m.messages, ChatMessage{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	})
	m.showSpinner = true
}

// AppendSystemMessage adds a system message (errors, status, etc).
func (m *MessagesModel) AppendSystemMessage(content string) {
	m.messages = append(m.messages, ChatMessage{
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
	})
}

// AppendAssistantMessage starts a new assistant message (streaming).
func (m *MessagesModel) AppendAssistantMessage(content string) {
	m.messages = append(m.messages, ChatMessage{
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
		Streaming: true,
	})
}

// AppendToken appends to the streaming assistant message.
func (m *MessagesModel) AppendToken(content string) {
	// Find the last streaming message
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" && m.messages[i].Streaming {
			m.messages[i].Content += content
			return
		}
	}
	// No streaming message, create one
	m.AppendAssistantMessage(content)
}

// FinalizeMessage marks the streaming message as complete.
func (m *MessagesModel) FinalizeMessage() {
	// Find the last streaming message and finalize it
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" && m.messages[i].Streaming {
			m.messages[i].Streaming = false
			m.messages[i].Rendered = RenderMarkdown(m.messages[i].Content, m.width-4)
			m.showSpinner = false
			return
		}
	}
}

// SetSize updates the viewport size.
func (m *MessagesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height - 3
}

// ScrollToBottom scrolls to the bottom of the viewport.
func (m *MessagesModel) ScrollToBottom() {
	m.viewport.GotoBottom()
}

// LoadHistory populates the message list from history.
func (m *MessagesModel) LoadHistory(events []gateway.HistoryMessage) {
	m.messages = make([]ChatMessage, len(events))
	for i, ev := range events {
		m.messages[i] = ChatMessage{
			Role:      ev.Role,
			Content:   ev.Content,
			Timestamp: time.Unix(ev.Timestamp, 0),
			Streaming: false,
		}
		// Pre-render markdown
		m.messages[i].Rendered = RenderMarkdown(ev.Content, m.width-4)
	}
}

// Messages returns the current messages.
func (m MessagesModel) Messages() []ChatMessage {
	return m.messages
}

// ClearMessages resets the message list, clears viewport content, and resets
// streaming state. The backing slice is reused to avoid allocation.
func (m *MessagesModel) ClearMessages() {
	m.messages = m.messages[:0]
	m.viewport.SetContent("")
	m.viewport.GotoTop()
	m.showSpinner = false
	m.currentStreamID = ""
}

// renderMessage styles a single chat message using role prefixes and timestamps.
func renderMessage(msg ChatMessage) string {
	ts := msg.Timestamp.Format("15:04")

	switch msg.Role {
	case "user":
		header := userRoleStyle.Render("▶ you") +
			"  " +
			timestampStyle.Render(ts)
		body := userMessageStyle.Render(msg.Content)
		return header + "\n" + body

	case "assistant":
		header := assistantRoleStyle.Render("◀ assistant") +
			"  " +
			timestampStyle.Render(ts)
		content := msg.Content
		if !msg.Streaming {
			if msg.Rendered != "" {
				content = msg.Rendered
			} else {
				content = RenderMarkdown(content, 80)
			}
		}
		body := assistantMessageStyle.Render(content)
		return header + "\n" + body

	case "system":
		divider := systemDividerStyle.Render("─── " + msg.Content + " ───")
		return divider

	case "tool":
		header := toolRoleStyle.Render("⚙ tool") +
			"  " +
			timestampStyle.Render(ts)
		body := toolMessageStyle.Render(msg.Content)
		return header + "\n" + body

	default:
		return defaultMessageStyle.Render(msg.Content)
	}
}

// Ensure lipgloss is used (styles are defined in styles.go).
var _ = lipgloss.NewStyle

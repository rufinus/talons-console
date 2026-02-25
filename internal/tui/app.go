package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rufinus/talons-console/internal/config"
	"github.com/rufinus/talons-console/internal/gateway"
)

// Model is the main Bubble Tea model that coordinates all UI components.
type Model struct {
	// Dependencies (injected)
	client gateway.GatewayClient
	config *config.Config

	// Sub-models
	header   HeaderModel
	messages MessagesModel
	input    InputModel

	// State
	width, height int
	quitting      bool
}

// NewModel creates a new TUI Model with all sub-components initialized.
func NewModel(client gateway.GatewayClient, cfg *config.Config) Model {
	return Model{
		client:   client,
		config:   cfg,
		header:   NewHeaderModel(cfg.Agent, cfg.Session),
		messages: NewMessagesModel(80, 24),
		input:    NewInputModel(80, 3),
	}
}

// Init implements tea.Model. Returns the initial command to start the Gateway listener.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.listenCmd(),
		m.messages.Init(),
	)
}

// listenCmd creates a Bubble Tea command that listens for Gateway events.
func (m Model) listenCmd() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-m.client.Messages()
		if !ok {
			return QuitMsg{}
		}
		return GatewayEventMsg{Event: event}
	}
}

// Update implements tea.Model. Routes messages to appropriate handlers.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Let input handle the key first
		inputModel, inputCmd := m.input.Update(msg)
		m.input = inputModel.(InputModel)
		cmds = append(cmds, inputCmd)

		// Handle special keys after input processing
		switch msg.Type {
		case tea.KeyEnter:
			// Send message if input has content
			if value := m.input.Value(); value != "" {
				cmds = append(cmds, m.handleSend(value))
				m.input.Reset()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()

	case GatewayEventMsg:
		cmd := m.handleGatewayEvent(msg.Event)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Continue listening
		cmds = append(cmds, m.listenCmd())

	case ConnectionStateMsg:
		m.header.SetConnectionState(msg.State)

	case SendRequestMsg:
		cmds = append(cmds, m.handleSend(msg.Text))

	case QuitMsg:
		m.quitting = true
		return m, tea.Quit
	}

	// Update sub-models
	msgModel, msgCmd := m.messages.Update(msg)
	m.messages = msgModel.(MessagesModel)
	cmds = append(cmds, msgCmd)

	return m, tea.Batch(cmds...)
}

// handleSend sends a message to the Gateway and updates the UI.
func (m *Model) handleSend(text string) tea.Cmd {
	return func() tea.Msg {
		// Build the chat.send params payload
		params := gateway.ChatSendParams{
			Content:    text,
			SessionKey: m.config.Session,
			AgentID:    m.config.Agent,
			Deliver:    m.config.Deliver,
			Thinking:   m.config.Thinking,
			TimeoutMs:  m.config.TimeoutMs,
		}

		msg := gateway.OutboundMessage{
			Type:    "chat.send",
			Payload: params,
		}

		if err := m.client.Send(msg); err != nil {
			return GatewayEventMsg{Event: gateway.InboundEvent{
				Kind:  gateway.KindError,
				Error: fmt.Sprintf("Failed to send: %v", err),
			}}
		}

		return nil
	}
}

// handleGatewayEvent processes events from the Gateway.
func (m *Model) handleGatewayEvent(event gateway.InboundEvent) tea.Cmd {
	switch event.Kind {
	case gateway.KindToken:
		m.messages.AppendToken(event.Content)

	case gateway.KindMessage:
		m.messages.FinalizeMessage()

	case gateway.KindAuthResult:
		if event.Success {
			m.header.SetConnectionState(gateway.StateConnected)
		}

	case gateway.KindError:
		// Display error in messages
		m.messages.AppendUserMessage(fmt.Sprintf("Error: %s", event.Error))

	case gateway.KindHistory:
		m.messages.LoadHistory(event.HistoryMessages)

	case gateway.KindSessionInfo:
		// Update header with session info
		m.header.agent = event.Agent
		m.header.session = event.Session
	}

	return nil
}

// updateSizes adjusts all components to fit the terminal.
func (m *Model) updateSizes() {
	// Header: 1 line
	// Input: 3 lines minimum
	// Messages: remaining space
	headerHeight := 1
	inputHeight := 3
	messagesHeight := m.height - headerHeight - inputHeight - 2 // padding/borders

	if messagesHeight < 10 {
		messagesHeight = 10 // minimum
	}

	m.header.SetSize(m.width)
	m.messages.SetSize(m.width, messagesHeight)
	m.input.SetSize(m.width, inputHeight)
}

// View implements tea.Model. Renders the complete UI.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	// Layout: Header at top, Messages in middle, Input at bottom
	headerView := m.header.View()
	messagesView := m.messages.View()
	inputView := m.input.View()

	// Use lipgloss to join vertically
	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerView,
		messagesView,
		inputView,
	)
}

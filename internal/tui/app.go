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
		width:    80,
		height:   24,
	}
}

// ListenCmd creates a Bubble Tea command that blocks on the given event channel
// and returns the next event (or a ConnectionStateMsg if the channel is closed).
// This is the recommended pattern — pass the channel directly to avoid capturing
// the model value inside a closure.
func ListenCmd(ch <-chan gateway.InboundEvent) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return ConnectionStateMsg{State: gateway.StateDisconnected}
		}
		return GatewayEventMsg{Event: evt}
	}
}

// Init implements tea.Model. Returns the initial command to start the Gateway listener.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		ListenCmd(m.client.Messages()),
		m.messages.Init(),
	)
}

// Update implements tea.Model. Routes messages to appropriate handlers.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle Ctrl-C first
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}

		// Let input handle the key first
		inputModel, inputCmd := m.input.Update(msg)
		m.input = inputModel.(InputModel)
		cmds = append(cmds, inputCmd)

		// Handle Enter: add user message immediately, then dispatch send
		if msg.Type == tea.KeyEnter {
			if value := m.input.Value(); value != "" {
				m.messages.AppendUserMessage(value)
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
		// Re-arm the listener for the next event
		cmds = append(cmds, ListenCmd(m.client.Messages()))

	case ConnectionStateMsg:
		m.header.SetConnectionState(msg.State)

	case SendRequestMsg:
		m.messages.AppendUserMessage(msg.Text)
		cmds = append(cmds, m.handleSend(msg.Text))

	case QuitMsg:
		m.quitting = true
		return m, tea.Quit
	}

	// Give sub-models a chance to handle the message too
	msgModel, msgCmd := m.messages.Update(msg)
	m.messages = msgModel.(MessagesModel)
	cmds = append(cmds, msgCmd)

	return m, tea.Batch(cmds...)
}

// handleSend sends a message to the Gateway.
func (m *Model) handleSend(text string) tea.Cmd {
	client := m.client
	cfg := m.config

	return func() tea.Msg {
		params := gateway.ChatSendParams{
			Content:    text,
			SessionKey: cfg.Session,
			AgentID:    cfg.Agent,
			Deliver:    cfg.Deliver,
			Thinking:   cfg.Thinking,
			TimeoutMs:  cfg.TimeoutMs,
		}

		outMsg := gateway.OutboundMessage{
			Type:    "chat.send",
			Payload: params,
		}

		if err := client.Send(outMsg); err != nil {
			return GatewayEventMsg{Event: gateway.InboundEvent{
				Kind:  gateway.KindError,
				Error: fmt.Sprintf("Failed to send: %v", err),
			}}
		}

		return nil
	}
}

// handleGatewayEvent processes events from the Gateway and updates sub-models.
func (m *Model) handleGatewayEvent(event gateway.InboundEvent) tea.Cmd {
	switch event.Kind {
	case gateway.KindToken:
		m.messages.AppendToken(event.Content)

	case gateway.KindMessage:
		m.messages.FinalizeMessage()

	case gateway.KindAuthResult:
		if event.Success {
			m.header.SetConnectionState(gateway.StateConnected)
		} else {
			m.header.SetConnectionState(gateway.StateDisconnected)
		}

	case gateway.KindError:
		m.messages.AppendSystemMessage("Error: " + event.Error)

	case gateway.KindHistory:
		m.messages.LoadHistory(event.HistoryMessages)

	case gateway.KindSessionInfo:
		m.header.agent = event.Agent
		m.header.session = event.Session
	}

	return nil
}

// updateSizes adjusts all components to fit the terminal.
func (m *Model) updateSizes() {
	// Header: 1 line, Input: 3 lines, Messages: remaining space
	headerHeight := 1
	inputHeight := 3
	messagesHeight := m.height - headerHeight - inputHeight - 2 // borders/padding

	if messagesHeight < 10 {
		messagesHeight = 10
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

	headerView := m.header.View()
	messagesView := m.messages.View()
	inputView := m.input.View()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerView,
		messagesView,
		inputView,
	)
}

// RecoverTerminal runs the Bubble Tea program with panic recovery,
// restoring the terminal on crash.
func RecoverTerminal(p *tea.Program) error {
	defer func() {
		if r := recover(); r != nil {
			_ = p.RestoreTerminal()
			fmt.Printf("talons crashed: %v\n", r)
		}
	}()
	_, err := p.Run()
	return err
}

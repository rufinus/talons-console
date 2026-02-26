package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/rufinus/talons-console/internal/commands"
	"github.com/rufinus/talons-console/internal/config"
	"github.com/rufinus/talons-console/internal/gateway"
)

// tabCompleteState holds state for a single tab-completion cycling session.
type tabCompleteState struct {
	prefix  string   // the original input when tab was first pressed
	matches []string // the "/name" matches returned by registry.Complete
	index   int      // which match is currently shown
}

// Model is the main Bubble Tea model that coordinates all UI components.
type Model struct {
	// Dependencies (injected)
	client gateway.GatewayClient
	config *config.Config

	// Sub-models
	header   HeaderModel
	messages MessagesModel
	input    InputModel

	// Commands
	registry *commands.CommandRegistry
	history  *commands.History
	tabState *tabCompleteState

	// Session state
	state *SessionState

	// Streaming guard: deferred /clear while a stream is in progress.
	pendingClear bool

	// First-launch hint guard — fired exactly once per session.
	hintShown bool

	// Layout
	width, height int
	quitting      bool
}

// Compile-time assertion: Model must satisfy commands.HandlerContext.
var _ commands.HandlerContext = (*Model)(nil)

// NewModel creates a new TUI Model with all sub-components initialized.
func NewModel(client gateway.GatewayClient, cfg *config.Config) Model {
	commands.InitCommands()
	st := NewSessionState(cfg)
	return Model{
		client:   client,
		config:   cfg,
		header:   NewHeaderModel(cfg.Agent, cfg.Session),
		messages: NewMessagesModel(80, 24),
		input:    NewInputModel(80, 3),
		registry: commands.DefaultRegistry,
		history:  commands.NewHistory(),
		state:    st,
		width:    80,
		height:   24,
	}
}

// ─────────────────────────────────────────────
// HandlerContext implementation
// ─────────────────────────────────────────────

func (m *Model) AppendSystemMessage(content string) { m.messages.AppendSystemMessage(content) }

func (m *Model) ClearMessages() {
	// If a stream is in progress, defer the clear until it completes.
	if m.messages.showSpinner {
		m.pendingClear = true
		return
	}
	m.messages.ClearMessages()
}

func (m *Model) GetAgent() string         { return m.state.GetAgent() }
func (m *Model) GetSession() string       { return m.state.GetSession() }
func (m *Model) GetModel() string         { return m.state.GetModel() }
func (m *Model) GetThinking() string      { return m.state.GetThinking() }
func (m *Model) GetTimeoutMs() int        { return m.state.GetTimeoutMs() }
func (m *Model) GetGatewayURL() string    { return m.state.GetGatewayURL() }
func (m *Model) GetVersion() string       { return m.state.GetVersion() }
func (m *Model) GetUptime() time.Duration { return m.state.Uptime() }
func (m *Model) GetMsgSent() int          { return m.state.GetMsgSent() }
func (m *Model) GetMsgRecv() int          { return m.state.GetMsgRecv() }

func (m *Model) SetAgent(name string)     { m.state.SetAgent(name) }
func (m *Model) SetSession(key string)    { m.state.SetSession(key) }
func (m *Model) SetModel(id string)       { m.state.SetModel(id) }
func (m *Model) SetThinking(level string) { m.state.SetThinking(level) }
func (m *Model) SetTimeoutMs(ms int)      { m.state.SetTimeoutMs(ms) }

func (m *Model) RequestHistory(sessionKey string) error { return m.client.RequestHistory(sessionKey) }
func (m *Model) Reconnect(ctx context.Context) error    { return m.client.Reconnect(ctx) }
func (m *Model) CloseGateway() error                    { return m.client.Close() }

func (m *Model) IsConnected() bool {
	return m.client.State() == gateway.StateConnected
}

func (m *Model) UpdateHeader() {
	m.header.SetAgent(m.state.GetAgent())
	m.header.SetSession(m.state.GetSession())
}

func (m *Model) GetWidth() int { return m.width }

// ─────────────────────────────────────────────
// ListenCmd / listenForMessages
// ─────────────────────────────────────────────

// ListenCmd creates a Bubble Tea command that blocks on the given event channel
// and returns the next event (or a ConnectionStateMsg if the channel is closed).
func ListenCmd(ch <-chan gateway.InboundEvent) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return ConnectionStateMsg{State: gateway.StateDisconnected}
		}
		return GatewayEventMsg{Event: evt}
	}
}

// listenForMessages returns a tea.Cmd to listen for the next gateway event.
func (m *Model) listenForMessages() tea.Cmd {
	return ListenCmd(m.client.Messages())
}

// ─────────────────────────────────────────────
// Init / Update / View
// ─────────────────────────────────────────────

// Init implements tea.Model. Returns the initial command to start the Gateway listener.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.listenForMessages(),
		m.messages.Init(),
		func() tea.Msg { return firstLaunchHintMsg{} },
	)
}

// Update implements tea.Model. Routes messages to appropriate handlers.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if cmd, done := m.handleKeyMsg(msg); done {
			return m, cmd
		} else if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
	case GatewayEventMsg:
		if cmd := m.handleGatewayEvent(msg.Event); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.listenForMessages())
	case ConnectionStateMsg:
		m.header.SetConnectionState(msg.State)
	case ReconnectedMsg:
		cmds = append(cmds, m.handleReconnectedMsg(msg)...)
	case SystemErrorMsg:
		m.messages.AppendSystemMessage("Error: " + msg.Err.Error())
	case SendRequestMsg:
		m.messages.AppendUserMessage(msg.Text)
		cmds = append(cmds, m.handleSend(msg.Text))
	case QuitMsg:
		m.quitting = true
		return m, tea.Quit
	case firstLaunchHintMsg:
		if !m.hintShown {
			m.hintShown = true
			m.messages.AppendSystemMessage("💡 Type /help for available commands")
		}
	}

	msgModel, msgCmd := m.messages.Update(msg)
	m.messages = msgModel.(MessagesModel)
	cmds = append(cmds, msgCmd)

	return m, tea.Batch(cmds...)
}

// handleKeyMsg routes keyboard events. Returns (cmd, true) when the caller should
// return immediately (e.g. quit), or (cmd, false) to continue normal processing.
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Cmd, bool) {
	// Always reset tab completion on any non-Tab key press.
	if msg.Type != tea.KeyTab {
		m.tabState = nil
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return tea.Quit, true

	case tea.KeyTab:
		_ = m.handleTab()
		return nil, true

	case tea.KeyUp:
		if !strings.Contains(m.input.Value(), "\n") {
			if entry, ok := m.history.Prev(m.input.Value()); ok {
				m.input.SetValue(entry)
			}
			return nil, true
		}

	case tea.KeyDown:
		if !strings.Contains(m.input.Value(), "\n") {
			if entry, ok := m.history.Next(); ok {
				m.input.SetValue(entry)
			}
			return nil, true
		}
	}

	// Let input handle all other keys.
	inputModel, inputCmd := m.input.Update(msg)
	m.input = inputModel.(InputModel)

	if msg.Type == tea.KeyEnter {
		enterCmd := m.handleEnter()
		return tea.Batch(inputCmd, enterCmd), false
	}

	return inputCmd, false
}

// handleReconnectedMsg handles a successful reconnect event.
func (m *Model) handleReconnectedMsg(msg ReconnectedMsg) []tea.Cmd {
	m.state.ResetCounters()
	m.state.SetConnected(m.state.GetGatewayURL(), msg.At)
	m.messages.ClearMessages()
	m.UpdateHeader()
	m.messages.AppendSystemMessage("Reconnected to gateway")
	return []tea.Cmd{m.listenForMessages()}
}

// handleEnter processes the Enter key: dispatches commands or sends chat messages.
func (m *Model) handleEnter() tea.Cmd {
	value := m.input.Value()
	if strings.TrimSpace(value) == "" {
		return nil
	}

	// Always reset tab state and add to history.
	m.tabState = nil
	m.history.Add(value)

	def, args, isCmd := m.registry.Parse(value)

	if isCmd {
		// Reset input before handler execution (handler may append messages synchronously).
		m.input.Reset()
		if def == nil {
			m.messages.AppendSystemMessage("⚠ Unknown command. Type /help for available commands.")
			return nil
		}
		return def.Handler(m, args)
	}

	// Regular chat message.
	m.messages.AppendUserMessage(value)
	cmd := m.handleSend(value)
	m.input.Reset()
	return cmd
}

// handleTab processes Tab key for command completion.
func (m *Model) handleTab() tea.Cmd {
	value := m.input.Value()
	trimmed := strings.TrimSpace(value)

	// Only activate for slash-prefixed input without a space (i.e., still typing command name).
	if !strings.HasPrefix(trimmed, "/") {
		return nil
	}
	if strings.Contains(trimmed, " ") {
		return nil
	}

	// Strip the leading slash to get the prefix for Complete.
	prefix := strings.TrimPrefix(trimmed, "/")

	if m.tabState == nil || m.tabState.prefix != value {
		// Start a new completion cycle.
		matches := m.registry.Complete(prefix)
		if len(matches) == 0 {
			return nil
		}
		m.tabState = &tabCompleteState{
			prefix:  value,
			matches: matches,
			index:   0,
		}
	} else {
		// Advance to the next match.
		m.tabState.index = (m.tabState.index + 1) % len(m.tabState.matches)
	}

	completed := m.tabState.matches[m.tabState.index] + " "
	m.input.SetValue(completed)
	// Update the stored prefix to the completed value (minus trailing space) so
	// subsequent Tab presses are detected as "same prefix".
	m.tabState.prefix = strings.TrimSuffix(completed, " ")
	return nil
}

// handleSend sends a message to the Gateway.
func (m *Model) handleSend(content string) tea.Cmd {
	client := m.client

	params := gateway.ChatSendParams{
		Message:        content,
		IdempotencyKey: uuid.NewString(),
	}
	m.state.ApplyToSendParams(&params)
	m.state.IncrSent()

	return func() tea.Msg {
		outMsg := gateway.OutboundMessage{
			Type:    "chat.send",
			Payload: params,
		}

		if err := client.Send(outMsg); err != nil {
			return SystemErrorMsg{Err: fmt.Errorf("failed to send: %w", err)}
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
		m.state.IncrRecv()
		m.messages.FinalizeMessage()
		if m.pendingClear {
			m.messages.ClearMessages()
			m.pendingClear = false
		}

	case gateway.KindAuthResult:
		if event.Success {
			m.header.SetConnectionState(gateway.StateConnected)
		} else {
			m.header.SetConnectionState(gateway.StateDisconnected)
		}

	case gateway.KindError:
		if m.pendingClear {
			m.messages.ClearMessages()
			m.pendingClear = false
		}
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
	headerHeight := 1
	inputHeight := 3
	messagesHeight := m.height - headerHeight - inputHeight - 2

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

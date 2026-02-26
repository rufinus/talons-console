package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rufinus/talons-console/internal/config"
	"github.com/rufinus/talons-console/internal/gateway"
)

// ─────────────────────────────────────────────
// Mock GatewayClient
// ─────────────────────────────────────────────

type mockClient struct {
	ch      chan gateway.InboundEvent
	sent    []gateway.OutboundMessage
	sendErr error
	state   gateway.ConnectionState
}

func newMockClient() *mockClient {
	return &mockClient{
		ch:    make(chan gateway.InboundEvent, 16),
		state: gateway.StateConnected,
	}
}

func (c *mockClient) Connect(_ context.Context) error       { return nil }
func (c *mockClient) Close() error                          { return nil }
func (c *mockClient) State() gateway.ConnectionState        { return c.state }
func (c *mockClient) Messages() <-chan gateway.InboundEvent { return c.ch }
func (c *mockClient) Send(msg gateway.OutboundMessage) error {
	c.sent = append(c.sent, msg)
	return c.sendErr
}
func (c *mockClient) RequestHistory(_ string) error     { return nil }
func (c *mockClient) Reconnect(_ context.Context) error { return nil }

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func newTestModel() (Model, *mockClient) {
	client := newMockClient()
	cfg := &config.Config{
		Agent:   "test-agent",
		Session: "test-session",
	}
	m := NewModel(client, cfg)
	return m, client
}

// runUpdate is a helper that casts the returned tea.Model to Model.
func runUpdate(m Model, msg tea.Msg) (Model, tea.Cmd) {
	newModel, cmd := m.Update(msg)
	return newModel.(Model), cmd
}

// ─────────────────────────────────────────────
// NewModel
// ─────────────────────────────────────────────

func TestNewModel(t *testing.T) {
	m, client := newTestModel()

	if m.client != client {
		t.Error("expected client to be set")
	}
	if m.config.Agent != "test-agent" {
		t.Errorf("expected agent %q, got %q", "test-agent", m.config.Agent)
	}
	if m.width != 80 {
		t.Errorf("expected default width 80, got %d", m.width)
	}
	if m.height != 24 {
		t.Errorf("expected default height 24, got %d", m.height)
	}
}

// ─────────────────────────────────────────────
// Update routing
// ─────────────────────────────────────────────

func TestUpdate_CtrlC_Quits(t *testing.T) {
	m, _ := newTestModel()
	newM, cmd := runUpdate(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if !newM.quitting {
		t.Error("expected quitting=true after Ctrl-C")
	}
	if cmd == nil {
		t.Error("expected non-nil quit cmd")
	}
}

func TestUpdate_WindowSize(t *testing.T) {
	m, _ := newTestModel()
	newM, _ := runUpdate(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if newM.width != 120 {
		t.Errorf("expected width 120, got %d", newM.width)
	}
	if newM.height != 40 {
		t.Errorf("expected height 40, got %d", newM.height)
	}
}

func TestUpdate_QuitMsg(t *testing.T) {
	m, _ := newTestModel()
	newM, cmd := runUpdate(m, QuitMsg{})
	if !newM.quitting {
		t.Error("expected quitting=true after QuitMsg")
	}
	if cmd == nil {
		t.Error("expected non-nil quit cmd")
	}
}

func TestUpdate_ConnectionStateMsg(t *testing.T) {
	m, _ := newTestModel()
	newM, _ := runUpdate(m, ConnectionStateMsg{State: gateway.StateConnected})
	if newM.header.connectionState != gateway.StateConnected {
		t.Errorf("expected header state Connected, got %v", newM.header.connectionState)
	}
}

func TestUpdate_GatewayEventMsg_ReArmsListener(t *testing.T) {
	m, _ := newTestModel()
	evt := gateway.InboundEvent{Kind: gateway.KindToken, Content: "hello"}
	newM, cmd := runUpdate(m, GatewayEventMsg{Event: evt})

	// Token should be appended
	msgs := newM.messages.Messages()
	if len(msgs) == 0 {
		t.Fatal("expected at least one message after token event")
	}
	found := false
	for _, msg := range msgs {
		if strings.Contains(msg.Content, "hello") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'hello' token in messages")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (listener re-arm)")
	}
}

func TestUpdate_SendRequestMsg_AddsUserMessage(t *testing.T) {
	m, _ := newTestModel()
	newM, _ := runUpdate(m, SendRequestMsg{Text: "hello world"})

	msgs := newM.messages.Messages()
	if len(msgs) == 0 {
		t.Fatal("expected user message after SendRequestMsg")
	}
	last := msgs[len(msgs)-1]
	if last.Role != "user" {
		t.Errorf("expected role=user, got %q", last.Role)
	}
	if last.Content != "hello world" {
		t.Errorf("expected content 'hello world', got %q", last.Content)
	}
}

func TestUpdate_EnterKey_AddsUserMessage(t *testing.T) {
	m, _ := newTestModel()
	// Simulate typing via direct field injection (bypassing real textarea input events)
	// We test the logic by checking the state after handleSend is invoked.
	// Since textarea update requires real keystrokes, test SendRequestMsg instead.
	// (EnterKey flow is covered by integration; unit test uses SendRequestMsg)
	newM, _ := runUpdate(m, SendRequestMsg{Text: "typed message"})
	msgs := newM.messages.Messages()
	if len(msgs) == 0 || msgs[len(msgs)-1].Content != "typed message" {
		t.Error("expected 'typed message' in messages")
	}
}

// ─────────────────────────────────────────────
// handleGatewayEvent
// ─────────────────────────────────────────────

func TestHandleGatewayEvent_Token(t *testing.T) {
	m, _ := newTestModel()
	m.handleGatewayEvent(gateway.InboundEvent{Kind: gateway.KindToken, Content: "partial"})

	msgs := m.messages.Messages()
	if len(msgs) == 0 {
		t.Fatal("expected message after KindToken")
	}
	last := msgs[len(msgs)-1]
	if last.Role != "assistant" {
		t.Errorf("expected role assistant, got %q", last.Role)
	}
	if !strings.Contains(last.Content, "partial") {
		t.Errorf("expected content to contain 'partial', got %q", last.Content)
	}
}

func TestHandleGatewayEvent_Message_Finalizes(t *testing.T) {
	m, _ := newTestModel()
	// Seed a streaming message
	m.messages.AppendToken("full response")
	m.handleGatewayEvent(gateway.InboundEvent{Kind: gateway.KindMessage})

	msgs := m.messages.Messages()
	if len(msgs) == 0 {
		t.Fatal("expected at least one message")
	}
	last := msgs[len(msgs)-1]
	if last.Streaming {
		t.Error("expected Streaming=false after KindMessage finalizes")
	}
}

func TestHandleGatewayEvent_Error(t *testing.T) {
	m, _ := newTestModel()
	m.handleGatewayEvent(gateway.InboundEvent{Kind: gateway.KindError, Error: "something went wrong"})

	msgs := m.messages.Messages()
	if len(msgs) == 0 {
		t.Fatal("expected error message in list")
	}
	last := msgs[len(msgs)-1]
	if last.Role != "system" {
		t.Errorf("expected role=system for error, got %q", last.Role)
	}
	if !strings.Contains(last.Content, "something went wrong") {
		t.Errorf("expected error text in content, got %q", last.Content)
	}
}

func TestHandleGatewayEvent_AuthResult_Connected(t *testing.T) {
	m, _ := newTestModel()
	m.handleGatewayEvent(gateway.InboundEvent{Kind: gateway.KindAuthResult, Success: true})
	if m.header.connectionState != gateway.StateConnected {
		t.Errorf("expected StateConnected after successful auth, got %v", m.header.connectionState)
	}
}

func TestHandleGatewayEvent_AuthResult_Disconnected(t *testing.T) {
	m, _ := newTestModel()
	m.header.SetConnectionState(gateway.StateConnecting)
	m.handleGatewayEvent(gateway.InboundEvent{Kind: gateway.KindAuthResult, Success: false})
	if m.header.connectionState != gateway.StateDisconnected {
		t.Errorf("expected StateDisconnected after failed auth, got %v", m.header.connectionState)
	}
}

func TestHandleGatewayEvent_History(t *testing.T) {
	m, _ := newTestModel()
	history := []gateway.HistoryMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	m.handleGatewayEvent(gateway.InboundEvent{Kind: gateway.KindHistory, HistoryMessages: history})

	msgs := m.messages.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 history messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Errorf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi there" {
		t.Errorf("unexpected second message: %+v", msgs[1])
	}
}

func TestHandleGatewayEvent_SessionInfo(t *testing.T) {
	m, _ := newTestModel()
	m.handleGatewayEvent(gateway.InboundEvent{
		Kind:    gateway.KindSessionInfo,
		Agent:   "my-agent",
		Session: "my-session",
	})
	if m.header.agent != "my-agent" {
		t.Errorf("expected header.agent=my-agent, got %q", m.header.agent)
	}
	if m.header.session != "my-session" {
		t.Errorf("expected header.session=my-session, got %q", m.header.session)
	}
}

// ─────────────────────────────────────────────
// handleSend
// ─────────────────────────────────────────────

func TestHandleSend_SendsOutboundMessage(t *testing.T) {
	m, client := newTestModel()
	cmd := m.handleSend("test message")
	// Execute the command synchronously
	result := cmd()

	if result != nil {
		t.Errorf("expected nil result on success, got %T %v", result, result)
	}
	if len(client.sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(client.sent))
	}
	if client.sent[0].Type != "chat.send" {
		t.Errorf("expected type 'chat.send', got %q", client.sent[0].Type)
	}
	params, ok := client.sent[0].Payload.(gateway.ChatSendParams)
	if !ok {
		t.Fatalf("expected ChatSendParams payload, got %T", client.sent[0].Payload)
	}
	if params.Content != "test message" {
		t.Errorf("expected content 'test message', got %q", params.Content)
	}
	if params.AgentID != "test-agent" {
		t.Errorf("expected agentId 'test-agent', got %q", params.AgentID)
	}
	if params.SessionKey != "test-session" {
		t.Errorf("expected sessionKey 'test-session', got %q", params.SessionKey)
	}
}

func TestHandleSend_ReturnsErrorEvent_OnSendFailure(t *testing.T) {
	m, client := newTestModel()
	client.sendErr = &mockSendError{"gateway unavailable"}

	cmd := m.handleSend("fail message")
	result := cmd()

	evtMsg, ok := result.(GatewayEventMsg)
	if !ok {
		t.Fatalf("expected GatewayEventMsg on send error, got %T", result)
	}
	if evtMsg.Event.Kind != gateway.KindError {
		t.Errorf("expected KindError, got %v", evtMsg.Event.Kind)
	}
	if !strings.Contains(evtMsg.Event.Error, "gateway unavailable") {
		t.Errorf("expected error message to contain 'gateway unavailable', got %q", evtMsg.Event.Error)
	}
}

// mockSendError implements error for send failure tests.
type mockSendError struct{ msg string }

func (e *mockSendError) Error() string { return e.msg }

// ─────────────────────────────────────────────
// ListenCmd
// ─────────────────────────────────────────────

func TestListenCmd_ReturnsGatewayEventMsg(t *testing.T) {
	ch := make(chan gateway.InboundEvent, 1)
	ch <- gateway.InboundEvent{Kind: gateway.KindToken, Content: "tok"}

	cmd := ListenCmd(ch)
	result := cmd()

	msg, ok := result.(GatewayEventMsg)
	if !ok {
		t.Fatalf("expected GatewayEventMsg, got %T", result)
	}
	if msg.Event.Kind != gateway.KindToken {
		t.Errorf("expected KindToken, got %v", msg.Event.Kind)
	}
	if msg.Event.Content != "tok" {
		t.Errorf("expected content 'tok', got %q", msg.Event.Content)
	}
}

func TestListenCmd_ReturnsConnectionStateMsg_OnClose(t *testing.T) {
	ch := make(chan gateway.InboundEvent)
	close(ch)

	cmd := ListenCmd(ch)
	result := cmd()

	stateMsg, ok := result.(ConnectionStateMsg)
	if !ok {
		t.Fatalf("expected ConnectionStateMsg on closed channel, got %T", result)
	}
	if stateMsg.State != gateway.StateDisconnected {
		t.Errorf("expected StateDisconnected, got %v", stateMsg.State)
	}
}

// ─────────────────────────────────────────────
// View
// ─────────────────────────────────────────────

func TestView_ContainsComponents(t *testing.T) {
	m, _ := newTestModel()
	// The View should produce non-empty output that joins header + messages + input
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
	// Header contains the agent/session
	if !strings.Contains(view, "test-agent") {
		t.Errorf("expected view to contain agent name, got:\n%s", view)
	}
}

func TestView_EmptyWhenQuitting(t *testing.T) {
	m, _ := newTestModel()
	m.quitting = true
	view := m.View()
	if view != "" {
		t.Errorf("expected empty view when quitting, got %q", view)
	}
}

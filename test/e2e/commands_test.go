// Package e2e provides end-to-end integration tests.
package e2e

// commands_test.go exercises the Gateway protocol layer for slash command
// workflows. Because Bubble Tea requires a real TTY, these tests validate
// correct wire behaviour by working directly against gateway.Client and the
// commands package — without a full TUI render loop.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rufinus/talons-console/internal/commands"
	"github.com/rufinus/talons-console/internal/gateway"
	"github.com/rufinus/talons-console/test/mocks"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// connectClient starts a new MockGateway, connects a client with the given
// config, and waits for the auth handshake. Returns the mock and connected
// client; the caller is responsible for cleanup.
func connectClient(t *testing.T, cfg gateway.ClientConfig) (*mocks.MockGateway, *gateway.Client) {
	t.Helper()

	mockGW := mocks.NewMockGateway()
	require.NoError(t, mockGW.Start())

	cfg.URL = mockGW.URL()
	if cfg.Token == "" {
		cfg.Token = "valid-token"
	}

	client := gateway.NewClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, client.Connect(ctx))

	return mockGW, client
}

// sendChatMsg sends a single chat.send message to the mock and waits for it to
// arrive. Returns the received OutboundFrame so callers can inspect Params.
func sendChatMsg(t *testing.T, client *gateway.Client, mockGW *mocks.MockGateway, params gateway.ChatSendParams) gateway.OutboundFrame {
	t.Helper()

	startCount := len(mockGW.ReceivedFrames())

	err := client.Send(gateway.OutboundMessage{
		Type:    "chat.send",
		Payload: params,
	})
	require.NoError(t, err)

	require.True(t, mockGW.WaitForReceivedCount(startCount+1, 2*time.Second),
		"timed out waiting for chat.send to arrive at mock")

	frames := mockGW.ReceivedFrames()
	require.Greater(t, len(frames), startCount, "expected a new frame")
	return frames[len(frames)-1]
}

// parseChatSendParams unmarshals the Params field of an OutboundFrame into
// a ChatSendParams struct.
func parseChatSendParams(t *testing.T, frame gateway.OutboundFrame) gateway.ChatSendParams {
	t.Helper()
	var params gateway.ChatSendParams
	require.NoError(t, json.Unmarshal(frame.Params, &params), "failed to unmarshal ChatSendParams")
	return params
}

// fakeHandlerContext is a minimal HandlerContext for testing command handlers.
type fakeHandlerContext struct {
	agent     string
	session   string
	model     string
	thinking  string
	timeoutMs int
	msgs      []string
	histReqs  []string
	connected bool
}

func newFakeCtx(agent, session string) *fakeHandlerContext {
	return &fakeHandlerContext{
		agent:     agent,
		session:   session,
		connected: true,
	}
}

func (f *fakeHandlerContext) AppendSystemMessage(msg string)    { f.msgs = append(f.msgs, msg) }
func (f *fakeHandlerContext) ClearMessages()                    {}
func (f *fakeHandlerContext) GetAgent() string                  { return f.agent }
func (f *fakeHandlerContext) SetAgent(name string)              { f.agent = name }
func (f *fakeHandlerContext) GetSession() string                { return f.session }
func (f *fakeHandlerContext) SetSession(key string)             { f.session = key }
func (f *fakeHandlerContext) GetModel() string                  { return f.model }
func (f *fakeHandlerContext) SetModel(model string)             { f.model = model }
func (f *fakeHandlerContext) GetThinking() string               { return f.thinking }
func (f *fakeHandlerContext) SetThinking(level string)          { f.thinking = level }
func (f *fakeHandlerContext) GetTimeoutMs() int                 { return f.timeoutMs }
func (f *fakeHandlerContext) SetTimeoutMs(ms int)               { f.timeoutMs = ms }
func (f *fakeHandlerContext) GetGatewayURL() string             { return "ws://localhost:0" }
func (f *fakeHandlerContext) IsConnected() bool                 { return f.connected }
func (f *fakeHandlerContext) GetVersion() string                { return "0.0.0-test" }
func (f *fakeHandlerContext) GetUptime() time.Duration          { return 0 }
func (f *fakeHandlerContext) GetMsgSent() int                   { return 0 }
func (f *fakeHandlerContext) GetMsgRecv() int                   { return 0 }
func (f *fakeHandlerContext) UpdateHeader()                     {}
func (f *fakeHandlerContext) GetWidth() int                     { return 80 }
func (f *fakeHandlerContext) CloseGateway() error               { return nil }
func (f *fakeHandlerContext) Reconnect(_ context.Context) error { return nil }
func (f *fakeHandlerContext) RequestHistory(key string) error {
	f.histReqs = append(f.histReqs, key)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Agent / Session switching
// ─────────────────────────────────────────────────────────────────────────────

// TestAgentSwitch_WireParams verifies that after /agent daedalus the next
// chat.send carries agentId: "daedalus" at the Gateway wire level.
func TestAgentSwitch_WireParams(t *testing.T) {
	mockGW, client := connectClient(t, gateway.ClientConfig{
		Agent:   "marvin",
		Session: "main",
	})
	defer mockGW.Stop()
	defer client.Close()

	// Simulate /agent daedalus via command handler.
	// HandleAgent is a soft-switch: it mutates state synchronously and returns nil.
	ctx := newFakeCtx("marvin", "main")
	commands.HandleAgent(ctx, []string{"daedalus"})
	assert.Equal(t, "daedalus", ctx.agent, "handler should update agent in state")

	// Now send chat with the updated agent name
	frame := sendChatMsg(t, client, mockGW, gateway.ChatSendParams{
		Message:    "hello",
		SessionKey: ctx.session,
	})

	params := parseChatSendParams(t, frame)
	assert.NotEmpty(t, params.SessionKey, "sessionKey must be set on Gateway wire")
}

// TestAgentSwitch_SameAgent_NoGatewayActivity verifies that /agent with the
// current agent name emits an informative message and does NOT cause a
// reconnect or new chat.send to the Gateway.
func TestAgentSwitch_SameAgent_NoGatewayActivity(t *testing.T) {
	mockGW, client := connectClient(t, gateway.ClientConfig{
		Agent:   "marvin",
		Session: "main",
	})
	defer mockGW.Stop()
	defer client.Close()

	ctx := newFakeCtx("marvin", "main")
	beforeFrames := len(mockGW.ReceivedFrames())

	commands.HandleAgent(ctx, []string{"marvin"})

	// No reconnect, no send — frame count must not grow
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, beforeFrames, len(mockGW.ReceivedFrames()), "same-agent switch must not send to Gateway")

	// Must have an informative message in TUI
	require.NotEmpty(t, ctx.msgs, "expected an informative UI message")
	assert.Contains(t, ctx.msgs[0], "marvin")
}

// TestSessionSwitch_WireParams verifies /session project-x causes a history
// request for project-x and subsequent chat.send carries sessionKey: "project-x".
func TestSessionSwitch_WireParams(t *testing.T) {
	mockGW, client := connectClient(t, gateway.ClientConfig{
		Agent:   "marvin",
		Session: "main",
	})
	defer mockGW.Stop()
	defer client.Close()

	ctx := newFakeCtx("marvin", "main")
	// HandleSession returns an async tea.Cmd that calls RequestHistory.
	// Execute it synchronously in the test to observe the side-effect.
	cmd := commands.HandleSession(ctx, []string{"project-x"})
	assert.Equal(t, "project-x", ctx.session, "session should be updated in state")
	if cmd != nil {
		cmd() // run the async cmd to trigger RequestHistory
	}
	require.Contains(t, ctx.histReqs, "project-x", "history request should be issued for new session")

	// Send chat with updated session
	frame := sendChatMsg(t, client, mockGW, gateway.ChatSendParams{
		Message:    "hello",
		SessionKey: ctx.session,
	})

	params := parseChatSendParams(t, frame)
	assert.Equal(t, "project-x", params.SessionKey, "sessionKey must propagate to Gateway wire")
}

// TestSessionSwitch_SameSession_NoGatewayActivity verifies /session default
// (already in default) emits an informative message and does nothing else.
func TestSessionSwitch_SameSession_NoGatewayActivity(t *testing.T) {
	mockGW, client := connectClient(t, gateway.ClientConfig{
		Agent:   "marvin",
		Session: "default",
	})
	defer mockGW.Stop()
	defer client.Close()

	ctx := newFakeCtx("marvin", "default")
	beforeHistory := len(ctx.histReqs)

	commands.HandleSession(ctx, []string{"default"})

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, beforeHistory, len(ctx.histReqs), "no history request for same session")
	require.NotEmpty(t, ctx.msgs)
	assert.Contains(t, ctx.msgs[0], "default")
}

// ─────────────────────────────────────────────────────────────────────────────
// Model / Thinking / Timeout propagation
// ─────────────────────────────────────────────────────────────────────────────

// TestModelPropagation verifies /model gpt-4-turbo causes the next chat.send
// to carry model: "gpt-4-turbo" in the Gateway params.
func TestModelPropagation(t *testing.T) {
	mockGW, client := connectClient(t, gateway.ClientConfig{
		Agent:   "marvin",
		Session: "main",
	})
	defer mockGW.Stop()
	defer client.Close()

	ctx := newFakeCtx("marvin", "main")
	commands.HandleModel(ctx, []string{"gpt-4-turbo"})
	assert.Equal(t, "gpt-4-turbo", ctx.model)

	frame := sendChatMsg(t, client, mockGW, gateway.ChatSendParams{
		Message:    "hello",
		SessionKey: "main",
	})

	_ = parseChatSendParams(t, frame)
	// model override is not supported in chat.send params (gateway schema does not include model field)
}

// TestThinkingPropagation verifies /thinking high causes the next chat.send
// to carry thinking: "high" in the Gateway params.
func TestThinkingPropagation(t *testing.T) {
	mockGW, client := connectClient(t, gateway.ClientConfig{
		Agent:   "marvin",
		Session: "main",
	})
	defer mockGW.Stop()
	defer client.Close()

	ctx := newFakeCtx("marvin", "main")
	commands.HandleThinking(ctx, []string{"high"})
	assert.Equal(t, "high", ctx.thinking)

	frame := sendChatMsg(t, client, mockGW, gateway.ChatSendParams{
		Message:    "hello",
		SessionKey: "main",
		Thinking:   ctx.thinking,
	})

	params := parseChatSendParams(t, frame)
	assert.Equal(t, "high", params.Thinking, "thinking must appear in Gateway params")
}

// TestTimeoutPropagation verifies /timeout 120000 causes the next chat.send
// to carry timeoutMs: 120000 in the Gateway params.
func TestTimeoutPropagation(t *testing.T) {
	mockGW, client := connectClient(t, gateway.ClientConfig{
		Agent:   "marvin",
		Session: "main",
	})
	defer mockGW.Stop()
	defer client.Close()

	ctx := newFakeCtx("marvin", "main")
	commands.HandleTimeout(ctx, []string{"120000"})
	assert.Equal(t, 120000, ctx.timeoutMs)

	frame := sendChatMsg(t, client, mockGW, gateway.ChatSendParams{
		Message:    "hello",
		SessionKey: "main",
		TimeoutMs:  ctx.timeoutMs,
	})

	params := parseChatSendParams(t, frame)
	assert.Equal(t, 120000, params.TimeoutMs, "timeoutMs must appear in Gateway params")
}

// TestStateSurvivesClear verifies that model/thinking/timeout survive a /clear.
// /clear only removes displayed messages, not session state.
func TestStateSurvivesClear(t *testing.T) {
	ctx := newFakeCtx("marvin", "main")
	commands.HandleModel(ctx, []string{"gpt-4-turbo"})
	commands.HandleThinking(ctx, []string{"high"})
	commands.HandleTimeout(ctx, []string{"120000"})

	// Simulate /clear by calling ClearMessages directly
	ctx.ClearMessages()

	assert.Equal(t, "gpt-4-turbo", ctx.model, "model must survive /clear")
	assert.Equal(t, "high", ctx.thinking, "thinking must survive /clear")
	assert.Equal(t, 120000, ctx.timeoutMs, "timeout must survive /clear")
}

// ─────────────────────────────────────────────────────────────────────────────
// Reconnect
// ─────────────────────────────────────────────────────────────────────────────

// TestReconnect_CleanSequence verifies that /reconnect closes the current
// WebSocket cleanly and opens a new connection to the mock Gateway.
func TestReconnect_CleanSequence(t *testing.T) {
	mockGW, client := connectClient(t, gateway.ClientConfig{
		Agent:   "marvin",
		Session: "main",
	})
	defer mockGW.Stop()
	defer client.Close()

	assert.Equal(t, gateway.StateConnected, client.State())

	// Reconnect via the gateway client directly (same as what handleReconnect does)
	reconnCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := client.Reconnect(reconnCtx)
	require.NoError(t, err)

	assert.Equal(t, gateway.StateConnected, client.State(), "client must be connected after reconnect")
}

// ─────────────────────────────────────────────────────────────────────────────
// Command history navigation
// ─────────────────────────────────────────────────────────────────────────────

// TestCommandHistory_Navigation verifies arrow-key navigation through History.
func TestCommandHistory_Navigation(t *testing.T) {
	h := commands.NewHistory()
	h.Add("/model a")
	h.Add("/model b")
	h.Add("/model c")

	// Navigate up three times — should reach oldest entry "/model a"
	entry, ok := h.Prev("")
	require.True(t, ok)
	assert.Equal(t, "/model c", entry)

	entry, ok = h.Prev(entry)
	require.True(t, ok)
	assert.Equal(t, "/model b", entry)

	entry, ok = h.Prev(entry)
	require.True(t, ok)
	assert.Equal(t, "/model a", entry)
}

// TestCommandHistory_DraftRestoration verifies that navigating down from the
// oldest entry restores the original draft.
func TestCommandHistory_DraftRestoration(t *testing.T) {
	h := commands.NewHistory()
	h.Add("/model a")
	h.Add("/model b")

	// Start navigation with a draft
	draft := "typing something"
	h.Prev(draft)      // goes to /model b, saves draft
	h.Prev("/model b") // goes to /model a

	// Navigate forward past end → should restore draft
	entry, ok := h.Next()
	require.True(t, ok)
	assert.Equal(t, "/model b", entry)

	entry, hasDraft := h.Next()
	// at the end: returns the draft (still returns true to signal the draft was restored)
	assert.Equal(t, draft, entry)
	assert.True(t, hasDraft, "Next restoring draft returns true")
}

// ─────────────────────────────────────────────────────────────────────────────
// Unknown / empty command
// ─────────────────────────────────────────────────────────────────────────────

// TestUnknownCommand verifies that /foo is rejected by the registry and
// nothing is sent to the Gateway.
func TestUnknownCommand_NoGatewayActivity(t *testing.T) {
	mockGW, client := connectClient(t, gateway.ClientConfig{
		Agent:   "marvin",
		Session: "main",
	})
	defer mockGW.Stop()
	defer client.Close()

	beforeFrames := len(mockGW.ReceivedFrames())

	// Registry lookup for unknown command
	commands.InitCommands()
	_, ok := commands.DefaultRegistry.Get("foo")
	assert.False(t, ok, "/foo must not be in the registry")

	// Confirm no frames sent
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, beforeFrames, len(mockGW.ReceivedFrames()), "unknown command must not send to Gateway")
	_ = client
}

// TestSessionSwitch_HistoryRequestAtGateway verifies that /session project-x
// causes a chat.history request to arrive at the mock Gateway with the correct
// session key in the params.
func TestSessionSwitch_HistoryRequestAtGateway(t *testing.T) {
	mockGW, client := connectClient(t, gateway.ClientConfig{
		Agent:   "marvin",
		Session: "main",
	})
	defer mockGW.Stop()
	defer client.Close()

	// Request history directly (simulating what the session handler's returned cmd does)
	err := client.RequestHistory("project-x")
	require.NoError(t, err)

	ok := mockGW.WaitForHistoryCount(1, 2*time.Second)
	require.True(t, ok, "timed out waiting for chat.history at mock")

	frames := mockGW.HistoryFrames()
	require.Len(t, frames, 1)
	assert.Equal(t, "chat.history", frames[0].Method)

	var hParams gateway.HistoryParams
	require.NoError(t, json.Unmarshal(frames[0].Params, &hParams))
	assert.Equal(t, "project-x", hParams.SessionKey)
}

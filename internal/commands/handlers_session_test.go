package commands

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// mockContext is a test implementation of HandlerContext.
type mockContext struct {
	agent     string
	session   string
	model     string
	thinking  string
	timeoutMs int

	messages []string

	// call trackers
	setAgentCalled       bool
	setSessionCalled     bool
	clearMessagesCalled  bool
	updateHeaderCalled   bool
	requestHistoryCalled bool
	requestHistoryKey    string
	reconnectCalled      bool

	// error stubs
	requestHistoryErr error
}

func (m *mockContext) AppendSystemMessage(msg string)    { m.messages = append(m.messages, msg) }
func (m *mockContext) ClearMessages()                    { m.clearMessagesCalled = true }
func (m *mockContext) GetAgent() string                  { return m.agent }
func (m *mockContext) SetAgent(name string)              { m.setAgentCalled = true; m.agent = name }
func (m *mockContext) GetSession() string                { return m.session }
func (m *mockContext) SetSession(key string)             { m.setSessionCalled = true; m.session = key }
func (m *mockContext) GetModel() string                  { return m.model }
func (m *mockContext) SetModel(model string)             { m.model = model }
func (m *mockContext) GetThinking() string               { return m.thinking }
func (m *mockContext) SetThinking(level string)          { m.thinking = level }
func (m *mockContext) GetTimeoutMs() int                 { return m.timeoutMs }
func (m *mockContext) SetTimeoutMs(ms int)               { m.timeoutMs = ms }
func (m *mockContext) GetGatewayURL() string             { return "ws://localhost:8080" }
func (m *mockContext) IsConnected() bool                 { return true }
func (m *mockContext) GetVersion() string                { return "0.2.0-test" }
func (m *mockContext) GetUptime() time.Duration          { return time.Second }
func (m *mockContext) GetMsgSent() int                   { return 0 }
func (m *mockContext) GetMsgRecv() int                   { return 0 }
func (m *mockContext) UpdateHeader()                     { m.updateHeaderCalled = true }
func (m *mockContext) GetWidth() int                     { return 80 }
func (m *mockContext) GetSessionKey() string             { return "agent:" + m.agent + ":" + m.session }
func (m *mockContext) PatchSession(_ SessionPatch) error { return nil }
func (m *mockContext) CloseGateway() error               { return nil }
func (m *mockContext) Reconnect(_ context.Context) error { m.reconnectCalled = true; return nil }
func (m *mockContext) RequestHistory(key string) error {
	m.requestHistoryCalled = true
	m.requestHistoryKey = key
	return m.requestHistoryErr
}

// lastMsg returns the last appended system message, or "" if none.
func (m *mockContext) lastMsg() string {
	if len(m.messages) == 0 {
		return ""
	}
	return m.messages[len(m.messages)-1]
}

// ── /agent tests ──────────────────────────────────────────────────────────────

func TestHandleAgent_NoArgs_Unset(t *testing.T) {
	ctx := &mockContext{}
	cmd := HandleAgent(ctx, nil)
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if !strings.Contains(ctx.lastMsg(), "(default)") {
		t.Errorf("expected (default) in message, got: %q", ctx.lastMsg())
	}
	if !strings.Contains(ctx.lastMsg(), "Usage: /agent") {
		t.Errorf("expected usage hint, got: %q", ctx.lastMsg())
	}
}

func TestHandleAgent_NoArgs_Set(t *testing.T) {
	ctx := &mockContext{agent: "gpt-4"}
	HandleAgent(ctx, nil)
	if !strings.Contains(ctx.lastMsg(), "gpt-4") {
		t.Errorf("expected agent name in message, got: %q", ctx.lastMsg())
	}
}

func TestHandleAgent_SameValue_NoOp(t *testing.T) {
	ctx := &mockContext{agent: "my-agent"}
	cmd := HandleAgent(ctx, []string{"my-agent"})
	if cmd != nil {
		t.Error("expected nil cmd for no-op")
	}
	if ctx.setAgentCalled {
		t.Error("SetAgent should not be called for no-op")
	}
	if !strings.Contains(ctx.lastMsg(), "Already using agent") {
		t.Errorf("expected no-op message, got: %q", ctx.lastMsg())
	}
}

func TestHandleAgent_ValidName(t *testing.T) {
	ctx := &mockContext{}
	cmd := HandleAgent(ctx, []string{"claude"})
	if cmd != nil {
		t.Error("expected nil cmd (agent switch is synchronous)")
	}
	if !ctx.setAgentCalled {
		t.Error("SetAgent should be called")
	}
	if !ctx.clearMessagesCalled {
		t.Error("ClearMessages should be called")
	}
	if !ctx.updateHeaderCalled {
		t.Error("UpdateHeader should be called")
	}
	if ctx.requestHistoryCalled {
		t.Error("RequestHistory must NOT be called after agent switch (architecture.md §3)")
	}
	if ctx.reconnectCalled {
		t.Error("Reconnect must NOT be called (soft switch)")
	}
	if !strings.Contains(ctx.lastMsg(), "Switched to agent 'claude'") {
		t.Errorf("expected success message, got: %q", ctx.lastMsg())
	}
}

func TestHandleAgent_NameWithHyphensUnderscoresDots(t *testing.T) {
	names := []string{"my-agent", "my_agent", "my.agent", "a-b_c.d"}
	for _, name := range names {
		ctx := &mockContext{}
		cmd := HandleAgent(ctx, []string{name})
		if cmd != nil {
			t.Errorf("name %q: expected nil cmd", name)
		}
		if !ctx.setAgentCalled {
			t.Errorf("name %q: SetAgent should be called", name)
		}
	}
}

func TestHandleAgent_NameTooLong(t *testing.T) {
	longName := strings.Repeat("a", 65)
	ctx := &mockContext{}
	HandleAgent(ctx, []string{longName})
	if ctx.setAgentCalled {
		t.Error("SetAgent must not be called for invalid name")
	}
	if !strings.Contains(ctx.lastMsg(), "⚠ /agent:") {
		t.Errorf("expected error message, got: %q", ctx.lastMsg())
	}
}

func TestHandleAgent_NameWithSpaces_Rejected(t *testing.T) {
	// spaces split by Fields, so "my agent" arrives as two args; only first is used
	// simulate what the registry provides: args=["my", "agent"] — first arg validated
	// "my" is valid; the second arg is ignored by the handler (only args[0] used)
	// Test actual rejection: name with embedded space won't reach us as one arg
	// Instead test a name that contains a special char like '@'
	ctx2 := &mockContext{}
	HandleAgent(ctx2, []string{"my@agent"})
	if ctx2.setAgentCalled {
		t.Error("SetAgent must not be called for name with @")
	}
	if !strings.Contains(ctx2.lastMsg(), "⚠ /agent:") {
		t.Errorf("expected error message, got: %q", ctx2.lastMsg())
	}
}

func TestHandleAgent_NameWithSpecialChars_Rejected(t *testing.T) {
	invalid := []string{"agent!", "agent#1", "agent/name", "agent name"}
	for _, name := range invalid {
		ctx := &mockContext{}
		HandleAgent(ctx, []string{name})
		if ctx.setAgentCalled {
			t.Errorf("name %q: SetAgent must not be called", name)
		}
	}
}

func TestHandleAgent_EmptyArg(t *testing.T) {
	ctx := &mockContext{}
	HandleAgent(ctx, []string{""})
	if ctx.setAgentCalled {
		t.Error("SetAgent must not be called for empty arg")
	}
	if !strings.Contains(ctx.lastMsg(), "⚠ /agent:") {
		t.Errorf("expected error message, got: %q", ctx.lastMsg())
	}
}

// ── /session tests ────────────────────────────────────────────────────────────

func TestHandleSession_NoArgs_Unset(t *testing.T) {
	ctx := &mockContext{}
	cmd := HandleSession(ctx, nil)
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if !strings.Contains(ctx.lastMsg(), "(none)") {
		t.Errorf("expected (none) in message, got: %q", ctx.lastMsg())
	}
	if !strings.Contains(ctx.lastMsg(), "Usage: /session") {
		t.Errorf("expected usage hint, got: %q", ctx.lastMsg())
	}
}

func TestHandleSession_NoArgs_Set(t *testing.T) {
	ctx := &mockContext{session: "sess-abc"}
	HandleSession(ctx, nil)
	if !strings.Contains(ctx.lastMsg(), "sess-abc") {
		t.Errorf("expected session key in message, got: %q", ctx.lastMsg())
	}
}

func TestHandleSession_SameValue_NoOp(t *testing.T) {
	ctx := &mockContext{session: "existing-session"}
	cmd := HandleSession(ctx, []string{"existing-session"})
	if cmd != nil {
		t.Error("expected nil cmd for no-op")
	}
	if ctx.setSessionCalled {
		t.Error("SetSession should not be called for no-op")
	}
	if !strings.Contains(ctx.lastMsg(), "Already in session") {
		t.Errorf("expected no-op message, got: %q", ctx.lastMsg())
	}
}

func TestHandleSession_ValidKey(t *testing.T) {
	ctx := &mockContext{}
	cmd := HandleSession(ctx, []string{"my-session"})
	if cmd == nil {
		t.Error("expected non-nil cmd for async history request")
	}
	if !ctx.setSessionCalled {
		t.Error("SetSession should be called")
	}
	if !ctx.clearMessagesCalled {
		t.Error("ClearMessages should be called")
	}
	if !ctx.updateHeaderCalled {
		t.Error("UpdateHeader should be called")
	}
	if ctx.reconnectCalled {
		t.Error("Reconnect must NOT be called (soft switch)")
	}
	if !strings.Contains(ctx.lastMsg(), "Switched to session 'my-session'") {
		t.Errorf("expected success message, got: %q", ctx.lastMsg())
	}
	// Execute the async cmd and verify RequestHistory is called.
	msg := cmd()
	if !ctx.requestHistoryCalled {
		t.Error("RequestHistory should be called by async cmd")
	}
	if ctx.requestHistoryKey != "my-session" {
		t.Errorf("RequestHistory called with wrong key: %q", ctx.requestHistoryKey)
	}
	if msg != nil {
		t.Errorf("expected nil msg on success, got: %v", msg)
	}
}

func TestHandleSession_KeyWithHyphensUnderscores(t *testing.T) {
	keys := []string{"my-session", "my_session", "sess123", "a-b_c"}
	for _, key := range keys {
		ctx := &mockContext{}
		cmd := HandleSession(ctx, []string{key})
		if !ctx.setSessionCalled {
			t.Errorf("key %q: SetSession should be called", key)
		}
		if cmd == nil {
			t.Errorf("key %q: expected non-nil cmd", key)
		}
	}
}

func TestHandleSession_KeyWithDots_Rejected(t *testing.T) {
	// Dots are NOT allowed in session keys (only in agent names).
	ctx := &mockContext{}
	HandleSession(ctx, []string{"my.session"})
	if ctx.setSessionCalled {
		t.Error("SetSession must not be called for key with dot")
	}
	if !strings.Contains(ctx.lastMsg(), "⚠ /session:") {
		t.Errorf("expected error message, got: %q", ctx.lastMsg())
	}
}

func TestHandleSession_KeyTooLong(t *testing.T) {
	longKey := strings.Repeat("a", 65)
	ctx := &mockContext{}
	HandleSession(ctx, []string{longKey})
	if ctx.setSessionCalled {
		t.Error("SetSession must not be called for invalid key")
	}
	if !strings.Contains(ctx.lastMsg(), "⚠ /session:") {
		t.Errorf("expected error message, got: %q", ctx.lastMsg())
	}
}

func TestHandleSession_EmptyArg(t *testing.T) {
	ctx := &mockContext{}
	HandleSession(ctx, []string{""})
	if ctx.setSessionCalled {
		t.Error("SetSession must not be called for empty arg")
	}
	if !strings.Contains(ctx.lastMsg(), "⚠ /session:") {
		t.Errorf("expected error message, got: %q", ctx.lastMsg())
	}
}

func TestHandleSession_RequestHistoryError(t *testing.T) {
	histErr := errors.New("gateway unavailable")
	ctx := &mockContext{requestHistoryErr: histErr}
	cmd := HandleSession(ctx, []string{"new-session"})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	errMsg, ok := msg.(SystemErrorMsg)
	if !ok {
		t.Fatalf("expected SystemErrorMsg, got %T: %v", msg, msg)
	}
	if errMsg.Err == nil {
		t.Fatal("expected non-nil error in SystemErrorMsg")
	}
	if !strings.Contains(errMsg.Err.Error(), "history request failed") {
		t.Errorf("expected wrapped error message, got: %v", errMsg.Err)
	}
	if !errors.Is(errMsg.Err, histErr) {
		t.Errorf("expected error to wrap original, got: %v", errMsg.Err)
	}
}

package commands

import (
	"context"
	"testing"
	"time"
)

// stateTestMock is a test double for HandlerContext.
type stateTestMock struct {
	model     string
	thinking  string
	timeoutMs int
	messages  []string

	setModelCalled    bool
	setThinkingCalled bool
	setTimeoutCalled  bool
}

func newStateMock() *stateTestMock {
	return &stateTestMock{timeoutMs: 30000}
}

func (m *stateTestMock) AppendSystemMessage(msg string)    { m.messages = append(m.messages, msg) }
func (m *stateTestMock) ClearMessages()                    {}
func (m *stateTestMock) GetAgent() string                  { return "test-agent" }
func (m *stateTestMock) SetAgent(_ string)                 {}
func (m *stateTestMock) GetSession() string                { return "test-session" }
func (m *stateTestMock) SetSession(_ string)               {}
func (m *stateTestMock) GetModel() string                  { return m.model }
func (m *stateTestMock) SetModel(model string)             { m.model = model; m.setModelCalled = true }
func (m *stateTestMock) GetThinking() string               { return m.thinking }
func (m *stateTestMock) SetThinking(level string)          { m.thinking = level; m.setThinkingCalled = true }
func (m *stateTestMock) GetTimeoutMs() int                 { return m.timeoutMs }
func (m *stateTestMock) SetTimeoutMs(ms int)               { m.timeoutMs = ms; m.setTimeoutCalled = true }
func (m *stateTestMock) GetGatewayURL() string             { return "ws://localhost" }
func (m *stateTestMock) IsConnected() bool                 { return true }
func (m *stateTestMock) GetVersion() string                { return "0.2.0" }
func (m *stateTestMock) GetUptime() time.Duration          { return 0 }
func (m *stateTestMock) GetMsgSent() int                   { return 0 }
func (m *stateTestMock) GetMsgRecv() int                   { return 0 }
func (m *stateTestMock) RequestHistory(_ string) error     { return nil }
func (m *stateTestMock) Reconnect(_ context.Context) error { return nil }
func (m *stateTestMock) CloseGateway() error               { return nil }
func (m *stateTestMock) UpdateHeader()                     {}
func (m *stateTestMock) GetWidth() int                     { return 80 }

func (m *stateTestMock) lastMsg() string {
	if len(m.messages) == 0 {
		return ""
	}
	return m.messages[len(m.messages)-1]
}

// ---------------------------------------------------------------------------
// /model tests
// ---------------------------------------------------------------------------

func TestHandleModel_NoArgs_DefaultValue(t *testing.T) {
	ctx := newStateMock()
	HandleModel(ctx, nil)
	want := "Current model: (default)\nUsage: /model <model-id>"
	if ctx.lastMsg() != want {
		t.Errorf("got %q, want %q", ctx.lastMsg(), want)
	}
	if ctx.setModelCalled {
		t.Error("SetModel should not be called")
	}
}

func TestHandleModel_NoArgs_WithValue(t *testing.T) {
	ctx := newStateMock()
	ctx.model = "gpt-4o"
	HandleModel(ctx, nil)
	want := "Current model: gpt-4o\nUsage: /model <model-id>"
	if ctx.lastMsg() != want {
		t.Errorf("got %q, want %q", ctx.lastMsg(), want)
	}
}

func TestHandleModel_ValidID(t *testing.T) {
	ctx := newStateMock()
	HandleModel(ctx, []string{"claude-opus-4-5"})
	if !ctx.setModelCalled {
		t.Error("SetModel should be called")
	}
	if ctx.model != "claude-opus-4-5" {
		t.Errorf("model = %q, want claude-opus-4-5", ctx.model)
	}
	if ctx.lastMsg() != "Model set to 'claude-opus-4-5'" {
		t.Errorf("unexpected message: %q", ctx.lastMsg())
	}
}

func TestHandleModel_TooLong(t *testing.T) {
	ctx := newStateMock()
	long := ""
	for range 257 {
		long += "a"
	}
	HandleModel(ctx, []string{long})
	if ctx.setModelCalled {
		t.Error("SetModel must not be called")
	}
	if ctx.lastMsg() != "⚠ /model: model ID must be 256 characters or fewer" {
		t.Errorf("unexpected message: %q", ctx.lastMsg())
	}
}

func TestHandleModel_InvalidChars(t *testing.T) {
	ctx := newStateMock()
	HandleModel(ctx, []string{"invalid model"})
	if ctx.setModelCalled {
		t.Error("SetModel must not be called")
	}
	want := "⚠ /model: model ID may only contain alphanumeric characters, hyphens, underscores, dots, and slashes"
	if ctx.lastMsg() != want {
		t.Errorf("got %q, want %q", ctx.lastMsg(), want)
	}
}

func TestHandleModel_SameValue_NoOp(t *testing.T) {
	ctx := newStateMock()
	ctx.model = "gpt-4o"
	HandleModel(ctx, []string{"gpt-4o"})
	if ctx.setModelCalled {
		t.Error("SetModel must not be called for same value")
	}
	if ctx.lastMsg() != "Already using model 'gpt-4o'" {
		t.Errorf("unexpected message: %q", ctx.lastMsg())
	}
}

func TestHandleModel_LeadingSlashStripped(t *testing.T) {
	ctx := newStateMock()
	HandleModel(ctx, []string{"/gpt-4o"})
	if ctx.model != "gpt-4o" {
		t.Errorf("expected leading slash stripped, got %q", ctx.model)
	}
}

func TestHandleModel_EmptyString(t *testing.T) {
	ctx := newStateMock()
	HandleModel(ctx, []string{""})
	if ctx.setModelCalled {
		t.Error("SetModel must not be called for empty string")
	}
}

func TestHandleModel_SlashAllowed(t *testing.T) {
	ctx := newStateMock()
	HandleModel(ctx, []string{"provider/model-name"})
	if !ctx.setModelCalled {
		t.Error("SetModel should be called for provider/model-name")
	}
}

func TestHandleModel_DotAllowed(t *testing.T) {
	ctx := newStateMock()
	HandleModel(ctx, []string{"gpt-4.0"})
	if !ctx.setModelCalled {
		t.Error("SetModel should be called for gpt-4.0")
	}
}

func TestHandleModel_SpaceRejected(t *testing.T) {
	ctx := newStateMock()
	HandleModel(ctx, []string{"gpt 4"})
	if ctx.setModelCalled {
		t.Error("SetModel must not be called for model with space")
	}
}

// ---------------------------------------------------------------------------
// /thinking tests
// ---------------------------------------------------------------------------

func TestHandleThinking_NoArgs_Default(t *testing.T) {
	ctx := newStateMock()
	HandleThinking(ctx, nil)
	want := "Current thinking: (default)\nUsage: /thinking <off|minimal|low|medium|high>"
	if ctx.lastMsg() != want {
		t.Errorf("got %q, want %q", ctx.lastMsg(), want)
	}
}

func TestHandleThinking_NoArgs_WithValue(t *testing.T) {
	ctx := newStateMock()
	ctx.thinking = "low"
	HandleThinking(ctx, nil)
	want := "Current thinking: low\nUsage: /thinking <off|minimal|low|medium|high>"
	if ctx.lastMsg() != want {
		t.Errorf("got %q, want %q", ctx.lastMsg(), want)
	}
}

func TestHandleThinking_ValidLevels(t *testing.T) {
	levels := []string{"off", "minimal", "low", "medium", "high"}
	for _, lvl := range levels {
		t.Run(lvl, func(t *testing.T) {
			ctx := newStateMock()
			HandleThinking(ctx, []string{lvl})
			if !ctx.setThinkingCalled {
				t.Error("SetThinking should be called")
			}
			if ctx.thinking != lvl {
				t.Errorf("thinking = %q, want %q", ctx.thinking, lvl)
			}
			if ctx.lastMsg() != "Thinking set to '"+lvl+"'" {
				t.Errorf("unexpected message: %q", ctx.lastMsg())
			}
		})
	}
}

func TestHandleThinking_UppercaseNormalised(t *testing.T) {
	ctx := newStateMock()
	HandleThinking(ctx, []string{"HIGH"})
	if ctx.thinking != "high" {
		t.Errorf("thinking = %q, want 'high'", ctx.thinking)
	}
}

func TestHandleThinking_Invalid(t *testing.T) {
	ctx := newStateMock()
	HandleThinking(ctx, []string{"turbo"})
	if ctx.setThinkingCalled {
		t.Error("SetThinking must not be called")
	}
	want := "⚠ /thinking: thinking level must be one of: off, minimal, low, medium, high"
	if ctx.lastMsg() != want {
		t.Errorf("got %q, want %q", ctx.lastMsg(), want)
	}
}

func TestHandleThinking_EmptyString(t *testing.T) {
	ctx := newStateMock()
	HandleThinking(ctx, []string{""})
	if ctx.setThinkingCalled {
		t.Error("SetThinking must not be called for empty string")
	}
}

// ---------------------------------------------------------------------------
// /timeout tests
// ---------------------------------------------------------------------------

func TestHandleTimeout_NoArgs(t *testing.T) {
	ctx := newStateMock()
	ctx.timeoutMs = 30000
	HandleTimeout(ctx, nil)
	want := "Current timeout: 30000ms\nUsage: /timeout <1000-600000>"
	if ctx.lastMsg() != want {
		t.Errorf("got %q, want %q", ctx.lastMsg(), want)
	}
}

func TestHandleTimeout_LowerBound(t *testing.T) {
	ctx := newStateMock()
	HandleTimeout(ctx, []string{"1000"})
	if !ctx.setTimeoutCalled {
		t.Error("SetTimeoutMs should be called")
	}
	if ctx.timeoutMs != 1000 {
		t.Errorf("timeoutMs = %d, want 1000", ctx.timeoutMs)
	}
	if ctx.lastMsg() != "Timeout set to 1000ms" {
		t.Errorf("unexpected message: %q", ctx.lastMsg())
	}
}

func TestHandleTimeout_UpperBound(t *testing.T) {
	ctx := newStateMock()
	HandleTimeout(ctx, []string{"600000"})
	if !ctx.setTimeoutCalled {
		t.Error("SetTimeoutMs should be called")
	}
	if ctx.timeoutMs != 600000 {
		t.Errorf("timeoutMs = %d, want 600000", ctx.timeoutMs)
	}
}

func TestHandleTimeout_BelowLower(t *testing.T) {
	ctx := newStateMock()
	HandleTimeout(ctx, []string{"999"})
	if ctx.setTimeoutCalled {
		t.Error("SetTimeoutMs must not be called")
	}
	want := "⚠ /timeout: timeout must be between 1000 and 600000 milliseconds"
	if ctx.lastMsg() != want {
		t.Errorf("got %q, want %q", ctx.lastMsg(), want)
	}
}

func TestHandleTimeout_AboveUpper(t *testing.T) {
	ctx := newStateMock()
	HandleTimeout(ctx, []string{"600001"})
	if ctx.setTimeoutCalled {
		t.Error("SetTimeoutMs must not be called")
	}
}

func TestHandleTimeout_Float(t *testing.T) {
	ctx := newStateMock()
	HandleTimeout(ctx, []string{"30000.5"})
	if ctx.setTimeoutCalled {
		t.Error("SetTimeoutMs must not be called for float")
	}
	want := "⚠ /timeout: timeout must be an integer (milliseconds)"
	if ctx.lastMsg() != want {
		t.Errorf("got %q, want %q", ctx.lastMsg(), want)
	}
}

func TestHandleTimeout_NonNumeric(t *testing.T) {
	ctx := newStateMock()
	HandleTimeout(ctx, []string{"abc"})
	if ctx.setTimeoutCalled {
		t.Error("SetTimeoutMs must not be called")
	}
}

func TestHandleTimeout_EmptyString(t *testing.T) {
	ctx := newStateMock()
	HandleTimeout(ctx, []string{""})
	if ctx.setTimeoutCalled {
		t.Error("SetTimeoutMs must not be called for empty string")
	}
}

// ---------------------------------------------------------------------------
// CmdError / CmdErrorWithUsage helpers
// ---------------------------------------------------------------------------

func TestCmdError(t *testing.T) {
	got := CmdError("model", "some error")
	want := "⚠ /model: some error"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCmdErrorWithUsage(t *testing.T) {
	got := CmdErrorWithUsage("timeout", "too small", "/timeout <1000-600000>")
	want := "⚠ /timeout: too small\nUsage: /timeout <1000-600000>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

package commands

import (
	"context"
	"strings"
	"testing"
	"time"
)

// displayMockCtx is a test implementation of HandlerContext.
type displayMockCtx struct {
	agent      string
	session    string
	model      string
	thinking   string
	timeoutMs  int
	gatewayURL string
	connected  bool
	version    string
	uptime     time.Duration
	msgSent    int
	msgRecv    int
	width      int

	messages      []string
	cleared       bool
	gatewayClosed bool
}

func newDisplayMock() *displayMockCtx {
	return &displayMockCtx{
		agent:      "daedalus",
		session:    "my-session",
		model:      "claude-opus-4-5",
		thinking:   "low",
		timeoutMs:  30000,
		gatewayURL: "ws://localhost:8080",
		connected:  true,
		version:    "0.2.0",
		uptime:     90 * time.Second,
		msgSent:    5,
		msgRecv:    10,
		width:      80,
	}
}

func (m *displayMockCtx) AppendSystemMessage(msg string)         { m.messages = append(m.messages, msg) }
func (m *displayMockCtx) ClearMessages()                         { m.cleared = true }
func (m *displayMockCtx) GetAgent() string                       { return m.agent }
func (m *displayMockCtx) SetAgent(name string)                   { m.agent = name }
func (m *displayMockCtx) GetSession() string                     { return m.session }
func (m *displayMockCtx) SetSession(key string)                  { m.session = key }
func (m *displayMockCtx) GetModel() string                       { return m.model }
func (m *displayMockCtx) SetModel(model string)                  { m.model = model }
func (m *displayMockCtx) GetThinking() string                    { return m.thinking }
func (m *displayMockCtx) SetThinking(level string)               { m.thinking = level }
func (m *displayMockCtx) GetTimeoutMs() int                      { return m.timeoutMs }
func (m *displayMockCtx) SetTimeoutMs(ms int)                    { m.timeoutMs = ms }
func (m *displayMockCtx) GetGatewayURL() string                  { return m.gatewayURL }
func (m *displayMockCtx) IsConnected() bool                      { return m.connected }
func (m *displayMockCtx) GetVersion() string                     { return m.version }
func (m *displayMockCtx) GetUptime() time.Duration               { return m.uptime }
func (m *displayMockCtx) GetMsgSent() int                        { return m.msgSent }
func (m *displayMockCtx) GetMsgRecv() int                        { return m.msgRecv }
func (m *displayMockCtx) UpdateHeader()                          {}
func (m *displayMockCtx) GetWidth() int                          { return m.width }
func (m *displayMockCtx) RequestHistory(sessionKey string) error { return nil }
func (m *displayMockCtx) Reconnect(ctx context.Context) error    { return nil }
func (m *displayMockCtx) CloseGateway() error {
	m.gatewayClosed = true
	return nil
}

// --- /help tests ---

func TestHandleHelp_NoArgs(t *testing.T) {
	InitCommands()
	ctx := newDisplayMock()
	cmd := handleHelp(ctx, nil)
	if cmd != nil {
		t.Error("expected nil tea.Cmd")
	}
	if len(ctx.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(ctx.messages))
	}
	output := ctx.messages[0]
	for _, line := range strings.Split(output, "\n") {
		if len([]rune(line)) > 80 {
			t.Errorf("line exceeds 80 chars (%d): %q", len([]rune(line)), line)
		}
	}
}

func TestHandleHelp_AllCommands(t *testing.T) {
	InitCommands()
	allCmds := DefaultRegistry.All()
	for _, def := range allCmds {
		t.Run(def.Name, func(t *testing.T) {
			ctx := newDisplayMock()
			handleHelp(ctx, []string{def.Name})
			if len(ctx.messages) == 0 {
				t.Fatal("expected output message")
			}
			if !strings.Contains(ctx.messages[0], "/"+def.Name) {
				t.Errorf("expected output to contain /%s", def.Name)
			}
		})
		// Also test with leading slash.
		t.Run("/"+def.Name, func(t *testing.T) {
			ctx := newDisplayMock()
			handleHelp(ctx, []string{"/" + def.Name})
			if len(ctx.messages) == 0 {
				t.Fatal("expected output message")
			}
		})
	}
}

func TestHandleHelp_Unknown(t *testing.T) {
	InitCommands()
	ctx := newDisplayMock()
	handleHelp(ctx, []string{"boguscmd"})
	if len(ctx.messages) == 0 {
		t.Fatal("expected error message")
	}
	want := "Unknown command: /boguscmd. Type /help for available commands."
	if ctx.messages[0] != want {
		t.Errorf("got %q, want %q", ctx.messages[0], want)
	}
}

// --- /status tests ---

func TestHandleStatus_KnownValues(t *testing.T) {
	InitCommands()
	ctx := newDisplayMock()
	cmd := handleStatus(ctx, nil)
	if cmd != nil {
		t.Error("expected nil tea.Cmd")
	}
	if len(ctx.messages) == 0 {
		t.Fatal("expected status output")
	}
	out := ctx.messages[0]
	checks := []string{
		"daedalus", "my-session", "claude-opus-4-5", "low",
		"30000ms", "ws://localhost:8080", "0.2.0",
		"1m 30s", // 90s uptime
		"5", "10",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("status output missing %q", c)
		}
	}
}

func TestHandleStatus_EmptyFields(t *testing.T) {
	InitCommands()
	ctx := newDisplayMock()
	ctx.model = ""
	ctx.thinking = ""
	ctx.session = ""
	ctx.version = ""
	ctx.uptime = 0
	handleStatus(ctx, nil)
	out := ctx.messages[0]
	if !strings.Contains(out, "(default)") {
		t.Error("expected (default) for empty model/thinking")
	}
	if !strings.Contains(out, "(none)") {
		t.Error("expected (none) for empty session")
	}
	if !strings.Contains(out, "unknown") {
		t.Error("expected unknown for empty version")
	}
	if !strings.Contains(out, "not connected") {
		t.Error("expected 'not connected' for zero uptime")
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{90 * time.Second, "1m 30s"},
		{3661 * time.Second, "1h 1m 1s"},
		{7322 * time.Second, "2h 2m 2s"},
	}
	for _, tc := range cases {
		got := formatDuration(tc.d)
		if got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// --- /clear tests ---

func TestHandleClear(t *testing.T) {
	InitCommands()
	ctx := newDisplayMock()
	cmd := handleClear(ctx, nil)
	if cmd != nil {
		t.Error("expected nil tea.Cmd")
	}
	if !ctx.cleared {
		t.Error("expected ClearMessages() to be called")
	}
	if len(ctx.messages) == 0 || ctx.messages[0] != "Messages cleared" {
		t.Errorf("expected confirmation message, got %v", ctx.messages)
	}
}

// --- /exit tests ---

func TestHandleExit(t *testing.T) {
	InitCommands()
	ctx := newDisplayMock()
	cmd := handleExit(ctx, nil)
	if !ctx.gatewayClosed {
		t.Error("expected CloseGateway() to be called")
	}
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd")
	}
	// Verify cmd is non-nil (tea.Quit); CloseGateway check above is sufficient.
	_ = cmd
}

// --- /history tests ---

func TestHandleHistory(t *testing.T) {
	InitCommands()
	ctx := newDisplayMock()
	cmd := handleHistory(ctx, nil)
	if cmd != nil {
		t.Error("expected nil tea.Cmd")
	}
	if len(ctx.messages) == 0 || ctx.messages[0] != "Coming in v0.3" {
		t.Errorf("expected stub message, got %v", ctx.messages)
	}
}

// --- WireDisplayHandlers tests ---

func TestWireDisplayHandlers(t *testing.T) {
	InitCommands()
	names := []string{"help", "status", "clear", "exit", "history"}
	for _, name := range names {
		def, ok := DefaultRegistry.Get(name)
		if !ok {
			t.Errorf("command %q not found in registry", name)
			continue
		}
		if def.Handler == nil {
			t.Errorf("handler for %q is nil after WireDisplayHandlers", name)
		}
	}
	// Check quit alias.
	def, ok := DefaultRegistry.Get("quit")
	if !ok {
		t.Error("quit alias not found")
	} else if def.Handler == nil {
		t.Error("quit alias handler is nil")
	}
}

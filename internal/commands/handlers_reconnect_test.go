package commands

import (
	"context"
	"errors"
	"testing"
	"time"
)

// reconnectMockCtx is a minimal HandlerContext for /reconnect tests.
// All HandlerContext methods not under test are no-ops.
type reconnectMockCtx struct {
	reconnectErr     error
	reconnectCtx     context.Context
	systemMessages   []string
	deadlineRecorded bool
}

// -- methods under test --

func (m *reconnectMockCtx) AppendSystemMessage(msg string) {
	m.systemMessages = append(m.systemMessages, msg)
}

func (m *reconnectMockCtx) Reconnect(ctx context.Context) error {
	m.reconnectCtx = ctx
	if _, ok := ctx.Deadline(); ok {
		m.deadlineRecorded = true
	}
	return m.reconnectErr
}

// -- no-op stubs to satisfy HandlerContext --

func (m *reconnectMockCtx) ClearMessages()                {}
func (m *reconnectMockCtx) GetAgent() string              { return "" }
func (m *reconnectMockCtx) SetAgent(_ string)             {}
func (m *reconnectMockCtx) GetSession() string            { return "" }
func (m *reconnectMockCtx) SetSession(_ string)           {}
func (m *reconnectMockCtx) GetModel() string              { return "" }
func (m *reconnectMockCtx) SetModel(_ string)             {}
func (m *reconnectMockCtx) GetThinking() string           { return "" }
func (m *reconnectMockCtx) SetThinking(_ string)          {}
func (m *reconnectMockCtx) GetTimeoutMs() int             { return 0 }
func (m *reconnectMockCtx) SetTimeoutMs(_ int)            {}
func (m *reconnectMockCtx) GetGatewayURL() string         { return "" }
func (m *reconnectMockCtx) IsConnected() bool             { return false }
func (m *reconnectMockCtx) GetVersion() string            { return "" }
func (m *reconnectMockCtx) GetUptime() time.Duration      { return 0 }
func (m *reconnectMockCtx) GetMsgSent() int               { return 0 }
func (m *reconnectMockCtx) GetMsgRecv() int               { return 0 }
func (m *reconnectMockCtx) RequestHistory(_ string) error { return nil }
func (m *reconnectMockCtx) CloseGateway() error           { return nil }
func (m *reconnectMockCtx) UpdateHeader()                 {}
func (m *reconnectMockCtx) GetWidth() int                 { return 80 }

// --- success path ---

func TestHandleReconnect_Success(t *testing.T) {
	mock := &reconnectMockCtx{}
	cmd := handleReconnect(mock, nil)

	// System message must be appended before returning
	if len(mock.systemMessages) != 1 || mock.systemMessages[0] != "Reconnecting..." {
		t.Fatalf("expected 'Reconnecting...' system message, got %v", mock.systemMessages)
	}

	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd")
	}

	msg := cmd()
	reconMsg, ok := msg.(ReconnectedMsg)
	if !ok {
		t.Fatalf("expected ReconnectedMsg, got %T", msg)
	}
	if reconMsg.At.IsZero() {
		t.Error("ReconnectedMsg.At should not be zero")
	}
	if time.Since(reconMsg.At) > 5*time.Second {
		t.Error("ReconnectedMsg.At is unexpectedly old")
	}
}

// --- failure path ---

func TestHandleReconnect_Failure(t *testing.T) {
	sentinel := errors.New("connection refused")
	mock := &reconnectMockCtx{reconnectErr: sentinel}
	cmd := handleReconnect(mock, nil)

	msg := cmd()
	errMsg, ok := msg.(SystemErrorMsg)
	if !ok {
		t.Fatalf("expected SystemErrorMsg, got %T", msg)
	}
	if errMsg.Err == nil {
		t.Fatal("expected non-nil error")
	}
	if !errors.Is(errMsg.Err, sentinel) {
		t.Errorf("error chain should wrap sentinel: %v", errMsg.Err)
	}
}

// --- 30-second timeout applied ---

func TestHandleReconnect_TimeoutApplied(t *testing.T) {
	mock := &reconnectMockCtx{}
	cmd := handleReconnect(mock, nil)
	_ = cmd()

	if !mock.deadlineRecorded {
		t.Error("expected context.WithTimeout deadline to be set on Reconnect call")
	}
	if mock.reconnectCtx == nil {
		t.Fatal("Reconnect was not called")
	}
	deadline, ok := mock.reconnectCtx.Deadline()
	if !ok {
		t.Fatal("context has no deadline")
	}
	remaining := time.Until(deadline)
	// Allow generous window for test latency; should be close to 30 s.
	if remaining < 25*time.Second || remaining > 31*time.Second {
		t.Errorf("unexpected timeout remaining: %v", remaining)
	}
}

// --- system message appended before cmd returned ---

func TestHandleReconnect_SystemMessageBeforeCmd(t *testing.T) {
	mock := &reconnectMockCtx{}
	_ = handleReconnect(mock, nil)

	if len(mock.systemMessages) == 0 {
		t.Fatal("system message not appended")
	}
	if mock.systemMessages[0] != "Reconnecting..." {
		t.Errorf("unexpected system message: %q", mock.systemMessages[0])
	}
}

// --- args are ignored ---

func TestHandleReconnect_ArgsIgnored(t *testing.T) {
	mock := &reconnectMockCtx{}
	cmd := handleReconnect(mock, []string{"extra", "args"})
	if cmd == nil {
		t.Fatal("expected non-nil cmd even with spurious args")
	}
}

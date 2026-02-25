package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

// boolPtr returns a pointer to a bool literal.
func boolPtr(b bool) *bool { return &b }

// buildChallengeMsg returns a raw WebSocket text frame for a connect.challenge event.
func buildChallengeMsg(t *testing.T) []byte {
	t.Helper()
	payload := mustJSON(t, ConnectChallengePayload{Nonce: "test-nonce", TS: 1234567890})
	frame := InboundFrame{
		Type:    "event",
		Event:   "connect.challenge",
		Payload: json.RawMessage(payload),
	}
	return mustJSON(t, frame)
}

// authHelloOK returns a raw WebSocket text frame for a successful hello-ok response.
func authHelloOK(t *testing.T) []byte {
	t.Helper()
	payload := mustJSON(t, HelloOKPayload{
		Type:     "hello-ok",
		Protocol: 3,
		Server:   HelloServerInfo{Version: "1.0.0", ConnID: "conn-abc"},
		Features: HelloFeatures{
			Methods: []string{"chat.send", "sessions.list"},
			Events:  []string{"chat.event"},
		},
	})
	frame := InboundFrame{
		Type:    "res",
		ID:      "connect-1",
		OK:      boolPtr(true),
		Payload: json.RawMessage(payload),
	}
	return mustJSON(t, frame)
}

// buildHelloFail returns a raw WebSocket text frame for a failed authentication response.
// The payload still has type "hello-ok" but the frame ok=false so ParseInbound produces
// KindAuthResult with Success=false.
func buildHelloFail(t *testing.T) []byte {
	t.Helper()
	payload := mustJSON(t, HelloOKPayload{
		Type:     "hello-ok",
		Protocol: 3,
		Server:   HelloServerInfo{Version: "1.0.0", ConnID: "conn-abc"},
		Features: HelloFeatures{},
	})
	frame := InboundFrame{
		Type:    "res",
		ID:      "connect-1",
		OK:      boolPtr(false),
		Payload: json.RawMessage(payload),
	}
	return mustJSON(t, frame)
}

// hangConn is a WebSocketConn that blocks forever in ReadMessage.
// Used to test context-cancellation / timeout paths.
type hangConn struct{}

func (h *hangConn) ReadMessage() (int, []byte, error) {
	// Block indefinitely — context cancellation is tested via select in readEvent.
	select {}
}
func (h *hangConn) WriteMessage(_ int, _ []byte) error { return nil }
func (h *hangConn) Close() error                       { return nil }
func (h *hangConn) SetReadDeadline(_ time.Time) error  { return nil }
func (h *hangConn) SetWriteDeadline(_ time.Time) error { return nil }

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

// TestAuthenticate_SuccessToken verifies a complete handshake using a token credential.
// Expected flow: connect.challenge → connect request (token) → hello-ok → KindAuthResult/Success=true.
func TestAuthenticate_SuccessToken(t *testing.T) {
	mock := &MockWebSocketConn{}
	mock.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock.EnqueueRead(websocket.TextMessage, authHelloOK(t))

	evt, err := Authenticate(context.Background(), mock, AuthConfig{Token: "my-secret-token"})
	require.NoError(t, err)
	assert.Equal(t, KindAuthResult, evt.Kind)
	assert.True(t, evt.Success)
	assert.Equal(t, "1.0.0", evt.Version)

	// Verify exactly one connect request was sent.
	sent := mock.SentMessages()
	require.Len(t, sent, 1, "expected exactly one outbound frame (connect request)")

	// Decode the sent frame and verify its shape.
	var outFrame OutboundFrame
	require.NoError(t, json.Unmarshal(sent[0], &outFrame))
	assert.Equal(t, "req", outFrame.Type)
	assert.Equal(t, "connect", outFrame.Method)
	assert.Equal(t, "connect-1", outFrame.ID)

	// Verify auth credentials in params.
	var params ConnectParams
	require.NoError(t, json.Unmarshal(outFrame.Params, &params))
	assert.Equal(t, "my-secret-token", params.Auth.Token)
	assert.Equal(t, "operator", params.Role)
	assert.Equal(t, []string{"operator.read", "operator.write"}, params.Scopes)
	assert.Equal(t, 3, params.MinProtocol)
	assert.Equal(t, 3, params.MaxProtocol)
}

// TestAuthenticate_SuccessPassword verifies the handshake using a password credential.
func TestAuthenticate_SuccessPassword(t *testing.T) {
	mock := &MockWebSocketConn{}
	mock.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock.EnqueueRead(websocket.TextMessage, authHelloOK(t))

	evt, err := Authenticate(context.Background(), mock, AuthConfig{Password: "hunter2"})
	require.NoError(t, err)
	assert.Equal(t, KindAuthResult, evt.Kind)
	assert.True(t, evt.Success)

	// Verify password was sent (not token).
	sent := mock.SentMessages()
	require.Len(t, sent, 1)

	var outFrame OutboundFrame
	require.NoError(t, json.Unmarshal(sent[0], &outFrame))

	var params ConnectParams
	require.NoError(t, json.Unmarshal(outFrame.Params, &params))
	assert.Equal(t, "hunter2", params.Auth.Password)
	assert.Empty(t, params.Auth.Token)
}

// TestAuthenticate_Failure verifies that a hello-fail response returns ErrAuthFailed.
func TestAuthenticate_Failure(t *testing.T) {
	mock := &MockWebSocketConn{}
	mock.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock.EnqueueRead(websocket.TextMessage, buildHelloFail(t))

	_, err := Authenticate(context.Background(), mock, AuthConfig{Token: "bad-token"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthFailed)
}

// TestAuthenticate_Timeout verifies that a cancelled context returns ErrAuthTimeout.
// hangConn blocks ReadMessage indefinitely so the goroutine never produces a result,
// guaranteeing the ctx.Done() path is taken deterministically.
func TestAuthenticate_Timeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before Authenticate is called

	_, err := Authenticate(ctx, &hangConn{}, AuthConfig{Token: "tok"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthTimeout)
}

// TestAuthenticate_ConnDrop verifies that a ReadMessage error during the challenge phase
// is propagated back to the caller.
func TestAuthenticate_ConnDrop(t *testing.T) {
	connErr := errors.New("connection reset by peer")
	mock := &MockWebSocketConn{}
	mock.EnqueueReadError(connErr)

	_, err := Authenticate(context.Background(), mock, AuthConfig{Token: "tok"})
	require.Error(t, err)
	assert.ErrorIs(t, err, connErr)
}

// TestAuthenticate_ConnDropAfterChallenge verifies ReadMessage errors after the challenge
// (i.e., during hello-ok wait) are also propagated.
func TestAuthenticate_ConnDropAfterChallenge(t *testing.T) {
	connErr := errors.New("EOF after challenge")
	mock := &MockWebSocketConn{}
	mock.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock.EnqueueReadError(connErr)

	_, err := Authenticate(context.Background(), mock, AuthConfig{Token: "tok"})
	require.Error(t, err)
	assert.ErrorIs(t, err, connErr)
}

// TestAuthenticate_WrongFirstMessage verifies that a non-challenge first message errors
// with an informative message (not ErrAuthFailed).
func TestAuthenticate_WrongFirstMessage(t *testing.T) {
	// Send a hello-ok where a challenge is expected.
	mock := &MockWebSocketConn{}
	mock.EnqueueRead(websocket.TextMessage, authHelloOK(t))

	_, err := Authenticate(context.Background(), mock, AuthConfig{Token: "tok"})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrAuthFailed)
}

// TestAuthenticate_WriteError verifies that a failure to send the connect request
// is returned as an error.
func TestAuthenticate_WriteError(t *testing.T) {
	writeErr := errors.New("write: broken pipe")
	mock := &MockWebSocketConn{}
	mock.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock.WriteErr = writeErr

	_, err := Authenticate(context.Background(), mock, AuthConfig{Token: "tok"})
	require.Error(t, err)
	assert.ErrorIs(t, err, writeErr)
}

// TestAuthenticate_DefaultTimeout verifies that a context without a deadline gets
// the default 30-second timeout applied (we just check no panic / infinite hang).
func TestAuthenticate_DefaultTimeout(t *testing.T) {
	// Use a context with no deadline — Authenticate should add defaultAuthTimeout.
	// We provide a quick mock that succeeds so the test doesn't take 30 s.
	mock := &MockWebSocketConn{}
	mock.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock.EnqueueRead(websocket.TextMessage, authHelloOK(t))

	evt, err := Authenticate(context.Background(), mock, AuthConfig{Token: "tok"})
	require.NoError(t, err)
	assert.True(t, evt.Success)
}

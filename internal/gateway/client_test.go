package gateway

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────
// Test helpers (client-specific)
// ─────────────────────────────────────────────

// newTestClient creates a Client whose dial function returns the provided mock.
func newTestClient(t *testing.T, cfg ClientConfig, mock *MockWebSocketConn) *Client {
	t.Helper()
	c := NewClient(cfg)
	c.dial = func(_ context.Context, _ string) (WebSocketConn, error) {
		return mock, nil
	}
	return c
}

// buildChatDelta returns a JSON chat.event with state=delta.
func buildChatDelta(t *testing.T, content string) []byte {
	t.Helper()
	f := InboundFrame{
		Type:  "event",
		Event: "chat.event",
		Payload: mustJSON(t, ChatEventPayload{
			State:   "delta",
			Message: &ChatEventMsg{Role: "assistant", Content: content},
		}),
	}
	return mustJSON(t, f)
}

// buildChatFinal returns a JSON chat.event with state=final.
func buildChatFinal(t *testing.T, content string) []byte {
	t.Helper()
	f := InboundFrame{
		Type:  "event",
		Event: "chat.event",
		Payload: mustJSON(t, ChatEventPayload{
			State:   "final",
			Message: &ChatEventMsg{Role: "assistant", Content: content},
		}),
	}
	return mustJSON(t, f)
}

// setupConnectedClient creates a mock + connected client ready for testing.
// The mock has the challenge and hello-ok enqueued; ReadErr is set to
// ErrConnectionClosed so the read loop eventually exits cleanly.
func setupConnectedClient(t *testing.T, cfg ClientConfig) (*Client, *MockWebSocketConn) {
	t.Helper()
	mock := &MockWebSocketConn{}
	mock.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock.EnqueueRead(websocket.TextMessage, authHelloOK(t))
	mock.ReadErr = ErrConnectionClosed

	c := newTestClient(t, cfg, mock)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	require.NoError(t, c.Connect(ctx))
	return c, mock
}

// ─────────────────────────────────────────────
// Connect & Auth
// ─────────────────────────────────────────────

func TestClient_Connect_Success(t *testing.T) {
	cfg := ClientConfig{URL: "ws://test", Token: "tok", HistoryLimit: 0}
	c, mock := setupConnectedClient(t, cfg)
	defer func() { require.NoError(t, c.Close()) }()

	assert.Equal(t, StateConnected, c.State())

	// Should have sent the connect req frame
	sent := mock.SentMessages()
	require.NotEmpty(t, sent)
	var frame OutboundFrame
	require.NoError(t, json.Unmarshal(sent[0], &frame))
	assert.Equal(t, "req", frame.Type)
	assert.Equal(t, "connect", frame.Method)
}

func TestClient_Connect_AuthFailed(t *testing.T) {
	mock := &MockWebSocketConn{}
	mock.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock.EnqueueRead(websocket.TextMessage, buildHelloFail(t))

	cfg := ClientConfig{URL: "ws://test", Token: "bad-tok"}
	c := newTestClient(t, cfg, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	assert.ErrorIs(t, err, ErrAuthFailed)
	assert.Equal(t, StateDisconnected, c.State())
}

func TestClient_Connect_DialError(t *testing.T) {
	cfg := ClientConfig{URL: "ws://test", Token: "tok"}
	c := NewClient(cfg)
	c.dial = func(_ context.Context, _ string) (WebSocketConn, error) {
		return nil, ErrConnectionClosed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	assert.Error(t, err)
	assert.Equal(t, StateDisconnected, c.State())
}

// ─────────────────────────────────────────────
// Send
// ─────────────────────────────────────────────

func TestClient_Send_Connected(t *testing.T) {
	cfg := ClientConfig{URL: "ws://test", Token: "tok"}
	c, mock := setupConnectedClient(t, cfg)
	defer func() { require.NoError(t, c.Close()) }()

	err := c.Send(OutboundMessage{Type: "message", Payload: "hello"})
	assert.NoError(t, err)

	// Give write loop time to process
	time.Sleep(50 * time.Millisecond)

	sent := mock.SentMessages()
	// sent[0] = connect req, sent[1] = user message
	require.GreaterOrEqual(t, len(sent), 2)
}

func TestClient_Send_Disconnected_Queues(t *testing.T) {
	cfg := ClientConfig{URL: "ws://test", Token: "tok"}
	c := NewClient(cfg) // not connected

	err := c.Send(OutboundMessage{Type: "message"})
	assert.NoError(t, err)
	assert.Equal(t, 1, c.queue.Len())
}

func TestClient_Send_AfterClose(t *testing.T) {
	cfg := ClientConfig{URL: "ws://test", Token: "tok"}
	c := NewClient(cfg)
	require.NoError(t, c.Close())

	err := c.Send(OutboundMessage{Type: "message"})
	assert.ErrorIs(t, err, ErrShutdown)
}

func TestClient_Send_QueueFull(t *testing.T) {
	cfg := ClientConfig{URL: "ws://test", Token: "tok"}
	c := NewClient(cfg)
	// fill the queue to capacity
	c.queue = NewQueue(1)
	c.queue.Enqueue(OutboundMessage{Type: "existing"})

	// Next enqueue drops oldest and reports ErrQueueFull
	err := c.Send(OutboundMessage{Type: "overflow"})
	assert.ErrorIs(t, err, ErrQueueFull)
}

// ─────────────────────────────────────────────
// Receive messages
// ─────────────────────────────────────────────

func TestClient_ReceiveMessages(t *testing.T) {
	mock := &MockWebSocketConn{}
	mock.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock.EnqueueRead(websocket.TextMessage, authHelloOK(t))
	mock.EnqueueRead(websocket.TextMessage, buildChatDelta(t, "Hello"))
	mock.EnqueueRead(websocket.TextMessage, buildChatFinal(t, "Hello world"))
	mock.ReadErr = ErrConnectionClosed

	cfg := ClientConfig{URL: "ws://test", Token: "tok"}
	c := newTestClient(t, cfg, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, c.Connect(ctx))
	defer func() { require.NoError(t, c.Close()) }()

	// Collect up to 4 events with a timeout
	var events []InboundEvent
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

loop:
	for {
		select {
		case evt, ok := <-c.Messages():
			if !ok {
				break loop
			}
			events = append(events, evt)
			if len(events) >= 4 {
				break loop
			}
		case <-timer.C:
			break loop
		}
	}

	require.GreaterOrEqual(t, len(events), 2)

	var gotDelta, gotFinal bool
	for _, e := range events {
		switch e.Kind {
		case KindToken:
			assert.Equal(t, "Hello", e.Content)
			gotDelta = true
		case KindMessage:
			assert.Equal(t, "Hello world", e.Content)
			gotFinal = true
		}
	}
	assert.True(t, gotDelta, "expected KindToken event")
	assert.True(t, gotFinal, "expected KindMessage event")
}

// ─────────────────────────────────────────────
// Close & State
// ─────────────────────────────────────────────

func TestClient_Close_Idempotent(t *testing.T) {
	cfg := ClientConfig{URL: "ws://test", Token: "tok"}
	c, _ := setupConnectedClient(t, cfg)

	assert.NoError(t, c.Close())
	assert.NoError(t, c.Close()) // second close — no panic, no hang
}

func TestClient_InitialState_Disconnected(t *testing.T) {
	cfg := ClientConfig{URL: "ws://test", Token: "tok"}
	c := NewClient(cfg)
	assert.Equal(t, StateDisconnected, c.State())
}

func TestClient_Messages_Channel(t *testing.T) {
	cfg := ClientConfig{URL: "ws://test", Token: "tok"}
	c := NewClient(cfg)
	ch := c.Messages()
	assert.NotNil(t, ch)
}

// ─────────────────────────────────────────────
// History request
// ─────────────────────────────────────────────

func TestClient_HistoryRequest_Sent(t *testing.T) {
	mock := &MockWebSocketConn{}
	mock.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock.EnqueueRead(websocket.TextMessage, authHelloOK(t))
	mock.ReadErr = ErrConnectionClosed

	cfg := ClientConfig{URL: "ws://test", Token: "tok", Session: "main", HistoryLimit: 50}
	c := newTestClient(t, cfg, mock)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, c.Connect(ctx))
	time.Sleep(50 * time.Millisecond)

	sent := mock.SentMessages()
	// Should have: connect req + history req
	require.GreaterOrEqual(t, len(sent), 2)

	var historyFound bool
	for _, msg := range sent {
		var frame OutboundFrame
		if json.Unmarshal(msg, &frame) == nil && frame.Method == "chat.history" {
			historyFound = true
		}
	}
	assert.True(t, historyFound, "expected chat.history request in sent messages")

	require.NoError(t, c.Close())
}

// ─────────────────────────────────────────────
// Reconnect loop — queue drain
// ─────────────────────────────────────────────

func TestClient_Reconnect_QueueDrained(t *testing.T) {
	var callCount atomic.Int32

	mock1 := &MockWebSocketConn{}
	mock1.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock1.EnqueueRead(websocket.TextMessage, authHelloOK(t))
	mock1.ReadErr = ErrConnectionClosed

	mock2 := &MockWebSocketConn{}
	mock2.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock2.EnqueueRead(websocket.TextMessage, authHelloOK(t))
	mock2.ReadErr = ErrConnectionClosed

	fastPolicy := ReconnectPolicy{
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		MaxAttempts:  3,
		Multiplier:   1.0,
	}
	cfg := ClientConfig{
		URL:             "ws://test",
		Token:           "tok",
		ReconnectPolicy: &fastPolicy,
	}
	c := NewClient(cfg)
	c.dial = func(_ context.Context, _ string) (WebSocketConn, error) {
		n := callCount.Add(1)
		if n == 1 {
			return mock1, nil
		}
		return mock2, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, c.Connect(ctx))

	// Queue a message while connected (it may be in the channel or queue)
	_ = c.Send(OutboundMessage{Type: "queued-msg"})

	// Wait for the reconnect to complete
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if callCount.Load() >= 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	assert.GreaterOrEqual(t, int(callCount.Load()), 2, "expected at least one reconnect attempt")
	require.NoError(t, c.Close())
}

func TestClient_Reconnect_MaxAttemptsExhausted(t *testing.T) {
	failPolicy := ReconnectPolicy{
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		MaxAttempts:  2,
		Multiplier:   1.0,
	}
	cfg := ClientConfig{
		URL:             "ws://test",
		Token:           "tok",
		ReconnectPolicy: &failPolicy,
	}

	mock1 := &MockWebSocketConn{}
	mock1.EnqueueRead(websocket.TextMessage, buildChallengeMsg(t))
	mock1.EnqueueRead(websocket.TextMessage, authHelloOK(t))
	mock1.ReadErr = ErrConnectionClosed

	var callCount atomic.Int32
	c := NewClient(cfg)
	c.dial = func(_ context.Context, _ string) (WebSocketConn, error) {
		n := callCount.Add(1)
		if n == 1 {
			return mock1, nil
		}
		// All reconnect attempts fail with dial error
		return nil, ErrConnectionClosed
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, c.Connect(ctx))

	// Wait for all reconnect attempts to exhaust
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if c.State() == StateDisconnected {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Should have received an error event about exhaustion
	var got []InboundEvent
	timer := time.NewTimer(500 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case evt := <-c.Messages():
			got = append(got, evt)
		case <-timer.C:
			goto done
		}
	}
done:

	var foundExhausted bool
	for _, e := range got {
		if e.Kind == KindError && len(e.Error) > 0 {
			foundExhausted = true
			break
		}
	}
	assert.True(t, foundExhausted, "expected exhaustion error event, got: %v", got)

	require.NoError(t, c.Close())
}

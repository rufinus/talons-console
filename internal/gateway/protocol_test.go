package gateway

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────
// ParseInbound — table-driven tests
// ─────────────────────────────────────────────

func TestParseInbound(t *testing.T) {
	okTrue := true
	okFalse := false

	tests := []struct {
		name     string
		input    []byte
		wantKind InboundKind
		check    func(t *testing.T, evt InboundEvent)
	}{
		// ── connect.challenge ──────────────────────
		{
			name: "connect.challenge event",
			input: mustJSON(t, InboundFrame{
				Type:    "event",
				Event:   "connect.challenge",
				Payload: mustJSON(t, ConnectChallengePayload{Nonce: "abc123", TS: 12345}),
			}),
			wantKind: KindChallenge,
			check: func(t *testing.T, evt InboundEvent) {
				assert.Equal(t, "abc123", evt.Nonce)
			},
		},

		// ── hello-ok (auth result success) ─────────
		{
			name: "hello-ok response",
			input: mustJSON(t, InboundFrame{
				Type: "res",
				ID:   "connect-1",
				OK:   &okTrue,
				Payload: mustJSON(t, HelloOKPayload{
					Type:     "hello-ok",
					Protocol: 3,
					Server:   HelloServerInfo{Version: "2.5.0", ConnID: "conn-1"},
					Features: HelloFeatures{
						Methods: []string{"chat.send"},
						Events:  []string{"chat"},
					},
				}),
			}),
			wantKind: KindAuthResult,
			check: func(t *testing.T, evt InboundEvent) {
				assert.True(t, evt.Success)
				assert.Equal(t, "2.5.0", evt.Version)
				assert.Contains(t, evt.Features, "chat.send")
				assert.Contains(t, evt.Features, "chat")
			},
		},

		// ── error response ──────────────────────────
		{
			name: "error res frame",
			input: mustJSON(t, map[string]any{
				"type": "res",
				"id":   "connect-1",
				"ok":   &okFalse,
				"error": map[string]string{
					"code":    "NOT_LINKED",
					"message": "device not linked",
				},
			}),
			wantKind: KindError,
			check: func(t *testing.T, evt InboundEvent) {
				assert.Contains(t, evt.Error, "NOT_LINKED")
				assert.Contains(t, evt.Error, "device not linked")
			},
		},

		// ── chat.event delta ──────────────────────────
		{
			name: "chat.event delta (streaming token)",
			input: mustJSON(t, InboundFrame{
				Type:  "event",
				Event: "chat",
				Payload: mustJSON(t, ChatEventPayload{
					RunID:      "run-1",
					SessionKey: "main",
					State:      "delta",
					Message:    &ChatEventMsg{Role: "assistant", Content: "Hello"},
				}),
			}),
			wantKind: KindToken,
			check: func(t *testing.T, evt InboundEvent) {
				assert.Equal(t, "Hello", evt.Content)
				assert.Equal(t, "assistant", evt.Role)
				assert.Equal(t, "delta", evt.State)
			},
		},

		// ── chat.event final ─────────────────────────
		{
			name: "chat.event final (complete message)",
			input: mustJSON(t, InboundFrame{
				Type:  "event",
				Event: "chat",
				Payload: mustJSON(t, ChatEventPayload{
					RunID:      "run-1",
					SessionKey: "main",
					State:      "final",
					Message:    &ChatEventMsg{Role: "assistant", Content: "Hello world"},
				}),
			}),
			wantKind: KindMessage,
			check: func(t *testing.T, evt InboundEvent) {
				assert.Equal(t, "Hello world", evt.Content)
				assert.Equal(t, "assistant", evt.Role)
				assert.Equal(t, "final", evt.State)
			},
		},

		// ── chat.event aborted ─────────────────────────
		{
			name: "chat.event aborted",
			input: mustJSON(t, InboundFrame{
				Type:  "event",
				Event: "chat",
				Payload: mustJSON(t, ChatEventPayload{
					State:    "aborted",
					ErrorMsg: "request cancelled",
				}),
			}),
			wantKind: KindError,
			check: func(t *testing.T, evt InboundEvent) {
				assert.Contains(t, evt.Error, "request cancelled")
				assert.Equal(t, "aborted", evt.State)
			},
		},

		// ── chat.event error ─────────────────────────
		{
			name: "chat.event error state",
			input: mustJSON(t, InboundFrame{
				Type:  "event",
				Event: "chat",
				Payload: mustJSON(t, ChatEventPayload{
					State:    "error",
					ErrorMsg: "agent timeout",
				}),
			}),
			wantKind: KindError,
			check: func(t *testing.T, evt InboundEvent) {
				assert.Contains(t, evt.Error, "agent timeout")
			},
		},

		// ── agent.event (tool call) ──────────────────
		{
			name: "agent.event (tool call)",
			input: mustJSON(t, InboundFrame{
				Type:  "event",
				Event: "agent",
				Payload: mustJSON(t, AgentEventPayload{
					RunID:  "run-1",
					Stream: "read",
					Data:   json.RawMessage(`{"path":"/tmp/file.txt"}`),
				}),
			}),
			wantKind: KindToolCall,
			check: func(t *testing.T, evt InboundEvent) {
				assert.Equal(t, "read", evt.ToolName)
				assert.Contains(t, evt.ToolArgs, "path")
			},
		},

		// ── chat.history response ────────────────────
		{
			name: "chat.history response",
			input: mustJSON(t, map[string]any{
				"type": "res",
				"id":   "history-1",
				"ok":   true,
				"payload": HistoryPayload{
					Messages: []HistoryMessage{
						{Role: "user", Content: "Hello", Timestamp: 1000},
						{Role: "assistant", Content: "Hi", Timestamp: 1001},
					},
				},
			}),
			wantKind: KindHistory,
			check: func(t *testing.T, evt InboundEvent) {
				require.Len(t, evt.HistoryMessages, 2)
				assert.Equal(t, "user", evt.HistoryMessages[0].Role)
				assert.Equal(t, "Hello", evt.HistoryMessages[0].Content)
				assert.Equal(t, "assistant", evt.HistoryMessages[1].Role)
			},
		},

		// ── unknown event type ───────────────────────
		{
			name:     "unknown event type",
			input:    []byte(`{"type":"event","event":"system.tick","payload":{}}`),
			wantKind: KindUnknown,
			check: func(t *testing.T, evt InboundEvent) {
				assert.NotEmpty(t, evt.Raw)
			},
		},

		// ── unknown frame type ───────────────────────
		{
			name:     "unknown frame type",
			input:    []byte(`{"type":"push","payload":{}}`),
			wantKind: KindUnknown,
			check: func(t *testing.T, evt InboundEvent) {
				assert.NotEmpty(t, evt.Raw)
			},
		},

		// ── completely malformed JSON ─────────────────
		{
			name:     "malformed JSON does not panic",
			input:    []byte(`{not json at all`),
			wantKind: KindUnknown,
			check: func(t *testing.T, evt InboundEvent) {
				assert.NotEmpty(t, evt.Raw)
			},
		},

		// ── empty input ──────────────────────────────
		{
			name:     "empty input",
			input:    []byte{},
			wantKind: KindUnknown,
		},

		// ── null JSON ────────────────────────────────
		{
			name:     "null JSON",
			input:    []byte(`null`),
			wantKind: KindUnknown,
		},

		// ── chat.event delta with empty content ──────
		{
			name: "chat.event delta empty content",
			input: mustJSON(t, InboundFrame{
				Type:  "event",
				Event: "chat",
				Payload: mustJSON(t, ChatEventPayload{
					State:   "delta",
					Message: &ChatEventMsg{Role: "assistant", Content: ""},
				}),
			}),
			wantKind: KindToken,
			check: func(t *testing.T, evt InboundEvent) {
				assert.Equal(t, "", evt.Content)
			},
		},

		// ── chat.event delta with no message field ──
		{
			name: "chat.event delta no message",
			input: mustJSON(t, InboundFrame{
				Type:    "event",
				Event:   "chat",
				Payload: mustJSON(t, ChatEventPayload{State: "delta"}),
			}),
			wantKind: KindToken,
			check: func(t *testing.T, evt InboundEvent) {
				assert.Equal(t, "", evt.Content)
				assert.Equal(t, "", evt.Role)
			},
		},

		// ── hello-ok with extra unknown fields ────────
		{
			name: "hello-ok with extra unknown fields preserved",
			input: []byte(`{
				"type": "res",
				"id":   "connect-1",
				"ok":   true,
				"payload": {
					"type": "hello-ok",
					"protocol": 3,
					"server": {"version": "1.0.0", "connId": "x"},
					"features": {"methods":[], "events":[]},
					"newFieldFromFuture": "should not crash"
				}
			}`),
			wantKind: KindAuthResult,
			check: func(t *testing.T, evt InboundEvent) {
				assert.True(t, evt.Success)
			},
		},

		// ── res frame with ok=true but no hello-ok payload ──
		{
			name: "res frame ok=true no hello-ok",
			input: mustJSON(t, map[string]any{
				"type":    "res",
				"id":      "chat-send-1",
				"ok":      true,
				"payload": map[string]any{"ack": true},
			}),
			wantKind: KindUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// ParseInbound must never panic
			evt := ParseInbound(tc.input)
			assert.Equal(t, tc.wantKind, evt.Kind,
				"expected kind %v, got %v (raw: %s)", tc.wantKind, evt.Kind, string(tc.input))
			if tc.check != nil {
				tc.check(t, evt)
			}
		})
	}
}

// TestParseInbound_NoPanic verifies the parser never panics on arbitrary input.
func TestParseInbound_NoPanic(t *testing.T) {
	fuzzInputs := [][]byte{
		nil,
		{},
		[]byte("null"),
		[]byte("[]"),
		[]byte("{}"),
		[]byte(`"string"`),
		[]byte("42"),
		[]byte(`{"type":null}`),
		[]byte(`{"type":{}}`),
		[]byte(`{"type":"event","event":null}`),
		[]byte(`{"type":"res","ok":"not-a-bool"}`),
	}
	for _, input := range fuzzInputs {
		t.Run(string(input), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParseInbound panicked on input %q: %v", input, r)
				}
			}()
			_ = ParseInbound(input)
		})
	}
}

// ─────────────────────────────────────────────
// MockWebSocketConn tests
// ─────────────────────────────────────────────

func TestMockWebSocketConn_ReadWrite(t *testing.T) {
	m := &MockWebSocketConn{}

	// Enqueue two messages
	m.EnqueueRead(1, []byte("hello"))
	m.EnqueueRead(1, []byte("world"))

	msgType, data, err := m.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, 1, msgType)
	assert.Equal(t, []byte("hello"), data)

	msgType, data, err = m.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, 1, msgType)
	assert.Equal(t, []byte("world"), data)

	// Exhausted → returns ErrConnectionClosed
	_, _, err = m.ReadMessage()
	assert.ErrorIs(t, err, ErrConnectionClosed)

	// Write
	require.NoError(t, m.WriteMessage(1, []byte("sent")))
	msgs := m.SentMessages()
	require.Len(t, msgs, 1)
	assert.Equal(t, []byte("sent"), msgs[0])
}

func TestMockWebSocketConn_WriteError(t *testing.T) {
	m := &MockWebSocketConn{WriteErr: ErrConnectionClosed}
	err := m.WriteMessage(1, []byte("data"))
	assert.ErrorIs(t, err, ErrConnectionClosed)
	assert.Empty(t, m.SentMessages())
}

func TestMockWebSocketConn_Close(t *testing.T) {
	m := &MockWebSocketConn{}
	assert.False(t, m.IsClosed())
	require.NoError(t, m.Close())
	assert.True(t, m.IsClosed())
}

func TestMockWebSocketConn_EnqueueReadError(t *testing.T) {
	m := &MockWebSocketConn{}
	m.EnqueueRead(1, []byte("ok"))
	m.EnqueueReadError(ErrConnectionClosed)

	_, _, err := m.ReadMessage()
	require.NoError(t, err)

	_, _, err = m.ReadMessage()
	assert.ErrorIs(t, err, ErrConnectionClosed)
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

// mustJSON serialises v to JSON, failing the test on error.
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return json.RawMessage(data)
}

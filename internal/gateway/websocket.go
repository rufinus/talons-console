package gateway

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ─────────────────────────────────────────────
// Production WebSocket wrapper
// ─────────────────────────────────────────────

// wsConn wraps a *websocket.Conn and implements WebSocketConn.
// It is a thin delegation layer that adds no additional logic.
type wsConn struct {
	conn *websocket.Conn
}

// newWebSocketConn wraps an established gorilla/websocket connection.
func newWebSocketConn(conn *websocket.Conn) WebSocketConn {
	return &wsConn{conn: conn}
}

func (w *wsConn) ReadMessage() (int, []byte, error) {
	return w.conn.ReadMessage()
}

func (w *wsConn) WriteMessage(messageType int, data []byte) error {
	return w.conn.WriteMessage(messageType, data)
}

func (w *wsConn) Close() error {
	return w.conn.Close()
}

func (w *wsConn) SetReadDeadline(t time.Time) error {
	return w.conn.SetReadDeadline(t)
}

func (w *wsConn) SetWriteDeadline(t time.Time) error {
	return w.conn.SetWriteDeadline(t)
}

// ─────────────────────────────────────────────
// Mock WebSocket — for testing
// ─────────────────────────────────────────────

// MockWebSocketConn is a test double for WebSocketConn.
// Feed it messages via EnqueueRead; inspect sent frames via SentMessages().
type MockWebSocketConn struct {
	mu sync.Mutex

	// inbound queue — what ReadMessage returns, in order
	readQueue []readItem
	readIdx   int

	// outbound capture — all frames passed to WriteMessage
	sentMessages [][]byte

	// error controls
	ReadErr  error // returned after the readQueue is exhausted
	WriteErr error // returned for every WriteMessage call when set
	CloseErr error // returned by Close

	// state
	closed bool
}

type readItem struct {
	messageType int
	data        []byte
	err         error
}

// EnqueueRead adds a message to the inbound queue.
func (m *MockWebSocketConn) EnqueueRead(messageType int, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readQueue = append(m.readQueue, readItem{messageType: messageType, data: data})
}

// EnqueueReadError adds an error frame to the inbound queue.
func (m *MockWebSocketConn) EnqueueReadError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readQueue = append(m.readQueue, readItem{err: err})
}

// SentMessages returns a copy of all frames written via WriteMessage.
func (m *MockWebSocketConn) SentMessages() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]byte, len(m.sentMessages))
	copy(out, m.sentMessages)
	return out
}

// IsClosed reports whether Close was called.
func (m *MockWebSocketConn) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *MockWebSocketConn) ReadMessage() (int, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readIdx < len(m.readQueue) {
		item := m.readQueue[m.readIdx]
		m.readIdx++
		return item.messageType, item.data, item.err
	}
	if m.ReadErr != nil {
		return 0, nil, m.ReadErr
	}
	// Default: block-simulating EOF
	return 0, nil, ErrConnectionClosed
}

func (m *MockWebSocketConn) WriteMessage(messageType int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.WriteErr != nil {
		return m.WriteErr
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	m.sentMessages = append(m.sentMessages, cp)
	return nil
}

func (m *MockWebSocketConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return m.CloseErr
}

func (m *MockWebSocketConn) SetReadDeadline(_ time.Time) error  { return nil }
func (m *MockWebSocketConn) SetWriteDeadline(_ time.Time) error { return nil }

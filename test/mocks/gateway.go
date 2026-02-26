// Package mocks provides mock implementations for testing.
package mocks

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/rufinus/talons-console/internal/gateway"
)

// MockGateway is a mock OpenClaw Gateway server for testing.
type MockGateway struct {
	listener net.Listener
	server   *http.Server
	upgrader websocket.Upgrader

	// Configurable behavior
	mu          sync.RWMutex
	authSuccess bool
	features    []string
	version     string

	// Connected clients
	clients   map[string]*mockClient
	clientsMu sync.RWMutex

	// Received messages
	receivedMu       sync.RWMutex
	receivedMessages []gateway.OutboundMessage
	receivedFrames   []gateway.OutboundFrame
}

type mockClient struct {
	id   string
	conn *websocket.Conn
	mu   sync.Mutex
}

// NewMockGateway creates a new mock Gateway server.
func NewMockGateway() *MockGateway {
	return &MockGateway{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients:          make(map[string]*mockClient),
		authSuccess:      true,
		features:         []string{"chat.send", "chat.history", "streaming"},
		version:          "2.5.0-mock",
		receivedMessages: make([]gateway.OutboundMessage, 0),
	}
}

// Start starts the mock Gateway server on a random port.
func (m *MockGateway) Start() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	m.listener = listener

	mux := http.NewServeMux()
	mux.HandleFunc("/", m.handleWebSocket)

	m.server = &http.Server{
		Handler: mux,
	}

	go func() { _ = m.server.Serve(listener) }()
	return nil
}

// Stop shuts down the mock Gateway server.
func (m *MockGateway) Stop() error {
	// Close all client connections
	m.clientsMu.Lock()
	for _, client := range m.clients {
		_ = client.conn.Close()
	}
	m.clients = make(map[string]*mockClient)
	m.clientsMu.Unlock()

	if m.server != nil {
		return m.server.Close()
	}
	return nil
}

// URL returns the WebSocket URL for connecting to this mock server.
func (m *MockGateway) URL() string {
	return "ws://" + m.listener.Addr().String()
}

// SetAuthResult configures the authentication response.
func (m *MockGateway) SetAuthResult(success bool, features []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authSuccess = success
	if features != nil {
		m.features = features
	}
}

// ReceivedMessages returns all messages received from clients.
func (m *MockGateway) ReceivedMessages() []gateway.OutboundMessage {
	m.receivedMu.RLock()
	defer m.receivedMu.RUnlock()
	return append([]gateway.OutboundMessage{}, m.receivedMessages...)
}

// WaitForReceivedCount polls until at least n messages have been received by
// the mock, or the timeout expires. Returns true when the count is reached.
func (m *MockGateway) WaitForReceivedCount(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m.receivedMu.RLock()
		count := len(m.receivedMessages)
		m.receivedMu.RUnlock()
		if count >= n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// findClient looks up a connected client by exact id or by IP prefix (without
// port). This lets tests pass bare IPs like "127.0.0.1" even though clients
// are keyed by the full RemoteAddr ("127.0.0.1:PORT").
func (m *MockGateway) findClient(clientID string) *mockClient {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()
	if c, ok := m.clients[clientID]; ok {
		return c
	}
	// Prefix match: "127.0.0.1" matches "127.0.0.1:PORT"
	prefix := clientID + ":"
	for id, c := range m.clients {
		if strings.HasPrefix(id, prefix) {
			return c
		}
	}
	return nil
}

// SendToken sends a streaming token to a specific client.
func (m *MockGateway) SendToken(clientID string, content string) error {
	client := m.findClient(clientID)
	if client == nil {
		return nil // Client not connected
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	frame := map[string]any{
		"type":  "event",
		"event": "chat.event",
		"payload": map[string]any{
			"state": "delta",
			"message": map[string]any{
				"role":    "assistant",
				"content": content,
			},
		},
	}
	return client.conn.WriteJSON(frame)
}

// SendMessageComplete sends a message completion event to a client.
func (m *MockGateway) SendMessageComplete(clientID string) error {
	client := m.findClient(clientID)
	if client == nil {
		return nil
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	frame := map[string]any{
		"type":  "event",
		"event": "chat.event",
		"payload": map[string]any{
			"state": "final",
			"message": map[string]any{
				"role":    "assistant",
				"content": "",
			},
		},
	}
	return client.conn.WriteJSON(frame)
}

// handleWebSocket handles WebSocket upgrade and client communication.
func (m *MockGateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	clientID := r.RemoteAddr
	client := &mockClient{id: clientID, conn: conn}

	m.clientsMu.Lock()
	m.clients[clientID] = client
	m.clientsMu.Unlock()

	defer func() {
		m.clientsMu.Lock()
		delete(m.clients, clientID)
		m.clientsMu.Unlock()
	}()

	// Send challenge
	challenge := map[string]any{
		"type":  "event",
		"event": "connect.challenge",
		"payload": map[string]any{
			"nonce": "test-nonce",
			"ts":    time.Now().Unix(),
		},
	}
	if err := conn.WriteJSON(challenge); err != nil {
		return
	}

	// Handle messages
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var frame gateway.OutboundFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			continue
		}

		m.handleFrame(client, frame, data)
	}
}

// handleFrame processes a frame from a client.
func (m *MockGateway) handleFrame(client *mockClient, frame gateway.OutboundFrame, raw []byte) {
	switch frame.Method {
	case "connect":
		m.handleConnect(client, frame)
	case "chat.send":
		m.handleChatSend(client, frame, raw)
	case "chat.history":
		m.handleHistory(client, frame)
	}
}

// handleConnect handles the connect/auth request.
func (m *MockGateway) handleConnect(client *mockClient, frame gateway.OutboundFrame) {
	m.mu.RLock()
	success := m.authSuccess
	features := m.features
	version := m.version
	m.mu.RUnlock()

	ok := success
	response := map[string]any{
		"type":   "res",
		"id":     frame.ID,
		"ok":     ok,
		"method": "connect",
	}

	if success {
		response["payload"] = map[string]any{
			"type":     "hello-ok",
			"protocol": 2,
			"server": map[string]any{
				"version": version,
				"connId":  "mock-conn-123",
			},
			"features": map[string]any{
				"methods": features,
				"events":  []string{"chat.event", "agent.event"},
			},
		}
	} else {
		response["error"] = map[string]any{
			"code":    "auth_failed",
			"message": "Authentication failed",
		}
	}

	client.mu.Lock()
	_ = client.conn.WriteJSON(response)
	client.mu.Unlock()
}

// handleChatSend handles a chat message from the client.
func (m *MockGateway) handleChatSend(client *mockClient, frame gateway.OutboundFrame, raw []byte) {
	// Store the message
	var msg gateway.OutboundMessage
	if err := json.Unmarshal(raw, &msg); err == nil {
		m.receivedMu.Lock()
		m.receivedMessages = append(m.receivedMessages, msg)
		m.receivedMu.Unlock()
	}

	// Send acknowledgment
	response := map[string]any{
		"type":   "res",
		"id":     frame.ID,
		"ok":     true,
		"method": "chat.send",
	}

	client.mu.Lock()
	_ = client.conn.WriteJSON(response)
	client.mu.Unlock()
}

// handleHistory handles a history request.
func (m *MockGateway) handleHistory(client *mockClient, frame gateway.OutboundFrame) {
	response := map[string]any{
		"type":   "res",
		"id":     frame.ID,
		"ok":     true,
		"method": "chat.history",
		"payload": map[string]any{
			"messages": []map[string]any{
				{
					"role":      "user",
					"content":   "Previous message",
					"timestamp": time.Now().Add(-time.Hour).Unix(),
				},
				{
					"role":      "assistant",
					"content":   "Previous response",
					"timestamp": time.Now().Add(-time.Hour + time.Minute).Unix(),
				},
			},
		},
	}

	client.mu.Lock()
	_ = client.conn.WriteJSON(response)
	client.mu.Unlock()
}

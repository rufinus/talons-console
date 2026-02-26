package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// dialFunc is the signature for a function that dials a WebSocket connection.
// Injected in tests to avoid real network I/O.
type dialFunc func(ctx context.Context, url string) (WebSocketConn, error)

// Client implements GatewayClient over a real WebSocket connection.
// It manages the read loop, write loop, message queue, and automatic reconnection.
type Client struct {
	cfg  ClientConfig
	dial dialFunc // injectable for testing

	// Connection state
	state  atomic.Int32 // stores ConnectionState
	conn   WebSocketConn
	connMu sync.RWMutex

	// Channels
	outbound chan OutboundMessage // TUI → write loop
	inbound  chan InboundEvent    // read loop → TUI
	quit     chan struct{}        // closed to signal shutdown

	// Queue for offline messages
	queue *Queue

	// Reconnection
	reconnect ReconnectPolicy

	// Lifecycle
	wg sync.WaitGroup
}

// ClientConfig holds all configuration for a Gateway Client.
type ClientConfig struct {
	URL      string
	Token    string
	Password string
	Agent    string
	Session  string

	// Client identity presented during the connect handshake
	ClientID      string
	ClientVersion string
	Platform      string

	// Maximum number of history messages to request after connect
	HistoryLimit int

	// Reconnect policy (zero value uses DefaultReconnectPolicy)
	ReconnectPolicy *ReconnectPolicy
}

// NewClient creates a new Client with the given configuration.
// If ClientID or Platform are empty, sensible defaults are applied.
func NewClient(cfg ClientConfig) *Client {
	rp := DefaultReconnectPolicy()
	if cfg.ReconnectPolicy != nil {
		rp = *cfg.ReconnectPolicy
	}
	if cfg.ClientID == "" {
		cfg.ClientID = "cli"
	}
	if cfg.Platform == "" {
		cfg.Platform = "linux"
	}

	c := &Client{
		cfg:       cfg,
		outbound:  make(chan OutboundMessage, 64),
		inbound:   make(chan InboundEvent, 256),
		quit:      make(chan struct{}),
		queue:     NewQueue(100),
		reconnect: rp,
	}
	c.dial = c.defaultDial
	return c
}

// Connect implements GatewayClient. It establishes the WebSocket connection,
// performs the auth challenge/response handshake, and starts the background
// read/write goroutines.
func (c *Client) Connect(ctx context.Context) error {
	c.setState(StateConnecting)
	conn, err := c.dial(ctx, c.cfg.URL)
	if err != nil {
		c.setState(StateDisconnected)
		return fmt.Errorf("dial: %w", err)
	}
	c.setConn(conn)

	// Perform auth handshake via auth.go
	c.setState(StateAuthenticating)
	result, err := Authenticate(ctx, conn, AuthConfig{
		Token:    c.cfg.Token,
		Password: c.cfg.Password,
	})
	if err != nil {
		_ = conn.Close()
		c.setState(StateDisconnected)
		return err
	}

	c.setState(StateConnected)
	c.emitEvent(result)
	c.startLoops(conn)

	// Request history after connect
	if c.cfg.HistoryLimit > 0 {
		_ = c.sendHistoryRequest(c.cfg.Session, conn)
	}

	return nil
}

// Send implements GatewayClient. If disconnected, the message is queued.
func (c *Client) Send(msg OutboundMessage) error {
	select {
	case <-c.quit:
		return ErrShutdown
	default:
	}

	if c.State() != StateConnected {
		dropped := c.queue.Enqueue(msg)
		if dropped {
			return ErrQueueFull
		}
		return nil
	}

	select {
	case c.outbound <- msg:
		return nil
	case <-c.quit:
		return ErrShutdown
	}
}

// Messages implements GatewayClient.
func (c *Client) Messages() <-chan InboundEvent {
	return c.inbound
}

// Close implements GatewayClient. It shuts down all goroutines and closes
// the WebSocket connection.
//
// The connection is closed *before* wg.Wait so that the readLoop unblocks
// from its blocking conn.ReadMessage() call and can exit cleanly.
func (c *Client) Close() error {
	select {
	case <-c.quit:
		return nil // already closed
	default:
		close(c.quit)
	}

	// Close the underlying connection first so readLoop unblocks from
	// its blocking ReadMessage() call; otherwise wg.Wait() would deadlock.
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn != nil {
		_ = conn.Close()
	}

	c.wg.Wait()
	return nil
}

// State implements GatewayClient.
func (c *Client) State() ConnectionState {
	return ConnectionState(c.state.Load())
}

// RequestHistory is the public version of sendHistoryRequest.
// It sends a chat.history request for the given session key using the current
// active connection.
func (c *Client) RequestHistory(sessionKey string) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		return ErrShutdown
	}
	return c.sendHistoryRequest(sessionKey, conn)
}

// Reconnect performs a single close + 100ms sleep + reconnect attempt.
// The caller is responsible for enforcing an overall timeout via ctx.
// This method does NOT implement retry/backoff — that is the automatic
// reconnect loop's responsibility.
func (c *Client) Reconnect(ctx context.Context) error {
	if err := c.Close(); err != nil {
		return fmt.Errorf("close before reconnect: %w", err)
	}
	// Re-initialise the quit channel and inbound/outbound channels so the
	// client can be used again after Close().
	c.quit = make(chan struct{})
	c.outbound = make(chan OutboundMessage, 64)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	return c.Connect(ctx)
}

// ─────────────────────────────────────────────
// Internal — dial
// ─────────────────────────────────────────────

// defaultDial is the production dial implementation.
func (c *Client) defaultDial(ctx context.Context, url string) (WebSocketConn, error) {
	dialer := &websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	header := http.Header{
		"User-Agent": []string{fmt.Sprintf("talons/%s", c.cfg.ClientVersion)},
	}
	conn, _, err := dialer.DialContext(ctx, url, header)
	if err != nil {
		return nil, err
	}
	return newWebSocketConn(conn), nil
}

// ─────────────────────────────────────────────
// Internal — history request
// ─────────────────────────────────────────────

func (c *Client) sendHistoryRequest(sessionKey string, conn WebSocketConn) error {
	params := HistoryParams{
		SessionKey: sessionKey,
		Limit:      c.cfg.HistoryLimit,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshalling history params: %w", err)
	}
	frame := OutboundFrame{
		Type:   "req",
		ID:     fmt.Sprintf("history-%d", time.Now().UnixMilli()),
		Method: "chat.history",
		Params: json.RawMessage(paramsJSON),
	}
	frameJSON, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("marshalling history frame: %w", err)
	}
	return conn.WriteMessage(websocket.TextMessage, frameJSON)
}

// ─────────────────────────────────────────────
// Internal — goroutine loops
// ─────────────────────────────────────────────

func (c *Client) startLoops(conn WebSocketConn) {
	c.wg.Add(2)
	go c.readLoop(conn)
	go c.writeLoop(conn)
}

// readLoop reads frames from the WebSocket and dispatches InboundEvents.
func (c *Client) readLoop(conn WebSocketConn) {
	defer c.wg.Done()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-c.quit:
				return
			default:
			}
			c.emitEvent(InboundEvent{
				Kind:  KindError,
				Error: fmt.Sprintf("connection lost: %v", err),
			})
			c.setState(StateReconnecting)
			c.wg.Add(1)
			go c.reconnectLoop()
			return
		}

		evt := ParseInbound(data)
		c.emitEvent(evt)
	}
}

// writeLoop serialises outbound messages to the WebSocket.
// All writes are serialised through this loop (gorilla/websocket is not
// concurrent-write-safe).
func (c *Client) writeLoop(conn WebSocketConn) {
	defer c.wg.Done()
	for {
		select {
		case <-c.quit:
			return
		case msg := <-c.outbound:
			if err := c.writeMsg(conn, msg); err != nil {
				select {
				case <-c.quit:
					return
				default:
				}
				c.emitEvent(InboundEvent{
					Kind:  KindError,
					Error: fmt.Sprintf("write error: %v", err),
				})
			}
		}
	}
}

func (c *Client) writeMsg(conn WebSocketConn, msg OutboundMessage) error {
	// Convert the high-level OutboundMessage to a wire-format OutboundFrame.
	// msg.Type maps to the "method" field; msg.Payload becomes "params".
	// This mirrors sendHistoryRequest and keeps the protocol consistent.
	paramsJSON, err := json.Marshal(msg.Payload)
	if err != nil {
		return fmt.Errorf("marshalling params: %w", err)
	}
	frame := OutboundFrame{
		Type:   "req",
		ID:     fmt.Sprintf("msg-%d", time.Now().UnixMilli()),
		Method: msg.Type,
		Params: json.RawMessage(paramsJSON),
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("marshalling frame: %w", err)
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

// ─────────────────────────────────────────────
// Internal — reconnect loop
// ─────────────────────────────────────────────

func (c *Client) reconnectLoop() {
	defer c.wg.Done()

	policy := c.reconnect
	delay := policy.InitialDelay

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		select {
		case <-c.quit:
			return
		case <-time.After(delay):
		}

		c.emitEvent(InboundEvent{
			Kind:  KindSessionInfo,
			Agent: fmt.Sprintf("Reconnecting (attempt %d/%d)…", attempt, policy.MaxAttempts),
		})

		ctx, dialCancel := context.WithTimeout(context.Background(), 30*time.Second)
		conn, dialErr := c.dial(ctx, c.cfg.URL)
		dialCancel()
		if dialErr != nil {
			delay = policy.next(delay)
			continue
		}

		authCtx, authCancel := context.WithTimeout(context.Background(), 15*time.Second)
		_, authErr := Authenticate(authCtx, conn, AuthConfig{
			Token:    c.cfg.Token,
			Password: c.cfg.Password,
		})
		authCancel()
		if authErr != nil {
			_ = conn.Close()
			delay = policy.next(delay)
			continue
		}

		c.setConn(conn)
		c.setState(StateConnected)

		// Flush queued messages
		for _, queued := range c.queue.Drain() {
			select {
			case c.outbound <- queued:
			case <-c.quit:
				return
			}
		}

		c.startLoops(conn)
		return
	}

	// All attempts exhausted
	c.setState(StateDisconnected)
	c.emitEvent(InboundEvent{
		Kind:  KindError,
		Error: "reconnection failed after maximum attempts — press 'r' to retry or 'q' to quit",
	})
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func (c *Client) setState(s ConnectionState) {
	c.state.Store(int32(s))
}

func (c *Client) setConn(conn WebSocketConn) {
	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()
}

func (c *Client) emitEvent(evt InboundEvent) {
	select {
	case c.inbound <- evt:
	case <-c.quit:
	}
}

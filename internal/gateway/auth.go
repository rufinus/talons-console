package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"

	"github.com/rufinus/talons-console/internal/version"
)

const defaultAuthTimeout = 30 * time.Second

// AuthConfig holds authentication credentials.
type AuthConfig struct {
	Token    string
	Password string
}

// Authenticate performs the OpenClaw Gateway auth handshake on conn.
// Flow: wait for connect.challenge → send connect request → receive hello-ok/fail.
// Returns the InboundEvent (KindAuthResult) on success.
func Authenticate(ctx context.Context, conn WebSocketConn, auth AuthConfig) (InboundEvent, error) {
	// Apply default timeout if context has no deadline.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultAuthTimeout)
		defer cancel()
	}

	// 1. Wait for connect.challenge.
	challenge, err := readEvent(ctx, conn)
	if err != nil {
		return InboundEvent{}, err
	}
	if challenge.Kind != KindChallenge {
		return InboundEvent{}, fmt.Errorf("expected connect.challenge, got kind %d", challenge.Kind)
	}

	// 2. Send connect request.
	if err := sendConnectRequest(conn, auth); err != nil {
		return InboundEvent{}, err
	}

	// 3. Wait for hello-ok (or failure).
	resp, err := readEvent(ctx, conn)
	if err != nil {
		return InboundEvent{}, err
	}

	switch resp.Kind {
	case KindAuthResult:
		if !resp.Success {
			return InboundEvent{}, ErrAuthFailed
		}
		return resp, nil
	case KindError:
		return InboundEvent{}, fmt.Errorf("%w: %s", ErrAuthFailed, resp.Error)
	default:
		return InboundEvent{}, fmt.Errorf("unexpected response during auth: kind %d", resp.Kind)
	}
}

// readResult is an internal type used to pass ReadMessage results through a channel,
// avoiding variable capture bugs in goroutine closures.
type readResult struct {
	evt InboundEvent
	err error
}

// readEvent reads one message from conn, respecting the context deadline.
// The goroutine communicates its result via a channel to avoid closure capture issues.
func readEvent(ctx context.Context, conn WebSocketConn) (InboundEvent, error) {
	resultCh := make(chan readResult, 1)

	go func() {
		_, data, err := conn.ReadMessage()
		if err != nil {
			resultCh <- readResult{err: err}
			return
		}
		resultCh <- readResult{evt: ParseInbound(data)}
	}()

	select {
	case <-ctx.Done():
		return InboundEvent{}, ErrAuthTimeout
	case r := <-resultCh:
		if r.err != nil {
			return InboundEvent{}, r.err
		}
		return r.evt, nil
	}
}

// sendConnectRequest sends the initial connect frame to the Gateway.
func sendConnectRequest(conn WebSocketConn, auth AuthConfig) error {
	params := ConnectParams{
		MinProtocol: 3,
		MaxProtocol: 3,
		Client: ClientInfo{
			ID:       "cli",
			Version:  version.Version,
			Platform: "cli",
			Mode:     "cli",
		},
		Role:      "operator",
		Scopes:    []string{"operator.admin", "operator.read", "operator.write", "operator.approvals", "operator.pairing"},
		Auth:      AuthCredentials(auth),
		UserAgent: fmt.Sprintf("talons/%s", version.Version),
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return err
	}

	frame := OutboundFrame{
		Type:   "req",
		ID:     "connect-1",
		Method: "connect",
		Params: paramsJSON,
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}

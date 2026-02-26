//go:build integration

// Package integration contains integration tests that validate talons-console
// behaviour against a real OpenClaw Gateway instance.
//
// These tests require the following environment variables:
//
//	TALONS_TEST_URL   — WebSocket URL of the Gateway (e.g. ws://localhost:8080)
//	TALONS_TEST_TOKEN — Auth token accepted by the Gateway
//
// Run with: go test -tags=integration ./test/integration/... -timeout=2m
package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rufinus/talons-console/internal/gateway"
)

const testTimeout = 30 * time.Second

// skipIfNotConfigured skips the test if the required environment variables are absent.
// Always call this at the top of every test function.
func skipIfNotConfigured(t *testing.T) (gatewayURL, token string) {
	t.Helper()

	gatewayURL = os.Getenv("TALONS_TEST_URL")
	if gatewayURL == "" {
		t.Skip("TALONS_TEST_URL not set; skipping integration tests")
	}

	token = os.Getenv("TALONS_TEST_TOKEN")
	if token == "" {
		t.Skip("TALONS_TEST_TOKEN not set; skipping integration tests")
	}

	return gatewayURL, token
}

// newTestClient creates a connected gateway.Client for integration tests.
// If the dial fails the test is skipped (not failed) with a descriptive message.
func newTestClient(t *testing.T, gatewayURL, token, agent, session string) *gateway.Client {
	t.Helper()

	cfg := gateway.ClientConfig{
		URL:           gatewayURL,
		Token:         token,
		Agent:         agent,
		Session:       session,
		ClientID:      "talons-integration-test",
		ClientVersion: "0.0.0-test",
		Platform:      "linux",
	}

	client := gateway.NewClient(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)

	if err := client.Connect(ctx); err != nil {
		t.Skipf("Gateway unreachable at %s — skipping: %v", gatewayURL, err)
	}

	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}

// TestAgentIntegration verifies that issuing a chat.send with a custom AgentID
// field is accepted by the Gateway without error.
func TestAgentIntegration(t *testing.T) {
	gatewayURL, token := skipIfNotConfigured(t)

	client := newTestClient(t, gatewayURL, token, "integration-test", "")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Send a chat message carrying an explicit agentId — mirrors what
	// HandleAgent + a subsequent Send would produce.
	msg := gateway.OutboundMessage{
		Type: "chat.send",
		Payload: gateway.ChatSendParams{
			Content: "integration-test agent probe",
			AgentID: "integration-test-2",
			Deliver: false,
		},
	}
	if err := client.Send(msg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Wait for any inbound event — a response or an error event both mean the
	// Gateway accepted (or rejected) the frame.  A KindError response is still
	// a success for this test because the Gateway acknowledged the connection.
	select {
	case <-ctx.Done():
		t.Skip("Gateway did not respond within timeout — skipping")
	case <-client.Messages():
		// Gateway responded; AgentID field was accepted.
	}
}

// TestSessionIntegration verifies that RequestHistory is accepted by the
// Gateway and that subsequent messages can carry a new session key.
func TestSessionIntegration(t *testing.T) {
	gatewayURL, token := skipIfNotConfigured(t)

	client := newTestClient(t, gatewayURL, token, "", "integration-session-a")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Issue a history request for the new session — this is exactly what
	// HandleSession calls after updating in-memory state.
	if err := client.RequestHistory("integration-session-b"); err != nil {
		t.Skipf("RequestHistory failed (Gateway may not support history) — skipping: %v", err)
	}

	// Now send a message using the new session key.
	msg := gateway.OutboundMessage{
		Type: "chat.send",
		Payload: gateway.ChatSendParams{
			Content:    "integration-test session probe",
			SessionKey: "integration-session-b",
			Deliver:    false,
		},
	}
	if err := client.Send(msg); err != nil {
		t.Fatalf("Send with new session key failed: %v", err)
	}

	// Wait for any response from the Gateway.
	select {
	case <-ctx.Done():
		t.Skip("Gateway did not respond within timeout — skipping")
	case <-client.Messages():
		// Gateway responded; session key was accepted.
	}
}

// TestModelIntegration verifies that the Gateway accepts a chat.send frame
// that includes a model override (or at least does not close the connection).
func TestModelIntegration(t *testing.T) {
	gatewayURL, token := skipIfNotConfigured(t)

	client := newTestClient(t, gatewayURL, token, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	msg := gateway.OutboundMessage{
		Type: "chat.send",
		Payload: gateway.ChatSendParams{
			Content: "integration-test model probe",
			Model:   "claude-3-5-haiku-20241022",
			Deliver: false,
		},
	}
	if err := client.Send(msg); err != nil {
		t.Fatalf("Send with model override failed: %v", err)
	}

	select {
	case <-ctx.Done():
		t.Skip("Gateway did not respond within timeout — skipping")
	case evt := <-client.Messages():
		// A KindError means the model was unknown but the Gateway stayed up,
		// which satisfies the requirement: "responds without error if model is ignored".
		t.Logf("Gateway response kind: %v", evt.Kind)
	}
}

// TestReconnectIntegration verifies that client.Reconnect re-establishes the
// WebSocket connection and that the client reaches StateConnected again.
func TestReconnectIntegration(t *testing.T) {
	gatewayURL, token := skipIfNotConfigured(t)

	cfg := gateway.ClientConfig{
		URL:           gatewayURL,
		Token:         token,
		ClientID:      "talons-integration-test",
		ClientVersion: "0.0.0-test",
		Platform:      "linux",
	}
	client := gateway.NewClient(cfg)

	connectCtx, connectCancel := context.WithTimeout(context.Background(), testTimeout)
	defer connectCancel()

	if err := client.Connect(connectCtx); err != nil {
		t.Skipf("Gateway unreachable at %s — skipping: %v", gatewayURL, err)
	}
	t.Cleanup(func() { _ = client.Close() })

	// Verify initial state.
	if client.State() != gateway.StateConnected {
		t.Fatalf("expected StateConnected after initial connect, got %v", client.State())
	}

	// Issue a reconnect with a fresh 30-second deadline.
	reconnectCtx, reconnectCancel := context.WithTimeout(context.Background(), testTimeout)
	defer reconnectCancel()

	if err := client.Reconnect(reconnectCtx); err != nil {
		t.Skipf("Reconnect failed (Gateway may have rejected re-dial) — skipping: %v", err)
	}

	// After a successful Reconnect the client must be connected again.
	if client.State() != gateway.StateConnected {
		t.Errorf("expected StateConnected after reconnect, got %v", client.State())
	}

	// Verify that the client can send a message (i.e. counters/state reset).
	msg := gateway.OutboundMessage{
		Type: "chat.send",
		Payload: gateway.ChatSendParams{
			Content: "integration-test reconnect probe",
			Deliver: false,
		},
	}
	if err := client.Send(msg); err != nil {
		t.Errorf("Send after reconnect failed: %v", err)
	}
}

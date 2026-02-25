// Package e2e provides end-to-end integration tests.
package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rufinus/talons-console/internal/gateway"
	"github.com/rufinus/talons-console/test/helpers"
	"github.com/rufinus/talons-console/test/mocks"
)

func TestConnectAndAuth(t *testing.T) {
	mockGW := mocks.NewMockGateway()
	require.NoError(t, mockGW.Start())
	defer mockGW.Stop()

	t.Run("successful token auth", func(t *testing.T) {
		client := gateway.NewClient(gateway.ClientConfig{
			URL:     mockGW.URL(),
			Token:   "valid-token",
			Agent:   "test-agent",
			Session: "test-session",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.Connect(ctx)
		require.NoError(t, err)
		assert.Equal(t, gateway.StateConnected, client.State())

		client.Close()
	})

	t.Run("failed auth", func(t *testing.T) {
		mockGW.SetAuthResult(false, nil)
		defer mockGW.SetAuthResult(true, nil) // Reset

		client := gateway.NewClient(gateway.ClientConfig{
			URL:     mockGW.URL(),
			Token:   "invalid-token",
			Agent:   "test-agent",
			Session: "test-session",
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.Connect(ctx)
		assert.Error(t, err)
		client.Close()
	})
}

func TestSendAndReceive(t *testing.T) {
	mockGW := mocks.NewMockGateway()
	require.NoError(t, mockGW.Start())
	defer mockGW.Stop()

	client := gateway.NewClient(gateway.ClientConfig{
		URL:     mockGW.URL(),
		Token:   "valid-token",
		Agent:   "test-agent",
		Session: "test-session",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, client.Connect(ctx))
	defer client.Close()

	t.Run("send message receives tokens", func(t *testing.T) {
		// Send a message
		err := client.Send(gateway.OutboundMessage{
			Type: "chat.send",
			Payload: gateway.ChatSendParams{
				Content:    "Hello",
				SessionKey: "test-session",
				AgentID:    "test-agent",
			},
		})
		require.NoError(t, err)

		// Wait for the mock to receive the message (Send is async via writeLoop).
		require.True(t, mockGW.WaitForReceivedCount(1, 2*time.Second),
			"timed out waiting for mock to receive the sent message")
		received := mockGW.ReceivedMessages()
		require.Len(t, received, 1)

		// Simulate streaming response
		mockGW.SendToken("127.0.0.1", "Hello")
		mockGW.SendToken("127.0.0.1", " there")
		mockGW.SendMessageComplete("127.0.0.1")

		// Wait for tokens
		var tokens []string
		done := time.After(2 * time.Second)
		for len(tokens) < 2 {
			select {
			case event := <-client.Messages():
				if event.Kind == gateway.KindToken {
					tokens = append(tokens, event.Content)
				}
			case <-done:
				t.Fatal("timeout waiting for tokens")
			}
		}

		assert.Equal(t, []string{"Hello", " there"}, tokens)
	})
}

func TestReconnection(t *testing.T) {
	mockGW := mocks.NewMockGateway()
	require.NoError(t, mockGW.Start())

	client := gateway.NewClient(gateway.ClientConfig{
		URL:     mockGW.URL(),
		Token:   "valid-token",
		Agent:   "test-agent",
		Session: "test-session",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, client.Connect(ctx))

	// Simulate disconnect by stopping the server
	mockGW.Stop()

	// Client should detect disconnect
	time.Sleep(100 * time.Millisecond)
	assert.NotEqual(t, gateway.StateConnected, client.State())

	client.Close()
}

func TestHistoryLoading(t *testing.T) {
	mockGW := mocks.NewMockGateway()
	require.NoError(t, mockGW.Start())
	defer mockGW.Stop()

	client := gateway.NewClient(gateway.ClientConfig{
		URL:          mockGW.URL(),
		Token:        "valid-token",
		Agent:        "test-agent",
		Session:      "test-session",
		HistoryLimit: 10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, client.Connect(ctx))
	defer client.Close()

	// Wait for history to be loaded
	event, err := helpers.WaitForMessageKind(ctx, client.Messages(), gateway.KindHistory)
	require.NoError(t, err)
	assert.Len(t, event.HistoryMessages, 2) // Mock returns 2 messages
}

func TestMessageQueueDuringDisconnect(t *testing.T) {
	mockGW := mocks.NewMockGateway()
	require.NoError(t, mockGW.Start())

	client := gateway.NewClient(gateway.ClientConfig{
		URL:     mockGW.URL(),
		Token:   "valid-token",
		Agent:   "test-agent",
		Session: "test-session",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, client.Connect(ctx))

	// Stop server to simulate disconnect
	mockGW.Stop()
	time.Sleep(100 * time.Millisecond)

	// Send should queue messages instead of failing
	err := client.Send(gateway.OutboundMessage{
		Type: "chat.send",
		Payload: gateway.ChatSendParams{
			Content:    "Queued message",
			SessionKey: "test-session",
			AgentID:    "test-agent",
		},
	})
	// Should not error - message is queued
	assert.NoError(t, err)

	client.Close()
}

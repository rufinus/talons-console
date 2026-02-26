// Package helpers provides test helpers for E2E tests.
package helpers

import (
	"context"
	"fmt"
	"time"

	"github.com/rufinus/talons-console/internal/gateway"
)

// WaitForConnect waits for a Gateway client to connect within the timeout.
func WaitForConnect(ctx context.Context, client gateway.GatewayClient) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for connection")
		case <-time.After(100 * time.Millisecond):
			if client.State() == gateway.StateConnected {
				return nil
			}
		}
	}
}

// WaitForAuth waits for authentication to complete within the timeout.
func WaitForAuth(ctx context.Context, messages <-chan gateway.InboundEvent) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for auth")
		case event := <-messages:
			if event.Kind == gateway.KindAuthResult {
				if event.Success {
					return nil
				}
				return fmt.Errorf("authentication failed")
			}
		}
	}
}

// WaitForMessage waits for any message within the timeout.
func WaitForMessage(ctx context.Context, messages <-chan gateway.InboundEvent) (gateway.InboundEvent, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		return gateway.InboundEvent{}, fmt.Errorf("timeout waiting for message")
	case event := <-messages:
		return event, nil
	}
}

// WaitForMessageKind waits for a specific message kind within the timeout.
func WaitForMessageKind(
	ctx context.Context,
	messages <-chan gateway.InboundEvent,
	kind gateway.InboundKind,
) (gateway.InboundEvent, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return gateway.InboundEvent{}, fmt.Errorf("timeout waiting for message kind %d", kind)
		case event := <-messages:
			if event.Kind == kind {
				return event, nil
			}
		}
	}
}

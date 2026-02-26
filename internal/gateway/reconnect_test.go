package gateway

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultReconnectPolicy(t *testing.T) {
	p := DefaultReconnectPolicy()
	assert.Equal(t, 1*time.Second, p.InitialDelay)
	assert.Equal(t, 30*time.Second, p.MaxDelay)
	assert.Equal(t, 5, p.MaxAttempts)
	assert.Equal(t, 2.0, p.Multiplier)
}

func TestReconnectPolicy_NextDelay_DefaultSequence(t *testing.T) {
	p := DefaultReconnectPolicy()
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		// attempt 6 exceeds MaxAttempts (5) → 0
		{6, 0},
		{7, 0},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			got := p.NextDelay(tc.attempt)
			assert.Equal(t, tc.want, got,
				"attempt %d: expected %v, got %v", tc.attempt, tc.want, got)
		})
	}
}

func TestReconnectPolicy_NextDelay_CappedAtMaxDelay(t *testing.T) {
	p := ReconnectPolicy{
		InitialDelay: 10 * time.Second,
		MaxDelay:     20 * time.Second,
		MaxAttempts:  10,
		Multiplier:   3.0,
	}
	// attempt 1: 10s, attempt 2: 30s → capped at 20s
	assert.Equal(t, 10*time.Second, p.NextDelay(1))
	assert.Equal(t, 20*time.Second, p.NextDelay(2)) // 30s capped to 20s
	assert.Equal(t, 20*time.Second, p.NextDelay(3))
}

func TestReconnectPolicy_ShouldRetry(t *testing.T) {
	p := DefaultReconnectPolicy() // MaxAttempts=5
	for i := 1; i <= 5; i++ {
		assert.True(t, p.ShouldRetry(i), "attempt %d should retry", i)
	}
	assert.False(t, p.ShouldRetry(6), "attempt 6 should not retry")
	assert.False(t, p.ShouldRetry(10), "attempt 10 should not retry")
}

func TestReconnectPolicy_ZeroMultiplier_DefaultsTo2(t *testing.T) {
	p := ReconnectPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		MaxAttempts:  3,
		Multiplier:   0, // invalid → should default to 2.0 internally
	}
	// With multiplier defaulting to 2.0: attempt 1=1s, 2=2s, 3=4s
	assert.Equal(t, 1*time.Second, p.NextDelay(1))
	assert.Equal(t, 2*time.Second, p.NextDelay(2))
}

func TestReconnectPolicy_next(t *testing.T) {
	p := DefaultReconnectPolicy()
	assert.Equal(t, 2*time.Second, p.next(1*time.Second))
	assert.Equal(t, 4*time.Second, p.next(2*time.Second))
	assert.Equal(t, 30*time.Second, p.next(20*time.Second)) // capped
}

func TestReconnectPolicy_FirstAttemptIsInitialDelay(t *testing.T) {
	p := DefaultReconnectPolicy()
	// First attempt (1) should return InitialDelay * Multiplier^0 = 1s
	assert.Equal(t, p.InitialDelay, p.NextDelay(1))
}

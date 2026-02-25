package gateway

import (
	"math"
	"time"
)

// ReconnectPolicy describes the exponential backoff strategy used when the
// Gateway connection drops. All methods use value receivers — the policy is
// treated as immutable once created.
type ReconnectPolicy struct {
	InitialDelay time.Duration // delay before the first reconnect attempt
	MaxDelay     time.Duration // delay is capped at this value
	MaxAttempts  int           // reconnect attempts before giving up
	Multiplier   float64       // back-off multiplier (typically 2.0)
}

// DefaultReconnectPolicy returns the policy described in the PRD:
// 1 s → 2 s → 4 s → … capped at 30 s, max 5 attempts.
func DefaultReconnectPolicy() ReconnectPolicy {
	return ReconnectPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		MaxAttempts:  5,
		Multiplier:   2.0,
	}
}

// NextDelay returns the delay to wait before attempt number n (1-based).
// Returns 0 if n > MaxAttempts (caller should stop retrying).
//
// Delay sequence for the default policy:
//
//	attempt 1 → 1s
//	attempt 2 → 2s
//	attempt 3 → 4s
//	attempt 4 → 8s
//	attempt 5 → 16s
//	attempt 6+→ 0 (give up)
func (p ReconnectPolicy) NextDelay(attempt int) time.Duration {
	if attempt > p.MaxAttempts {
		return 0
	}
	multiplier := p.Multiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}
	delay := float64(p.InitialDelay) * math.Pow(multiplier, float64(attempt-1))
	if delay > float64(p.MaxDelay) {
		return p.MaxDelay
	}
	return time.Duration(delay)
}

// next is an unexported helper used by the client's reconnect loop to
// advance the delay for the next iteration.
func (p ReconnectPolicy) next(current time.Duration) time.Duration {
	next := time.Duration(float64(current) * p.Multiplier)
	if next > p.MaxDelay {
		return p.MaxDelay
	}
	return next
}

// ShouldRetry reports whether another reconnection attempt is warranted.
// attempt is 1-based.
func (p ReconnectPolicy) ShouldRetry(attempt int) bool {
	return attempt <= p.MaxAttempts
}

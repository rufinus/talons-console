package gateway

import "sync"

// Queue is a bounded, thread-safe FIFO buffer for outbound messages.
// When the queue is full, the oldest message is dropped to make room.
type Queue struct {
	MaxSize  int
	mu       sync.Mutex
	messages []OutboundMessage
}

// NewQueue creates a Queue with the given maximum capacity.
func NewQueue(maxSize int) *Queue {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &Queue{
		MaxSize:  maxSize,
		messages: make([]OutboundMessage, 0, maxSize),
	}
}

// Enqueue adds msg to the back of the queue.
// If the queue is at capacity, the oldest message is silently dropped
// and the method returns dropped=true.
func (q *Queue) Enqueue(msg OutboundMessage) (dropped bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.messages) >= q.MaxSize {
		// Drop the oldest (front) message
		q.messages = q.messages[1:]
		dropped = true
	}
	q.messages = append(q.messages, msg)
	return dropped
}

// Drain returns all queued messages in FIFO order and resets the queue.
// Returns nil if the queue is empty.
func (q *Queue) Drain() []OutboundMessage {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.messages) == 0 {
		return nil
	}
	out := make([]OutboundMessage, len(q.messages))
	copy(out, q.messages)
	q.messages = q.messages[:0]
	return out
}

// Len returns the current number of messages in the queue.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.messages)
}

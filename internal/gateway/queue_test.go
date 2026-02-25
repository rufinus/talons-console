package gateway

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueue_EnqueueDrain(t *testing.T) {
	q := NewQueue(5)

	for i := 0; i < 3; i++ {
		dropped := q.Enqueue(OutboundMessage{Type: fmt.Sprintf("msg-%d", i)})
		assert.False(t, dropped, "should not drop when below capacity")
	}
	assert.Equal(t, 3, q.Len())

	msgs := q.Drain()
	require.Len(t, msgs, 3)
	// FIFO order
	assert.Equal(t, "msg-0", msgs[0].Type)
	assert.Equal(t, "msg-1", msgs[1].Type)
	assert.Equal(t, "msg-2", msgs[2].Type)

	assert.Equal(t, 0, q.Len())
	assert.Nil(t, q.Drain())
}

func TestQueue_Overflow_DropsOldest(t *testing.T) {
	q := NewQueue(3)

	for i := 0; i < 3; i++ {
		q.Enqueue(OutboundMessage{Type: fmt.Sprintf("msg-%d", i)})
	}
	assert.Equal(t, 3, q.Len())

	// 4th enqueue should drop msg-0
	dropped := q.Enqueue(OutboundMessage{Type: "msg-3"})
	assert.True(t, dropped)
	assert.Equal(t, 3, q.Len())

	msgs := q.Drain()
	require.Len(t, msgs, 3)
	assert.Equal(t, "msg-1", msgs[0].Type, "oldest should be dropped")
	assert.Equal(t, "msg-2", msgs[1].Type)
	assert.Equal(t, "msg-3", msgs[2].Type)
}

func TestQueue_DrainEmpty(t *testing.T) {
	q := NewQueue(10)
	assert.Nil(t, q.Drain())
	assert.Equal(t, 0, q.Len())
}

func TestQueue_DefaultMaxSize(t *testing.T) {
	q := NewQueue(0) // invalid → defaults to 100
	assert.Equal(t, 100, q.MaxSize)
}

func TestQueue_ConcurrentEnqueue(t *testing.T) {
	q := NewQueue(200)
	var wg sync.WaitGroup
	n := 50
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			q.Enqueue(OutboundMessage{Type: fmt.Sprintf("msg-%d", idx)})
		}(i)
	}
	wg.Wait()
	assert.Equal(t, n, q.Len())
}

func TestQueue_ConcurrentEnqueueDrain(t *testing.T) {
	q := NewQueue(100)
	var wg sync.WaitGroup

	// 5 goroutines enqueuing
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				q.Enqueue(OutboundMessage{Type: fmt.Sprintf("g%d-msg%d", idx, j)})
			}
		}(i)
	}

	// 2 goroutines draining
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_ = q.Drain()
		}()
	}

	wg.Wait()
	// No assertions on count — just verify no race conditions or panics
}

func TestQueue_FullOverflow_MultipleDrops(t *testing.T) {
	q := NewQueue(2)

	q.Enqueue(OutboundMessage{Type: "a"})
	q.Enqueue(OutboundMessage{Type: "b"})

	dropped1 := q.Enqueue(OutboundMessage{Type: "c"}) // drops "a"
	dropped2 := q.Enqueue(OutboundMessage{Type: "d"}) // drops "b"

	assert.True(t, dropped1)
	assert.True(t, dropped2)

	msgs := q.Drain()
	require.Len(t, msgs, 2)
	assert.Equal(t, "c", msgs[0].Type)
	assert.Equal(t, "d", msgs[1].Type)
}

// BenchmarkQueue_Enqueue measures enqueue throughput.
func BenchmarkQueue_Enqueue(b *testing.B) {
	q := NewQueue(1000)
	msg := OutboundMessage{Type: "bench"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(msg)
	}
}

// BenchmarkQueue_EnqueueDrain measures enqueue+drain throughput.
func BenchmarkQueue_EnqueueDrain(b *testing.B) {
	q := NewQueue(100)
	msg := OutboundMessage{Type: "bench"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Enqueue(msg)
		if i%100 == 99 {
			q.Drain()
		}
	}
}

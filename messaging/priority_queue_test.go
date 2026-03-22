package messaging

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriorityNames(t *testing.T) {
	assert.Equal(t, "realtime", PriorityNames[PriorityRealtime])
	assert.Equal(t, "normal", PriorityNames[PriorityNormal])
	assert.Equal(t, "dht", PriorityNames[PriorityDHT])
	assert.Equal(t, "file_transfer", PriorityNames[PriorityFileTransfer])
}

func TestDefaultPriorityQueueConfig(t *testing.T) {
	config := DefaultPriorityQueueConfig()

	assert.Equal(t, 10000, config.MaxSize)
	assert.Equal(t, PriorityNormal, config.DefaultPriority)
	assert.True(t, config.EnableStats)
}

func TestNewPriorityQueue(t *testing.T) {
	pq := NewPriorityQueue(nil)
	require.NotNil(t, pq)

	assert.Equal(t, 0, pq.Len())
	assert.True(t, pq.IsEmpty())
	assert.False(t, pq.IsClosed())
}

func TestPriorityQueueCustomConfig(t *testing.T) {
	config := &PriorityQueueConfig{
		MaxSize:         100,
		DefaultPriority: PriorityDHT,
		EnableStats:     false,
	}

	pq := NewPriorityQueue(config)
	require.NotNil(t, pq)

	assert.Equal(t, 100, pq.config.MaxSize)
	assert.Equal(t, PriorityDHT, pq.config.DefaultPriority)
}

func TestPriorityQueueEnqueueDequeue(t *testing.T) {
	pq := NewPriorityQueue(nil)

	msg1 := NewMessage(1, "message 1", MessageTypeNormal)
	msg2 := NewMessage(2, "message 2", MessageTypeNormal)

	assert.True(t, pq.Enqueue(msg1, PriorityNormal))
	assert.True(t, pq.Enqueue(msg2, PriorityNormal))

	assert.Equal(t, 2, pq.Len())

	dequeued := pq.Dequeue()
	assert.Equal(t, msg1, dequeued) // FIFO within same priority

	dequeued = pq.Dequeue()
	assert.Equal(t, msg2, dequeued)

	assert.True(t, pq.IsEmpty())
}

func TestPriorityQueuePriorityOrdering(t *testing.T) {
	pq := NewPriorityQueue(nil)

	// Enqueue in reverse priority order
	msgLow := NewMessage(1, "low priority", MessageTypeNormal)
	msgMed := NewMessage(2, "medium priority", MessageTypeNormal)
	msgHigh := NewMessage(3, "high priority", MessageTypeNormal)

	pq.Enqueue(msgLow, PriorityFileTransfer) // Lowest priority (3)
	pq.Enqueue(msgMed, PriorityNormal)       // Medium priority (1)
	pq.Enqueue(msgHigh, PriorityRealtime)    // Highest priority (0)

	// Should dequeue in priority order
	assert.Equal(t, msgHigh, pq.Dequeue())
	assert.Equal(t, msgMed, pq.Dequeue())
	assert.Equal(t, msgLow, pq.Dequeue())
}

func TestPriorityQueueFIFOWithinPriority(t *testing.T) {
	pq := NewPriorityQueue(nil)

	// Enqueue multiple messages at same priority
	msg1 := NewMessage(1, "first", MessageTypeNormal)
	msg2 := NewMessage(2, "second", MessageTypeNormal)
	msg3 := NewMessage(3, "third", MessageTypeNormal)

	// Small delay to ensure timestamps differ
	pq.Enqueue(msg1, PriorityNormal)
	time.Sleep(1 * time.Millisecond)
	pq.Enqueue(msg2, PriorityNormal)
	time.Sleep(1 * time.Millisecond)
	pq.Enqueue(msg3, PriorityNormal)

	// Should be FIFO within same priority
	assert.Equal(t, msg1, pq.Dequeue())
	assert.Equal(t, msg2, pq.Dequeue())
	assert.Equal(t, msg3, pq.Dequeue())
}

func TestPriorityQueueMaxSize(t *testing.T) {
	config := &PriorityQueueConfig{
		MaxSize:         3,
		DefaultPriority: PriorityNormal,
	}

	pq := NewPriorityQueue(config)

	msg1 := NewMessage(1, "msg1", MessageTypeNormal)
	msg2 := NewMessage(2, "msg2", MessageTypeNormal)
	msg3 := NewMessage(3, "msg3", MessageTypeNormal)
	msg4 := NewMessage(4, "msg4", MessageTypeNormal)

	assert.True(t, pq.Enqueue(msg1, PriorityNormal))
	assert.True(t, pq.Enqueue(msg2, PriorityNormal))
	assert.True(t, pq.Enqueue(msg3, PriorityNormal))
	assert.False(t, pq.Enqueue(msg4, PriorityNormal)) // Should be dropped

	assert.Equal(t, 3, pq.Len())
}

func TestPriorityQueueEnqueueWithDefault(t *testing.T) {
	config := &PriorityQueueConfig{
		MaxSize:         100,
		DefaultPriority: PriorityDHT,
	}

	pq := NewPriorityQueue(config)

	msg := NewMessage(1, "test", MessageTypeNormal)
	assert.True(t, pq.EnqueueWithDefault(msg))

	counts := pq.CountByPriority()
	assert.Equal(t, 1, counts[PriorityDHT])
}

func TestPriorityQueuePeek(t *testing.T) {
	pq := NewPriorityQueue(nil)

	// Empty queue
	assert.Nil(t, pq.Peek())

	msg := NewMessage(1, "test", MessageTypeNormal)
	pq.Enqueue(msg, PriorityNormal)

	// Peek should return message without removing it
	peeked := pq.Peek()
	assert.Equal(t, msg, peeked)
	assert.Equal(t, 1, pq.Len()) // Still in queue

	// Dequeue should return the same message
	dequeued := pq.Dequeue()
	assert.Equal(t, msg, dequeued)
	assert.Equal(t, 0, pq.Len())
}

func TestPriorityQueuePeekPriority(t *testing.T) {
	pq := NewPriorityQueue(nil)

	// Empty queue returns default
	assert.Equal(t, PriorityNormal, pq.PeekPriority())

	msg := NewMessage(1, "test", MessageTypeNormal)
	pq.Enqueue(msg, PriorityRealtime)

	assert.Equal(t, PriorityRealtime, pq.PeekPriority())
}

func TestPriorityQueueClear(t *testing.T) {
	pq := NewPriorityQueue(nil)

	for i := 0; i < 10; i++ {
		msg := NewMessage(uint32(i), "test", MessageTypeNormal)
		pq.Enqueue(msg, PriorityNormal)
	}

	assert.Equal(t, 10, pq.Len())

	pq.Clear()

	assert.Equal(t, 0, pq.Len())
	assert.True(t, pq.IsEmpty())
}

func TestPriorityQueueClose(t *testing.T) {
	pq := NewPriorityQueue(nil)

	assert.False(t, pq.IsClosed())

	pq.Close()

	assert.True(t, pq.IsClosed())

	// Enqueue should fail on closed queue
	msg := NewMessage(1, "test", MessageTypeNormal)
	assert.False(t, pq.Enqueue(msg, PriorityNormal))
}

func TestPriorityQueueDequeueEmpty(t *testing.T) {
	pq := NewPriorityQueue(nil)

	// Dequeue from empty queue should return nil
	assert.Nil(t, pq.Dequeue())
}

func TestPriorityQueueStats(t *testing.T) {
	pq := NewPriorityQueue(nil)

	// Enqueue messages at different priorities
	for i := 0; i < 5; i++ {
		msg := NewMessage(uint32(i), "realtime", MessageTypeNormal)
		pq.Enqueue(msg, PriorityRealtime)
	}
	for i := 0; i < 3; i++ {
		msg := NewMessage(uint32(i+10), "normal", MessageTypeNormal)
		pq.Enqueue(msg, PriorityNormal)
	}

	// Dequeue some
	pq.Dequeue()
	pq.Dequeue()
	pq.Dequeue()

	enqueued, dequeued, dropped, byPriority, peakSize, _ := pq.Stats()

	assert.Equal(t, uint64(8), enqueued)
	assert.Equal(t, uint64(3), dequeued)
	assert.Equal(t, uint64(0), dropped)
	assert.Equal(t, uint64(5), byPriority[PriorityRealtime])
	assert.Equal(t, uint64(3), byPriority[PriorityNormal])
	assert.Equal(t, int64(8), peakSize)
}

func TestPriorityQueueDrainTo(t *testing.T) {
	pq := NewPriorityQueue(nil)

	for i := 0; i < 5; i++ {
		msg := NewMessage(uint32(i), "test", MessageTypeNormal)
		pq.Enqueue(msg, PriorityNormal)
	}

	var dst []*Message
	dst = pq.DrainTo(dst)

	assert.Equal(t, 5, len(dst))
	assert.True(t, pq.IsEmpty())
}

func TestPriorityQueueCountByPriority(t *testing.T) {
	pq := NewPriorityQueue(nil)

	for i := 0; i < 3; i++ {
		msg := NewMessage(uint32(i), "realtime", MessageTypeNormal)
		pq.Enqueue(msg, PriorityRealtime)
	}
	for i := 0; i < 5; i++ {
		msg := NewMessage(uint32(i+10), "normal", MessageTypeNormal)
		pq.Enqueue(msg, PriorityNormal)
	}
	for i := 0; i < 2; i++ {
		msg := NewMessage(uint32(i+20), "file", MessageTypeNormal)
		pq.Enqueue(msg, PriorityFileTransfer)
	}

	counts := pq.CountByPriority()

	assert.Equal(t, 3, counts[PriorityRealtime])
	assert.Equal(t, 5, counts[PriorityNormal])
	assert.Equal(t, 2, counts[PriorityFileTransfer])
	assert.Equal(t, 0, counts[PriorityDHT])
}

func TestPriorityQueueConcurrency(t *testing.T) {
	pq := NewPriorityQueue(nil)

	var wg sync.WaitGroup
	numProducers := 5
	numConsumers := 3
	msgsPerProducer := 100

	// Producers
	for p := 0; p < numProducers; p++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			for i := 0; i < msgsPerProducer; i++ {
				msg := NewMessage(uint32(producerID*1000+i), "test", MessageTypeNormal)
				priority := MessagePriority(i % 4)
				pq.Enqueue(msg, priority)
			}
		}(p)
	}

	// Consumers
	var consumed int64
	var consumedMu sync.Mutex
	for c := 0; c < numConsumers; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				msg := pq.Dequeue()
				if msg == nil {
					time.Sleep(1 * time.Millisecond)
					// Check if producers are done and queue is empty
					consumedMu.Lock()
					if consumed >= int64(numProducers*msgsPerProducer) {
						consumedMu.Unlock()
						return
					}
					consumedMu.Unlock()
					continue
				}
				consumedMu.Lock()
				consumed++
				consumedMu.Unlock()
			}
		}()
	}

	wg.Wait()

	// All messages should be processed
	consumedMu.Lock()
	assert.Equal(t, int64(numProducers*msgsPerProducer), consumed)
	consumedMu.Unlock()
}

func TestGetMessagePriority(t *testing.T) {
	assert.Equal(t, PriorityRealtime, GetMessagePriority(MessageTypeAction))
	assert.Equal(t, PriorityNormal, GetMessagePriority(MessageTypeNormal))
}

func TestIsRealtimeMessage(t *testing.T) {
	// Action message is always real-time
	actionMsg := NewMessage(1, "test", MessageTypeAction)
	assert.True(t, IsRealtimeMessage(actionMsg))

	// Recent normal message is real-time
	recentMsg := NewMessage(2, "test", MessageTypeNormal)
	assert.True(t, IsRealtimeMessage(recentMsg))

	// Old normal message is not real-time
	oldMsg := NewMessage(3, "test", MessageTypeNormal)
	oldMsg.Timestamp = time.Now().Add(-1 * time.Second)
	assert.False(t, IsRealtimeMessage(oldMsg))
}

func TestPriorityHeapIntegrity(t *testing.T) {
	// Test that heap property is maintained
	pq := NewPriorityQueue(nil)

	// Add many messages in random order
	priorities := []MessagePriority{
		PriorityFileTransfer, PriorityRealtime, PriorityNormal,
		PriorityDHT, PriorityRealtime, PriorityNormal,
		PriorityFileTransfer, PriorityRealtime, PriorityDHT,
	}

	for i, p := range priorities {
		msg := NewMessage(uint32(i), "test", MessageTypeNormal)
		pq.Enqueue(msg, p)
	}

	// Dequeue and verify ordering
	var lastPriority MessagePriority = PriorityRealtime
	for !pq.IsEmpty() {
		peeked := pq.PeekPriority()
		dequeued := pq.Dequeue()
		assert.NotNil(t, dequeued)
		// Current priority should be >= last (higher number = lower priority)
		assert.True(t, peeked >= lastPriority || lastPriority == peeked)
		lastPriority = peeked
	}
}

func TestPriorityQueueDequeueWaitClose(t *testing.T) {
	pq := NewPriorityQueue(nil)

	done := make(chan bool)

	go func() {
		msg := pq.DequeueWait()
		assert.Nil(t, msg) // Should return nil when closed
		done <- true
	}()

	// Give goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	pq.Close()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("DequeueWait did not return after Close")
	}
}

func TestPriorityQueueDequeueWaitItem(t *testing.T) {
	pq := NewPriorityQueue(nil)

	done := make(chan *Message)

	go func() {
		msg := pq.DequeueWait()
		done <- msg
	}()

	// Give goroutine time to start waiting
	time.Sleep(50 * time.Millisecond)

	expected := NewMessage(1, "test", MessageTypeNormal)
	pq.Enqueue(expected, PriorityNormal)

	select {
	case msg := <-done:
		assert.Equal(t, expected, msg)
	case <-time.After(1 * time.Second):
		t.Fatal("DequeueWait did not return after Enqueue")
	}
}

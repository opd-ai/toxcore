// Package messaging provides message handling with priority-based processing.
//
// This file implements priority queues for message types, enabling real-time
// messages to be processed before lower-priority items like DHT maintenance
// and file transfers.
package messaging

import (
	"container/heap"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// MessagePriority defines priority levels for message processing.
type MessagePriority uint8

const (
	// PriorityRealtime is for real-time messages (highest priority).
	PriorityRealtime MessagePriority = 0
	// PriorityNormal is for regular messages.
	PriorityNormal MessagePriority = 1
	// PriorityDHT is for DHT maintenance operations.
	PriorityDHT MessagePriority = 2
	// PriorityFileTransfer is for file transfer chunks (lowest priority).
	PriorityFileTransfer MessagePriority = 3
)

// PriorityNames maps priorities to human-readable names.
var PriorityNames = map[MessagePriority]string{
	PriorityRealtime:     "realtime",
	PriorityNormal:       "normal",
	PriorityDHT:          "dht",
	PriorityFileTransfer: "file_transfer",
}

// PriorityItem wraps a message with priority metadata for queue ordering.
type PriorityItem struct {
	Message   *Message
	Priority  MessagePriority
	Timestamp time.Time
	index     int // Internal index for heap operations
}

// PriorityHeap implements heap.Interface for priority-based message ordering.
// Lower priority values are processed first (0 = highest priority).
// Within the same priority, earlier timestamps are processed first (FIFO).
type PriorityHeap []*PriorityItem

// Len returns the number of items in the heap.
func (h PriorityHeap) Len() int { return len(h) }

// Less compares two items for heap ordering.
// Returns true if item i should be processed before item j.
func (h PriorityHeap) Less(i, j int) bool {
	// First compare by priority (lower value = higher priority)
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	// Same priority: earlier timestamp first (FIFO within priority)
	return h[i].Timestamp.Before(h[j].Timestamp)
}

// Swap swaps two items in the heap.
func (h PriorityHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

// Push adds an item to the heap.
func (h *PriorityHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*PriorityItem)
	item.index = n
	*h = append(*h, item)
}

// Pop removes and returns the highest-priority item.
func (h *PriorityHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // Avoid memory leak
	item.index = -1 // Mark as removed
	*h = old[0 : n-1]
	return item
}

// PriorityQueueConfig holds configuration for the priority queue.
type PriorityQueueConfig struct {
	// MaxSize is the maximum number of items in the queue (0 = unlimited).
	MaxSize int
	// DefaultPriority is the priority for items without explicit priority.
	DefaultPriority MessagePriority
	// EnableStats enables statistics tracking.
	EnableStats bool
}

// DefaultPriorityQueueConfig returns sensible defaults.
func DefaultPriorityQueueConfig() PriorityQueueConfig {
	return PriorityQueueConfig{
		MaxSize:         10000,
		DefaultPriority: PriorityNormal,
		EnableStats:     true,
	}
}

// PriorityQueueStats tracks queue statistics.
type PriorityQueueStats struct {
	Enqueued      atomic.Uint64
	Dequeued      atomic.Uint64
	Dropped       atomic.Uint64
	ByPriority    [4]atomic.Uint64 // One counter per priority level
	PeakSize      atomic.Int64
	TotalWaitTime atomic.Int64 // Total wait time in nanoseconds
}

// PriorityQueue provides a thread-safe priority queue for messages.
//
//export ToxPriorityQueue
type PriorityQueue struct {
	heap   PriorityHeap
	mu     sync.Mutex
	config PriorityQueueConfig
	stats  PriorityQueueStats
	cond   *sync.Cond
	closed atomic.Bool
}

// NewPriorityQueue creates a new priority queue.
//
//export ToxNewPriorityQueue
func NewPriorityQueue(config *PriorityQueueConfig) *PriorityQueue {
	if config == nil {
		defaultConfig := DefaultPriorityQueueConfig()
		config = &defaultConfig
	}

	pq := &PriorityQueue{
		heap:   make(PriorityHeap, 0, 100),
		config: *config,
	}
	pq.cond = sync.NewCond(&pq.mu)
	heap.Init(&pq.heap)

	logrus.WithFields(logrus.Fields{
		"function":         "NewPriorityQueue",
		"max_size":         config.MaxSize,
		"default_priority": PriorityNames[config.DefaultPriority],
	}).Debug("Created priority queue")

	return pq
}

// Enqueue adds a message to the queue with the specified priority.
// Returns false if the queue is full or closed.
//
//export ToxPriorityQueueEnqueue
func (pq *PriorityQueue) Enqueue(msg *Message, priority MessagePriority) bool {
	if pq.closed.Load() {
		return false
	}

	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Check size limit
	if pq.config.MaxSize > 0 && pq.heap.Len() >= pq.config.MaxSize {
		pq.stats.Dropped.Add(1)
		logrus.WithFields(logrus.Fields{
			"function":   "Enqueue",
			"queue_size": pq.heap.Len(),
			"max_size":   pq.config.MaxSize,
			"message_id": msg.ID,
			"priority":   PriorityNames[priority],
		}).Debug("Queue full, dropping message")
		return false
	}

	item := &PriorityItem{
		Message:   msg,
		Priority:  priority,
		Timestamp: time.Now(),
	}

	heap.Push(&pq.heap, item)

	// Update stats
	pq.stats.Enqueued.Add(1)
	if priority < 4 {
		pq.stats.ByPriority[priority].Add(1)
	}

	currentSize := int64(pq.heap.Len())
	for {
		peak := pq.stats.PeakSize.Load()
		if currentSize <= peak || pq.stats.PeakSize.CompareAndSwap(peak, currentSize) {
			break
		}
	}

	// Signal waiting consumers
	pq.cond.Signal()

	return true
}

// EnqueueWithDefault adds a message using the default priority.
func (pq *PriorityQueue) EnqueueWithDefault(msg *Message) bool {
	return pq.Enqueue(msg, pq.config.DefaultPriority)
}

// popAndUpdateStats removes an item from the heap and updates dequeue statistics.
// Caller must hold pq.mu.
func (pq *PriorityQueue) popAndUpdateStats() *Message {
	item := heap.Pop(&pq.heap).(*PriorityItem)
	pq.stats.Dequeued.Add(1)
	waitTime := time.Since(item.Timestamp)
	pq.stats.TotalWaitTime.Add(int64(waitTime))
	return item.Message
}

// Dequeue removes and returns the highest-priority message.
// Returns nil if the queue is empty.
//
//export ToxPriorityQueueDequeue
func (pq *PriorityQueue) Dequeue() *Message {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.heap.Len() == 0 {
		return nil
	}

	return pq.popAndUpdateStats()
}

// DequeueWait blocks until a message is available or the queue is closed.
// Returns nil if the queue is closed.
//
//export ToxPriorityQueueDequeueWait
func (pq *PriorityQueue) DequeueWait() *Message {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for pq.heap.Len() == 0 && !pq.closed.Load() {
		pq.cond.Wait()
	}

	if pq.closed.Load() && pq.heap.Len() == 0 {
		return nil
	}

	return pq.popAndUpdateStats()
}

// DequeueWithTimeout waits up to the specified duration for a message.
// Returns nil if timeout expires or queue is closed.
func (pq *PriorityQueue) DequeueWithTimeout(timeout time.Duration) *Message {
	deadline := time.Now().Add(timeout)

	pq.mu.Lock()
	defer pq.mu.Unlock()

	for pq.heap.Len() == 0 && !pq.closed.Load() {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil
		}

		// Use timed wait with a channel
		done := make(chan struct{})
		go func() {
			time.Sleep(remaining)
			pq.mu.Lock()
			pq.cond.Broadcast()
			pq.mu.Unlock()
			close(done)
		}()

		pq.cond.Wait()

		select {
		case <-done:
			// Timeout goroutine finished
		default:
			// Was signaled by new item
		}
	}

	if pq.heap.Len() == 0 {
		return nil
	}

	return pq.popAndUpdateStats()
}

// Peek returns the highest-priority message without removing it.
// Returns nil if the queue is empty.
//
//export ToxPriorityQueuePeek
func (pq *PriorityQueue) Peek() *Message {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.heap.Len() == 0 {
		return nil
	}

	return pq.heap[0].Message
}

// PeekPriority returns the priority of the next message to be processed.
// Returns PriorityNormal if the queue is empty.
func (pq *PriorityQueue) PeekPriority() MessagePriority {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.heap.Len() == 0 {
		return pq.config.DefaultPriority
	}

	return pq.heap[0].Priority
}

// Len returns the current queue size.
//
//export ToxPriorityQueueLen
func (pq *PriorityQueue) Len() int {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	return pq.heap.Len()
}

// IsEmpty returns true if the queue is empty.
func (pq *PriorityQueue) IsEmpty() bool {
	return pq.Len() == 0
}

// Clear removes all items from the queue.
//
//export ToxPriorityQueueClear
func (pq *PriorityQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	pq.heap = make(PriorityHeap, 0, 100)
	heap.Init(&pq.heap)
}

// Close closes the queue and wakes up any waiting consumers.
//
//export ToxPriorityQueueClose
func (pq *PriorityQueue) Close() {
	pq.closed.Store(true)
	pq.cond.Broadcast()
}

// IsClosed returns true if the queue has been closed.
func (pq *PriorityQueue) IsClosed() bool {
	return pq.closed.Load()
}

// Stats returns a snapshot of queue statistics.
func (pq *PriorityQueue) Stats() (enqueued, dequeued, dropped uint64, byPriority [4]uint64, peakSize, avgWaitNs int64) {
	enqueued = pq.stats.Enqueued.Load()
	dequeued = pq.stats.Dequeued.Load()
	dropped = pq.stats.Dropped.Load()
	for i := 0; i < 4; i++ {
		byPriority[i] = pq.stats.ByPriority[i].Load()
	}
	peakSize = pq.stats.PeakSize.Load()

	if dequeued > 0 {
		avgWaitNs = pq.stats.TotalWaitTime.Load() / int64(dequeued)
	}

	return enqueued, dequeued, dropped, byPriority, peakSize, avgWaitNs
}

// DrainTo moves all items to a slice and clears the queue.
// Useful for graceful shutdown processing.
func (pq *PriorityQueue) DrainTo(dst []*Message) []*Message {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for pq.heap.Len() > 0 {
		item := heap.Pop(&pq.heap).(*PriorityItem)
		dst = append(dst, item.Message)
	}

	return dst
}

// CountByPriority returns the count of messages at each priority level.
func (pq *PriorityQueue) CountByPriority() map[MessagePriority]int {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	counts := make(map[MessagePriority]int)
	for _, item := range pq.heap {
		counts[item.Priority]++
	}
	return counts
}

// --- Helper functions for priority assignment ---

// GetMessagePriority determines the priority for a message based on its type.
func GetMessagePriority(msgType MessageType) MessagePriority {
	switch msgType {
	case MessageTypeAction:
		// Action messages ("/me does something") are real-time
		return PriorityRealtime
	case MessageTypeNormal:
		return PriorityNormal
	default:
		return PriorityNormal
	}
}

// IsRealtimeMessage returns true if the message should be treated as real-time.
func IsRealtimeMessage(msg *Message) bool {
	// Action messages and recently-created messages are real-time
	if msg.Type == MessageTypeAction {
		return true
	}
	// Messages created in the last 100ms are considered real-time
	return time.Since(msg.Timestamp) < 100*time.Millisecond
}

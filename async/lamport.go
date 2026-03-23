// Package async implements asynchronous offline messaging with forward secrecy.
package async

import (
	"sync/atomic"
)

// LamportClock implements a Lamport logical clock for message ordering.
// Lamport clocks provide causal ordering of events in a distributed system
// without requiring synchronized physical clocks.
//
// The clock guarantees that if event A happens-before event B, then
// clock(A) < clock(B). However, the converse is not guaranteed:
// if clock(A) < clock(B), A may or may not have happened before B.
//
// This implementation is thread-safe and uses atomic operations.
type LamportClock struct {
	counter uint64
}

// NewLamportClock creates a new Lamport clock initialized to zero.
func NewLamportClock() *LamportClock {
	return &LamportClock{counter: 0}
}

// NewLamportClockFrom creates a new Lamport clock initialized to the given value.
// This is useful when restoring state from persistence.
func NewLamportClockFrom(value uint64) *LamportClock {
	return &LamportClock{counter: value}
}

// Tick increments the clock and returns the new timestamp.
// Call this before sending a message or performing a local event.
func (lc *LamportClock) Tick() uint64 {
	return atomic.AddUint64(&lc.counter, 1)
}

// Update updates the clock based on a received timestamp.
// The clock is set to max(current, received) + 1.
// Call this when receiving a message with a timestamp.
func (lc *LamportClock) Update(received uint64) uint64 {
	for {
		current := atomic.LoadUint64(&lc.counter)
		newValue := current
		if received > current {
			newValue = received
		}
		newValue++
		if atomic.CompareAndSwapUint64(&lc.counter, current, newValue) {
			return newValue
		}
	}
}

// Current returns the current clock value without incrementing.
func (lc *LamportClock) Current() uint64 {
	return atomic.LoadUint64(&lc.counter)
}

// Set sets the clock to a specific value.
// This should only be used during initialization or state restoration.
func (lc *LamportClock) Set(value uint64) {
	atomic.StoreUint64(&lc.counter, value)
}

// Compare returns the ordering relationship between two timestamps:
//   - -1 if a < b (a happened before b)
//   - 0 if a == b (concurrent or same event)
//   - 1 if a > b (b happened before a)
func Compare(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// MessageOrdering provides utilities for ordering messages by Lamport timestamp.
type MessageOrdering struct {
	clock *LamportClock
}

// NewMessageOrdering creates a new MessageOrdering instance.
func NewMessageOrdering() *MessageOrdering {
	return &MessageOrdering{
		clock: NewLamportClock(),
	}
}

// NewMessageOrderingFrom creates a new MessageOrdering with a preset clock value.
func NewMessageOrderingFrom(clockValue uint64) *MessageOrdering {
	return &MessageOrdering{
		clock: NewLamportClockFrom(clockValue),
	}
}

// GetTimestamp returns a new timestamp for an outgoing message.
func (mo *MessageOrdering) GetTimestamp() uint64 {
	return mo.clock.Tick()
}

// ProcessIncoming updates the clock based on an incoming message's timestamp.
// Returns the updated local timestamp.
func (mo *MessageOrdering) ProcessIncoming(timestamp uint64) uint64 {
	return mo.clock.Update(timestamp)
}

// CurrentClock returns the current clock value.
func (mo *MessageOrdering) CurrentClock() uint64 {
	return mo.clock.Current()
}

// SortByLamport sorts a slice of items by their Lamport timestamps.
// Items with equal timestamps are considered concurrent and maintain
// their relative order (stable sort).
func SortByLamport[T any](items []T, getTimestamp func(T) uint64) {
	// Simple insertion sort for stability (typically small message batches)
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && getTimestamp(items[j-1]) > getTimestamp(items[j]) {
			items[j-1], items[j] = items[j], items[j-1]
			j--
		}
	}
}

// FilterCausallyOrdered returns messages that are causally ordered
// (timestamp > threshold). This is useful for retrieving only new messages.
func FilterCausallyOrdered[T any](items []T, threshold uint64, getTimestamp func(T) uint64) []T {
	result := make([]T, 0, len(items))
	for _, item := range items {
		if getTimestamp(item) > threshold {
			result = append(result, item)
		}
	}
	return result
}

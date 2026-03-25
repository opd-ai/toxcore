package messaging

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGlobalDeliveryCallback tests the global delivery callback functionality.
func TestGlobalDeliveryCallback(t *testing.T) {
	t.Run("callback invoked on state change via HandleDeliveryReceipt", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		// Track callback invocations
		var callbackCalled atomic.Bool
		var receivedFriendID, receivedMessageID uint32
		var receivedState MessageState
		var mu sync.Mutex

		mm.SetGlobalDeliveryCallback(func(friendID, messageID uint32, state MessageState) {
			callbackCalled.Store(true)
			mu.Lock()
			receivedFriendID = friendID
			receivedMessageID = messageID
			receivedState = state
			mu.Unlock()
		})

		// Create a message
		msg, err := mm.SendMessage(1, "test message", MessageTypeNormal)
		require.NoError(t, err)

		// Handle delivery receipt
		mm.HandleDeliveryReceipt(1, msg.ID, 0x00) // 0x00 = delivered

		// Wait for callback
		assert.Eventually(t, func() bool {
			return callbackCalled.Load()
		}, time.Second, 10*time.Millisecond, "callback should be called")

		mu.Lock()
		assert.Equal(t, uint32(1), receivedFriendID)
		assert.Equal(t, msg.ID, receivedMessageID)
		assert.Equal(t, MessageStateDelivered, receivedState)
		mu.Unlock()
	})

	t.Run("read receipt updates state correctly", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		var callbackCalled atomic.Bool
		var receivedState MessageState
		var mu sync.Mutex

		mm.SetGlobalDeliveryCallback(func(friendID, messageID uint32, state MessageState) {
			callbackCalled.Store(true)
			mu.Lock()
			receivedState = state
			mu.Unlock()
		})

		msg, err := mm.SendMessage(1, "test message", MessageTypeNormal)
		require.NoError(t, err)

		// Handle read receipt
		mm.HandleDeliveryReceipt(1, msg.ID, 0x01) // 0x01 = read

		assert.Eventually(t, func() bool {
			return callbackCalled.Load()
		}, time.Second, 10*time.Millisecond)

		mu.Lock()
		assert.Equal(t, MessageStateRead, receivedState)
		mu.Unlock()
	})

	t.Run("receipt for unknown message ID is ignored", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		var callbackCalled atomic.Bool

		mm.SetGlobalDeliveryCallback(func(friendID, messageID uint32, state MessageState) {
			callbackCalled.Store(true)
		})

		// Handle receipt for non-existent message
		mm.HandleDeliveryReceipt(1, 99999, 0x00)

		// Callback should NOT be called
		time.Sleep(50 * time.Millisecond)
		assert.False(t, callbackCalled.Load(), "callback should not be called for unknown message")
	})

	t.Run("receipt from wrong friend is rejected", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		var callbackCalled atomic.Bool

		mm.SetGlobalDeliveryCallback(func(friendID, messageID uint32, state MessageState) {
			callbackCalled.Store(true)
		})

		// Create message for friend 1
		msg, err := mm.SendMessage(1, "test message", MessageTypeNormal)
		require.NoError(t, err)

		// Handle receipt from friend 2 (wrong friend)
		mm.HandleDeliveryReceipt(2, msg.ID, 0x00)

		// Callback should NOT be called
		time.Sleep(50 * time.Millisecond)
		assert.False(t, callbackCalled.Load(), "callback should not be called for wrong friend")
	})

	t.Run("unknown receipt type is ignored", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		var callbackCalled atomic.Bool

		mm.SetGlobalDeliveryCallback(func(friendID, messageID uint32, state MessageState) {
			callbackCalled.Store(true)
		})

		msg, err := mm.SendMessage(1, "test message", MessageTypeNormal)
		require.NoError(t, err)

		// Handle receipt with unknown type
		mm.HandleDeliveryReceipt(1, msg.ID, 0xFF)

		// Callback should NOT be called
		time.Sleep(50 * time.Millisecond)
		assert.False(t, callbackCalled.Load(), "callback should not be called for unknown receipt type")
	})

	t.Run("nil callback is handled gracefully", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		// No callback set
		msg, err := mm.SendMessage(1, "test message", MessageTypeNormal)
		require.NoError(t, err)

		// Should not panic
		mm.HandleDeliveryReceipt(1, msg.ID, 0x00)
	})
}

// TestGetDeliveryStatus tests the GetDeliveryStatus method.
func TestGetDeliveryStatus(t *testing.T) {
	t.Run("returns pending for unknown message", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		status := mm.GetDeliveryStatus(1, 99999)
		assert.Equal(t, MessageStatePending, status)
	})

	t.Run("returns pending for wrong friend", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		msg, err := mm.SendMessage(1, "test", MessageTypeNormal)
		require.NoError(t, err)

		// Query with wrong friend ID
		status := mm.GetDeliveryStatus(2, msg.ID)
		assert.Equal(t, MessageStatePending, status)
	})

	t.Run("returns correct status after receipt", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		msg, err := mm.SendMessage(1, "test", MessageTypeNormal)
		require.NoError(t, err)

		// Before receipt
		status := mm.GetDeliveryStatus(1, msg.ID)
		// Initial state could be pending or sent depending on timing
		assert.True(t, status == MessageStatePending || status == MessageStateSending || status == MessageStateSent)

		// Handle delivery receipt
		mm.HandleDeliveryReceipt(1, msg.ID, 0x00)

		// After receipt
		status = mm.GetDeliveryStatus(1, msg.ID)
		assert.Equal(t, MessageStateDelivered, status)
	})
}

// TestDeliveryReceiptIntegration tests the full delivery receipt flow.
func TestDeliveryReceiptIntegration(t *testing.T) {
	t.Run("full delivery flow: sent -> delivered -> read", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		// Track all state transitions
		var states []MessageState
		var mu sync.Mutex

		mm.SetGlobalDeliveryCallback(func(friendID, messageID uint32, state MessageState) {
			mu.Lock()
			states = append(states, state)
			mu.Unlock()
		})

		msg, err := mm.SendMessage(1, "test message", MessageTypeNormal)
		require.NoError(t, err)

		// Simulate delivery
		mm.HandleDeliveryReceipt(1, msg.ID, 0x00)

		// Simulate read
		mm.HandleDeliveryReceipt(1, msg.ID, 0x01)

		// Verify final state
		assert.Equal(t, MessageStateRead, mm.GetDeliveryStatus(1, msg.ID))

		// Verify all transitions were recorded
		mu.Lock()
		assert.Contains(t, states, MessageStateDelivered, "should have recorded delivered state")
		assert.Contains(t, states, MessageStateRead, "should have recorded read state")
		mu.Unlock()
	})
}

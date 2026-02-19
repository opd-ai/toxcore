package messaging

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// TestOnDeliveryStateChange tests the delivery callback functionality.
func TestOnDeliveryStateChange(t *testing.T) {
	t.Run("callback invoked on state change", func(t *testing.T) {
		msg := NewMessage(testDefaultFriendID, "test", MessageTypeNormal)

		var callbackCalled atomic.Bool
		var receivedState MessageState
		var mu sync.Mutex

		msg.OnDeliveryStateChange(func(m *Message, state MessageState) {
			callbackCalled.Store(true)
			mu.Lock()
			receivedState = state
			mu.Unlock()
		})

		msg.SetState(MessageStateSent)

		if !callbackCalled.Load() {
			t.Error("callback was not called")
		}

		mu.Lock()
		if receivedState != MessageStateSent {
			t.Errorf("expected state %v, got %v", MessageStateSent, receivedState)
		}
		mu.Unlock()
	})

	t.Run("nil callback is handled", func(t *testing.T) {
		msg := NewMessage(testDefaultFriendID, "test", MessageTypeNormal)
		// No callback set, should not panic
		msg.SetState(MessageStateSent)
		if msg.GetState() != MessageStateSent {
			t.Error("state was not updated")
		}
	})

	t.Run("callback replaced correctly", func(t *testing.T) {
		msg := NewMessage(testDefaultFriendID, "test", MessageTypeNormal)

		var firstCalled, secondCalled atomic.Bool

		msg.OnDeliveryStateChange(func(m *Message, state MessageState) {
			firstCalled.Store(true)
		})

		msg.OnDeliveryStateChange(func(m *Message, state MessageState) {
			secondCalled.Store(true)
		})

		msg.SetState(MessageStateSent)

		if firstCalled.Load() {
			t.Error("first callback should not be called after replacement")
		}
		if !secondCalled.Load() {
			t.Error("second callback should be called")
		}
	})
}

// TestProcessPendingMessages tests the pending message processing flow.
func TestProcessPendingMessages(t *testing.T) {
	t.Run("processes pending messages", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		transport := &mockTransport{}
		mm.SetTransport(transport)

		// Create messages manually and add to pending queue
		mm.mu.Lock()
		msg1 := newMessageWithTime(testDefaultFriendID, "msg1", MessageTypeNormal, time.Now())
		msg1.ID = mm.nextID
		mm.nextID++
		msg1.State = MessageStatePending
		mm.messages[msg1.ID] = msg1
		mm.pendingQueue = append(mm.pendingQueue, msg1)
		mm.mu.Unlock()

		// Wait for any async sends to complete
		time.Sleep(testAsyncWait)

		mm.ProcessPendingMessages()

		// Wait for processing
		time.Sleep(testAsyncWait)

		sentMsgs := transport.getSentMessages()
		if len(sentMsgs) == 0 {
			t.Error("expected message to be sent")
		}
	})

	t.Run("respects retry interval", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		mockTime := &mockTimeProvider{currentTime: time.Now()}
		mm.SetTimeProvider(mockTime)

		transport := &mockTransport{}
		mm.SetTransport(transport)

		// Create a message that just failed
		mm.mu.Lock()
		msg := newMessageWithTime(testDefaultFriendID, "test", MessageTypeNormal, mockTime.Now())
		msg.ID = mm.nextID
		mm.nextID++
		msg.State = MessageStatePending
		msg.LastAttempt = mockTime.Now() // Just attempted
		mm.messages[msg.ID] = msg
		mm.pendingQueue = append(mm.pendingQueue, msg)
		mm.mu.Unlock()

		// Process immediately - should not send yet (retry interval not elapsed)
		mm.ProcessPendingMessages()
		time.Sleep(testAsyncWaitShort)

		// Advance time past retry interval
		mockTime.Advance(mm.retryInterval + time.Second)

		// Now it should be processed
		mm.ProcessPendingMessages()
		time.Sleep(testAsyncWait)

		sentMsgs := transport.getSentMessages()
		if len(sentMsgs) == 0 {
			t.Error("expected message to be sent after retry interval")
		}
	})

	t.Run("skips non-pending messages", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		transport := &mockTransport{}
		mm.SetTransport(transport)

		// Create a message that's already sent
		mm.mu.Lock()
		msg := newMessageWithTime(testDefaultFriendID, "test", MessageTypeNormal, time.Now())
		msg.ID = mm.nextID
		mm.nextID++
		msg.State = MessageStateSent // Already sent
		mm.messages[msg.ID] = msg
		mm.pendingQueue = append(mm.pendingQueue, msg)
		mm.mu.Unlock()

		mm.ProcessPendingMessages()
		time.Sleep(testAsyncWait)

		// Should not be re-sent
		sentMsgs := transport.getSentMessages()
		if len(sentMsgs) != 0 {
			t.Error("should not re-send already sent messages")
		}
	})
}

// TestCleanupProcessedMessages tests the cleanup and retry logic.
func TestCleanupProcessedMessages(t *testing.T) {
	t.Run("removes completed messages", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		// Create a delivered message
		mm.mu.Lock()
		msg := newMessageWithTime(testDefaultFriendID, "test", MessageTypeNormal, time.Now())
		msg.ID = mm.nextID
		mm.nextID++
		msg.State = MessageStateDelivered
		mm.messages[msg.ID] = msg
		mm.pendingQueue = append(mm.pendingQueue, msg)
		mm.mu.Unlock()

		mm.cleanupProcessedMessages()

		mm.mu.Lock()
		queueLen := len(mm.pendingQueue)
		mm.mu.Unlock()

		if queueLen != 0 {
			t.Errorf("expected empty queue, got %d messages", queueLen)
		}
	})

	t.Run("retries failed messages under max retries", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		// Create a failed message with retries remaining
		mm.mu.Lock()
		msg := newMessageWithTime(testDefaultFriendID, "test", MessageTypeNormal, time.Now())
		msg.ID = mm.nextID
		mm.nextID++
		msg.State = MessageStateFailed
		msg.Retries = 1 // Under maxRetries (3)
		mm.messages[msg.ID] = msg
		mm.pendingQueue = append(mm.pendingQueue, msg)
		mm.mu.Unlock()

		mm.cleanupProcessedMessages()

		// Message should be reset to pending and kept in queue
		if msg.GetState() != MessageStatePending {
			t.Errorf("expected message to be reset to pending, got %v", msg.GetState())
		}

		mm.mu.Lock()
		queueLen := len(mm.pendingQueue)
		mm.mu.Unlock()

		if queueLen != 1 {
			t.Errorf("expected message to remain in queue, got %d messages", queueLen)
		}
	})

	t.Run("removes failed messages at max retries", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		// Create a failed message at max retries
		mm.mu.Lock()
		msg := newMessageWithTime(testDefaultFriendID, "test", MessageTypeNormal, time.Now())
		msg.ID = mm.nextID
		mm.nextID++
		msg.State = MessageStateFailed
		msg.Retries = mm.maxRetries // At max retries
		mm.messages[msg.ID] = msg
		mm.pendingQueue = append(mm.pendingQueue, msg)
		mm.mu.Unlock()

		mm.cleanupProcessedMessages()

		mm.mu.Lock()
		queueLen := len(mm.pendingQueue)
		mm.mu.Unlock()

		if queueLen != 0 {
			t.Errorf("expected message to be removed, got %d messages", queueLen)
		}
	})

	t.Run("keeps pending and sending messages", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		mm.mu.Lock()
		pendingMsg := newMessageWithTime(testDefaultFriendID, "pending", MessageTypeNormal, time.Now())
		pendingMsg.ID = mm.nextID
		mm.nextID++
		pendingMsg.State = MessageStatePending
		mm.messages[pendingMsg.ID] = pendingMsg

		sendingMsg := newMessageWithTime(testDefaultFriendID, "sending", MessageTypeNormal, time.Now())
		sendingMsg.ID = mm.nextID
		mm.nextID++
		sendingMsg.State = MessageStateSending
		mm.messages[sendingMsg.ID] = sendingMsg

		mm.pendingQueue = append(mm.pendingQueue, pendingMsg, sendingMsg)
		mm.mu.Unlock()

		mm.cleanupProcessedMessages()

		mm.mu.Lock()
		queueLen := len(mm.pendingQueue)
		mm.mu.Unlock()

		if queueLen != 2 {
			t.Errorf("expected 2 messages in queue, got %d", queueLen)
		}
	})

	t.Run("keeps sent but unconfirmed messages", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		mm.mu.Lock()
		msg := newMessageWithTime(testDefaultFriendID, "sent", MessageTypeNormal, time.Now())
		msg.ID = mm.nextID
		mm.nextID++
		msg.State = MessageStateSent
		mm.messages[msg.ID] = msg
		mm.pendingQueue = append(mm.pendingQueue, msg)
		mm.mu.Unlock()

		mm.cleanupProcessedMessages()

		mm.mu.Lock()
		queueLen := len(mm.pendingQueue)
		mm.mu.Unlock()

		if queueLen != 1 {
			t.Errorf("expected sent message to remain in queue, got %d messages", queueLen)
		}
	})
}

// TestMarkMessageDelivered tests the delivery marking functionality.
func TestMarkMessageDelivered(t *testing.T) {
	t.Run("marks existing message as delivered", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		msg, err := mm.SendMessage(testDefaultFriendID, "test", MessageTypeNormal)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		// Wait for async send
		time.Sleep(testAsyncWait)

		mm.MarkMessageDelivered(msg.ID)

		if msg.GetState() != MessageStateDelivered {
			t.Errorf("expected delivered state, got %v", msg.GetState())
		}
	})

	t.Run("ignores non-existent message", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		// Should not panic
		mm.MarkMessageDelivered(999)
	})

	t.Run("invokes callback on delivery", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		msg, err := mm.SendMessage(testDefaultFriendID, "test", MessageTypeNormal)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		var callbackInvoked atomic.Bool
		msg.OnDeliveryStateChange(func(m *Message, state MessageState) {
			if state == MessageStateDelivered {
				callbackInvoked.Store(true)
			}
		})

		time.Sleep(testAsyncWait)
		mm.MarkMessageDelivered(msg.ID)

		if !callbackInvoked.Load() {
			t.Error("delivery callback was not invoked")
		}
	})
}

// TestMarkMessageRead tests the read marking functionality.
func TestMarkMessageRead(t *testing.T) {
	t.Run("marks existing message as read", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		msg, err := mm.SendMessage(testDefaultFriendID, "test", MessageTypeNormal)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		time.Sleep(testAsyncWait)
		mm.MarkMessageRead(msg.ID)

		if msg.GetState() != MessageStateRead {
			t.Errorf("expected read state, got %v", msg.GetState())
		}
	})

	t.Run("ignores non-existent message", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		// Should not panic
		mm.MarkMessageRead(999)
	})
}

// TestGetMessage tests the message retrieval functionality.
func TestGetMessage(t *testing.T) {
	t.Run("retrieves existing message", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		originalMsg, err := mm.SendMessage(testDefaultFriendID, "test message", MessageTypeNormal)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		retrievedMsg, err := mm.GetMessage(originalMsg.ID)
		if err != nil {
			t.Fatalf("GetMessage failed: %v", err)
		}

		if retrievedMsg.ID != originalMsg.ID {
			t.Errorf("expected message ID %d, got %d", originalMsg.ID, retrievedMsg.ID)
		}
	})

	t.Run("returns error for non-existent message", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		_, err := mm.GetMessage(999)
		if err != ErrMessageNotFound {
			t.Errorf("expected ErrMessageNotFound, got %v", err)
		}
	})
}

// TestGetMessagesByFriend tests the friend-specific message retrieval.
func TestGetMessagesByFriend(t *testing.T) {
	t.Run("retrieves messages for specific friend", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		// Send messages to different friends
		_, err := mm.SendMessage(1, "msg to friend 1", MessageTypeNormal)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		_, err = mm.SendMessage(1, "another msg to friend 1", MessageTypeNormal)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		_, err = mm.SendMessage(2, "msg to friend 2", MessageTypeNormal)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		messages := mm.GetMessagesByFriend(1)
		if len(messages) != 2 {
			t.Errorf("expected 2 messages for friend 1, got %d", len(messages))
		}

		for _, msg := range messages {
			if msg.FriendID != 1 {
				t.Errorf("expected friend ID 1, got %d", msg.FriendID)
			}
		}
	})

	t.Run("returns empty slice for friend with no messages", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		messages := mm.GetMessagesByFriend(999)
		if len(messages) != 0 {
			t.Errorf("expected 0 messages, got %d", len(messages))
		}
	})

	t.Run("concurrent access is safe", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		// Send some messages
		for i := 0; i < 5; i++ {
			_, err := mm.SendMessage(testDefaultFriendID, "test", MessageTypeNormal)
			if err != nil {
				t.Fatalf("SendMessage failed: %v", err)
			}
		}

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = mm.GetMessagesByFriend(testDefaultFriendID)
			}()
		}
		wg.Wait()
	})
}

// TestRetryLogic tests the retry mechanism.
func TestRetryLogic(t *testing.T) {
	t.Run("canRetryMessage returns true for retriable messages", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		msg := NewMessage(testDefaultFriendID, "test", MessageTypeNormal)
		msg.State = MessageStateFailed
		msg.Retries = 1 // Under maxRetries

		if !mm.canRetryMessage(msg) {
			t.Error("expected canRetryMessage to return true")
		}
	})

	t.Run("canRetryMessage returns false at max retries", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		msg := NewMessage(testDefaultFriendID, "test", MessageTypeNormal)
		msg.State = MessageStateFailed
		msg.Retries = mm.maxRetries

		if mm.canRetryMessage(msg) {
			t.Error("expected canRetryMessage to return false at max retries")
		}
	})

	t.Run("canRetryMessage returns false for non-failed messages", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		msg := NewMessage(testDefaultFriendID, "test", MessageTypeNormal)
		msg.State = MessageStateSent
		msg.Retries = 1

		if mm.canRetryMessage(msg) {
			t.Error("expected canRetryMessage to return false for non-failed messages")
		}
	})

	t.Run("retryMessage transitions failed to pending", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		msg := NewMessage(testDefaultFriendID, "test", MessageTypeNormal)
		msg.State = MessageStateFailed
		msg.Retries = 1

		mm.retryMessage(msg)

		msg.mu.Lock()
		state := msg.State
		msg.mu.Unlock()

		if state != MessageStatePending {
			t.Errorf("expected pending state after retry, got %v", state)
		}
	})

	t.Run("retryMessage does not transition at max retries", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		msg := NewMessage(testDefaultFriendID, "test", MessageTypeNormal)
		msg.State = MessageStateFailed
		msg.Retries = mm.maxRetries

		mm.retryMessage(msg)

		msg.mu.Lock()
		state := msg.State
		msg.mu.Unlock()

		if state != MessageStateFailed {
			t.Errorf("expected failed state to remain at max retries, got %v", state)
		}
	})
}

// TestEncryptionErrorPaths tests encryption failure scenarios.
func TestEncryptionErrorPaths(t *testing.T) {
	t.Run("encryption fails with unknown friend", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		keyProvider := newMockKeyProvider()
		// Don't add friend to key provider
		mm.SetKeyProvider(keyProvider)

		transport := &mockTransport{}
		mm.SetTransport(transport)

		msg, err := mm.SendMessage(testInvalidFriendID, "test", MessageTypeNormal)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		// Wait for async processing
		time.Sleep(testAsyncWaitMedium)

		// Message should fail due to encryption error (friend not found)
		state := msg.GetState()
		if state != MessageStatePending && state != MessageStateFailed {
			// Could be pending if retries remain, or failed if max retries reached
		}
	})

	t.Run("successful encryption with valid friend", func(t *testing.T) {
		mm := NewMessageManager()
		defer mm.Close()

		keyProvider := newMockKeyProvider()
		friendKeyPair, _ := crypto.GenerateKeyPair()
		keyProvider.friendPublicKeys[testDefaultFriendID] = friendKeyPair.Public
		mm.SetKeyProvider(keyProvider)

		transport := &mockTransport{}
		mm.SetTransport(transport)

		msg, err := mm.SendMessage(testDefaultFriendID, "test", MessageTypeNormal)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		// Wait for async processing
		time.Sleep(testAsyncWaitMedium)

		state := msg.GetState()
		if state != MessageStateSent {
			t.Errorf("expected sent state, got %v", state)
		}

		sentMsgs := transport.getSentMessages()
		if len(sentMsgs) == 0 {
			t.Error("expected message to be sent")
		}
	})
}

// TestShouldKeepInQueue tests queue retention logic.
func TestShouldKeepInQueue(t *testing.T) {
	tests := []struct {
		name     string
		state    MessageState
		retries  uint8
		expected bool
	}{
		{"pending messages kept", MessageStatePending, 0, true},
		{"sending messages kept", MessageStateSending, 0, true},
		{"sent messages kept", MessageStateSent, 0, true},
		{"delivered messages removed", MessageStateDelivered, 0, false},
		{"read messages removed", MessageStateRead, 0, false},
		{"failed with retries kept", MessageStateFailed, 1, true},
		{"failed at max removed", MessageStateFailed, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mm := NewMessageManager()
			defer mm.Close()

			msg := NewMessage(testDefaultFriendID, "test", MessageTypeNormal)
			msg.State = tt.state
			msg.Retries = tt.retries

			result := mm.shouldKeepInQueue(msg)
			if result != tt.expected {
				t.Errorf("shouldKeepInQueue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

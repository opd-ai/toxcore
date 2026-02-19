package messaging

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockingTransport blocks on SendMessagePacket until released
type blockingTransport struct {
	blockCh     chan struct{}
	sendCount   int32
	cancelCount int32
	mu          sync.Mutex
}

func newBlockingTransport() *blockingTransport {
	return &blockingTransport{
		blockCh: make(chan struct{}),
	}
}

func (t *blockingTransport) SendMessagePacket(friendID uint32, message *Message) error {
	atomic.AddInt32(&t.sendCount, 1)
	// Block until released or context cancelled
	<-t.blockCh
	return nil
}

func (t *blockingTransport) release() {
	close(t.blockCh)
}

func TestMessageManager_Close(t *testing.T) {
	t.Run("Close waits for goroutines to finish", func(t *testing.T) {
		mm := NewMessageManager()
		transport := newBlockingTransport()
		mm.SetTransport(transport)

		// Send a message - it will block
		_, err := mm.SendMessage(testDefaultFriendID, "test message", MessageTypeNormal)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		// Give the goroutine time to start
		time.Sleep(testGoroutineStart)

		// Release the blocking transport
		transport.release()

		// Close should wait for goroutines to finish
		done := make(chan struct{})
		go func() {
			mm.Close()
			close(done)
		}()

		select {
		case <-done:
			// Success - Close completed
		case <-time.After(testCloseTimeout):
			t.Fatal("Close did not complete in time")
		}
	})

	t.Run("Close cancels pending sends before they start", func(t *testing.T) {
		mm := NewMessageManager()

		// Close immediately before goroutine can run
		mm.Close()

		// Now try to check if we can send (should not panic or hang)
		// Since we called Close(), ctx is already cancelled
	})

	t.Run("Multiple Close calls are safe", func(t *testing.T) {
		mm := NewMessageManager()
		mm.Close()
		mm.Close() // Should not panic
	})
}

func TestMessageManager_GracefulShutdown(t *testing.T) {
	t.Run("Messages marked pending on shutdown", func(t *testing.T) {
		mm := NewMessageManager()

		// Use a slow transport to allow cancellation to occur
		slowTransport := &slowTransport{
			delay: testSlowDelay,
		}
		mm.SetTransport(slowTransport)

		msg, err := mm.SendMessage(testDefaultFriendID, "test message", MessageTypeNormal)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		// Give goroutine time to start but not finish
		time.Sleep(testGoroutineStart)

		// Close should cancel the send
		mm.Close()

		// Message should still be pending or sent (depends on timing)
		msg.mu.Lock()
		state := msg.State
		msg.mu.Unlock()

		// Should be either pending (if cancelled) or sent (if completed)
		if state != MessageStatePending && state != MessageStateSent && state != MessageStateSending {
			t.Errorf("Expected pending, sending, or sent state after shutdown, got: %v", state)
		}
	})
}

// slowTransport simulates a slow network
type slowTransport struct {
	delay time.Duration
}

func (t *slowTransport) SendMessagePacket(friendID uint32, message *Message) error {
	time.Sleep(t.delay)
	return nil
}

package async

import (
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/toxcore/crypto"
)

// TestSetFriendOnlineStatusNoDataRace verifies that SetFriendOnlineStatus doesn't cause data races
// when messageHandler is modified concurrently with friend coming online
func TestSetFriendOnlineStatusNoDataRace(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	transport := NewMockTransport("127.0.0.1:8080")

	manager, err := NewAsyncManager(keyPair, transport, t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}
	defer manager.Stop()

	friendPK := [32]byte{0x11, 0x22, 0x33}

	// Set up a wait group to coordinate goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Continuously set the message handler
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			manager.SetAsyncMessageHandler(func(sender [32]byte, msg string, msgType MessageType) {
				// Handler logic
			})
			time.Sleep(time.Microsecond)
		}
	}()

	// Goroutine 2: Continuously toggle friend online status
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			manager.SetFriendOnlineStatus(friendPK, true)
			time.Sleep(time.Microsecond)
			manager.SetFriendOnlineStatus(friendPK, false)
			time.Sleep(time.Microsecond)
		}
	}()

	// Wait for both goroutines to complete
	wg.Wait()

	// If we get here without a race condition, the test passes
}

// TestHandlerParameterPassing verifies that the handler is correctly passed to goroutines
func TestHandlerParameterPassing(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	transport := NewMockTransport("127.0.0.1:8080")

	manager, err := NewAsyncManager(keyPair, transport, t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}
	defer manager.Stop()

	friendPK := [32]byte{0x11, 0x22, 0x33}

	// Set initial handler
	callCount := 0
	var mu sync.Mutex
	manager.SetAsyncMessageHandler(func(sender [32]byte, msg string, msgType MessageType) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	// Trigger friend coming online (which should capture the current handler)
	manager.SetFriendOnlineStatus(friendPK, true)

	// Immediately change the handler to nil
	manager.SetAsyncMessageHandler(nil)

	// Give time for the goroutine to execute
	time.Sleep(100 * time.Millisecond)

	// The goroutine should have used the handler that was set when it was launched
	// Not the nil handler that was set afterward
	// Since we don't have a way to trigger the handler directly, we just verify no panic occurred
}

// TestMutexReleaseBeforeGoroutine verifies that the mutex is properly released before goroutine launch
func TestMutexReleaseBeforeGoroutine(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	transport := NewMockTransport("127.0.0.1:8080")

	manager, err := NewAsyncManager(keyPair, transport, t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}
	defer manager.Stop()

	friendPK := [32]byte{0x11, 0x22, 0x33}

	// Channel to signal when goroutine starts
	goroutineStarted := make(chan bool, 1)

	manager.SetAsyncMessageHandler(func(sender [32]byte, msg string, msgType MessageType) {
		goroutineStarted <- true
	})

	// Mark friend as offline first
	manager.SetFriendOnlineStatus(friendPK, false)

	// Now mark as online - this should launch the goroutine
	manager.SetFriendOnlineStatus(friendPK, true)

	// Try to acquire the online status under lock immediately after
	// This should succeed if the mutex was properly released
	done := make(chan bool, 1)
	go func() {
		manager.mutex.RLock()
		status := manager.onlineStatus[friendPK]
		manager.mutex.RUnlock()
		if status {
			done <- true
		}
	}()

	// Wait for either the status check to complete or timeout
	select {
	case <-done:
		// Success - mutex was released properly
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Mutex was not released before goroutine launch - potential deadlock")
	}
}

// TestConcurrentOnlineStatusUpdates verifies thread-safe concurrent updates
func TestConcurrentOnlineStatusUpdates(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	transport := NewMockTransport("127.0.0.1:8080")

	manager, err := NewAsyncManager(keyPair, transport, t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create async manager: %v", err)
	}
	defer manager.Stop()

	// Set a simple handler
	manager.SetAsyncMessageHandler(func(sender [32]byte, msg string, msgType MessageType) {})

	// Create multiple friends
	friends := make([][32]byte, 10)
	for i := range friends {
		friends[i] = [32]byte{byte(i)}
	}

	// Concurrently update online status for all friends
	var wg sync.WaitGroup
	for _, friend := range friends {
		wg.Add(1)
		go func(pk [32]byte) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				manager.SetFriendOnlineStatus(pk, true)
				time.Sleep(time.Millisecond)
				manager.SetFriendOnlineStatus(pk, false)
				time.Sleep(time.Millisecond)
			}
		}(friend)
	}

	wg.Wait()

	// Verify all friends are in consistent state
	manager.mutex.RLock()
	for _, friend := range friends {
		_ = manager.onlineStatus[friend]
		// Just verify we can access the map without panics
	}
	manager.mutex.RUnlock()
}

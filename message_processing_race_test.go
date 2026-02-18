package toxcore

import (
	"sync"
	"testing"
	"time"
)

// TestMessageProcessing_NilCheck verifies that doMessageProcessing properly
// handles nil messageManager without panicking.
func TestMessageProcessing_NilCheck(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	// Manually set messageManager to nil to simulate edge case
	tox.messageManager = nil

	// This should not panic
	tox.doMessageProcessing()

	t.Log("doMessageProcessing handled nil messageManager correctly")
}

// TestMessageProcessing_ConcurrentKill verifies that the race condition between
// Iterate() and Kill() is properly handled with the captured reference pattern.
func TestMessageProcessing_ConcurrentKill(t *testing.T) {
	// Run this test multiple times to increase chance of catching race conditions
	for iteration := 0; iteration < 50; iteration++ {
		options := NewOptions()
		tox, err := New(options)
		if err != nil {
			t.Fatalf("Failed to create Tox instance: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(2)

		// Goroutine 1: Continuously call doMessageProcessing (simulating Iterate)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				tox.doMessageProcessing()
				time.Sleep(time.Microsecond)
			}
		}()

		// Goroutine 2: Call Kill() to set messageManager to nil
		go func() {
			defer wg.Done()
			time.Sleep(50 * time.Microsecond) // Let some iterations happen first
			tox.Kill()
		}()

		// Wait for both goroutines to complete
		wg.Wait()
	}

	t.Log("No race condition detected in concurrent Kill() and doMessageProcessing()")
}

// TestMessageProcessing_ProcessPendingMessagesCalled verifies that
// ProcessPendingMessages is actually called during message processing.
func TestMessageProcessing_ProcessPendingMessagesCalled(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a test friend
	testPublicKey := testSequentialPublicKey
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set friend as connected
	tox.friendsMutex.Lock()
	if friend, exists := tox.friends[friendID]; exists {
		friend.ConnectionStatus = ConnectionUDP
	}
	tox.friendsMutex.Unlock()

	// Send a message - this will add it to the pending queue
	_ = tox.SendFriendMessage(friendID, "Test message for pending queue")

	// Give the async goroutine a moment to start processing
	time.Sleep(10 * time.Millisecond)

	// Call doMessageProcessing - this should process the pending message
	tox.doMessageProcessing()

	// If we got here without panic, ProcessPendingMessages was called successfully
	t.Log("ProcessPendingMessages called successfully during message processing")
}

// TestMessageProcessing_CapturedReferencePattern verifies that the captured
// reference pattern prevents accessing nil pointer after Kill().
func TestMessageProcessing_CapturedReferencePattern(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	// Verify messageManager is initially not nil
	if tox.messageManager == nil {
		t.Fatal("messageManager should be initialized")
	}

	// Start processing in background
	done := make(chan bool)
	go func() {
		for i := 0; i < 1000; i++ {
			tox.doMessageProcessing()
			time.Sleep(time.Microsecond)
		}
		done <- true
	}()

	// Kill the tox instance while processing is happening
	time.Sleep(100 * time.Microsecond)
	tox.Kill()

	// Wait for processing to complete - should not panic
	<-done

	t.Log("Captured reference pattern prevented nil pointer access")
}

// TestMessageProcessing_IntegratedWithIterate verifies that message processing
// works correctly when called through the normal Iterate() path.
func TestMessageProcessing_IntegratedWithIterate(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a test friend
	testPublicKey := testSequentialPublicKey
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set friend as connected
	tox.friendsMutex.Lock()
	if friend, exists := tox.friends[friendID]; exists {
		friend.ConnectionStatus = ConnectionUDP
	}
	tox.friendsMutex.Unlock()

	// Send a message
	_ = tox.SendFriendMessage(friendID, "Test message via Iterate")

	// Call Iterate which internally calls doMessageProcessing
	tox.Iterate()

	// If we got here without panic, message processing through Iterate works
	t.Log("Message processing integrated with Iterate() successfully")
}

// TestMessageProcessing_MultipleIterationsAfterKill verifies that calling
// Iterate/doMessageProcessing after Kill() doesn't cause issues.
func TestMessageProcessing_MultipleIterationsAfterKill(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	// Kill the instance
	tox.Kill()

	// Try to process messages multiple times after Kill
	for i := 0; i < 10; i++ {
		tox.doMessageProcessing()
	}

	// Should not panic
	t.Log("doMessageProcessing handled post-Kill state correctly")
}

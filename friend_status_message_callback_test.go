package toxcore

import (
	"sync"
	"testing"
	"time"
)

// TestOnFriendStatusMessage_CallbackInvoked verifies that the OnFriendStatusMessage callback
// is invoked when a friend updates their status message
func TestOnFriendStatusMessage_CallbackInvoked(t *testing.T) {
	// Create two Tox instances
	options1 := NewOptions()
	options1.UDPEnabled = false
	tox1, err := New(options1)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	options2 := NewOptions()
	options2.UDPEnabled = false
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	// Setup callback tracking
	var callbackMu sync.Mutex
	var receivedFriendID uint32
	var receivedStatusMessage string
	var callbackInvoked bool

	tox1.OnFriendStatusMessage(func(friendID uint32, statusMessage string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		receivedFriendID = friendID
		receivedStatusMessage = statusMessage
		callbackInvoked = true
	})

	// Add tox2 as friend on tox1
	addr2 := tox2.SelfGetAddress()
	friendID, err := tox1.AddFriend(addr2, "Test friend request")
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Simulate receiving a status message update
	testStatusMessage := "Feeling happy today!"
	tox1.receiveFriendStatusMessageUpdate(friendID, testStatusMessage)

	// Verify callback was invoked with correct parameters
	callbackMu.Lock()
	defer callbackMu.Unlock()

	if !callbackInvoked {
		t.Error("Expected OnFriendStatusMessage callback to be invoked, but it wasn't")
	}

	if receivedFriendID != friendID {
		t.Errorf("Expected friendID %d in callback, got %d", friendID, receivedFriendID)
	}

	if receivedStatusMessage != testStatusMessage {
		t.Errorf("Expected status message '%s' in callback, got '%s'", testStatusMessage, receivedStatusMessage)
	}
}

// TestOnFriendStatusMessage_NoCallbackSet verifies that no panic occurs when
// receiving status message updates without a callback set
func TestOnFriendStatusMessage_NoCallbackSet(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create second instance to get a valid address
	options2 := NewOptions()
	options2.UDPEnabled = false
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	addr2 := tox2.SelfGetAddress()
	friendID, err := tox.AddFriend(addr2, "Test")
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// This should not panic even without a callback set
	tox.receiveFriendStatusMessageUpdate(friendID, "Testing without callback")

	// If we get here without panic, test passes
}

// TestOnFriendStatusMessage_CallbackThreadSafety verifies that the callback
// system is thread-safe
func TestOnFriendStatusMessage_CallbackThreadSafety(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Counter for callback invocations
	var callbackCount int
	var mu sync.Mutex

	callback := func(friendID uint32, statusMessage string) {
		mu.Lock()
		defer mu.Unlock()
		callbackCount++
	}

	// Create second instance to get a valid address for adding as friend
	options2 := NewOptions()
	options2.UDPEnabled = false
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	addr := tox2.SelfGetAddress()
	friendID, err := tox.AddFriend(addr, "Test")
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Register callback
	tox.OnFriendStatusMessage(callback)

	// Simulate concurrent status message updates
	var wg sync.WaitGroup
	numUpdates := 10

	for i := 0; i < numUpdates; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			tox.receiveFriendStatusMessageUpdate(friendID, "Status update")
		}(i)
	}

	wg.Wait()

	// Allow brief time for all callbacks to complete
	time.Sleep(50 * time.Millisecond)

	// Verify all callbacks were invoked
	mu.Lock()
	defer mu.Unlock()

	if callbackCount != numUpdates {
		t.Errorf("Expected %d callback invocations, got %d", numUpdates, callbackCount)
	}
}

// TestOnFriendStatusMessage_OversizedStatusMessage verifies that oversized
// status messages are rejected and don't invoke the callback
func TestOnFriendStatusMessage_OversizedStatusMessage(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Track callback invocations
	var callbackInvoked bool
	var mu sync.Mutex

	tox.OnFriendStatusMessage(func(friendID uint32, statusMessage string) {
		mu.Lock()
		defer mu.Unlock()
		callbackInvoked = true
	})

	// Create second instance to get a valid address
	options2 := NewOptions()
	options2.UDPEnabled = false
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	addr := tox2.SelfGetAddress()
	friendID, err := tox.AddFriend(addr, "Test")
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Create an oversized status message (>1007 bytes)
	oversizedMessage := make([]byte, 1008)
	for i := range oversizedMessage {
		oversizedMessage[i] = 'A'
	}

	// Attempt to receive oversized status message
	tox.receiveFriendStatusMessageUpdate(friendID, string(oversizedMessage))

	// Brief wait to ensure callback wouldn't be invoked
	time.Sleep(10 * time.Millisecond)

	// Verify callback was NOT invoked
	mu.Lock()
	defer mu.Unlock()

	if callbackInvoked {
		t.Error("Expected callback NOT to be invoked for oversized status message, but it was")
	}
}

// TestOnFriendStatusMessage_UnknownFriend verifies that status message updates
// from unknown friends are ignored and don't invoke the callback
func TestOnFriendStatusMessage_UnknownFriend(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Track callback invocations
	var callbackInvoked bool
	var mu sync.Mutex

	tox.OnFriendStatusMessage(func(friendID uint32, statusMessage string) {
		mu.Lock()
		defer mu.Unlock()
		callbackInvoked = true
	})

	// Attempt to receive status message update from non-existent friend
	nonExistentFriendID := uint32(99999)
	tox.receiveFriendStatusMessageUpdate(nonExistentFriendID, "Ghost message")

	// Brief wait to ensure callback wouldn't be invoked
	time.Sleep(10 * time.Millisecond)

	// Verify callback was NOT invoked
	mu.Lock()
	defer mu.Unlock()

	if callbackInvoked {
		t.Error("Expected callback NOT to be invoked for unknown friend, but it was")
	}
}

// TestOnFriendStatusMessage_ValidStatusMessage verifies that valid status messages
// are properly stored and trigger the callback
func TestOnFriendStatusMessage_ValidStatusMessage(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Track callback data
	var receivedStatusMessage string
	var mu sync.Mutex

	tox.OnFriendStatusMessage(func(friendID uint32, statusMessage string) {
		mu.Lock()
		defer mu.Unlock()
		receivedStatusMessage = statusMessage
	})

	// Create second instance to get a valid address
	options2 := NewOptions()
	options2.UDPEnabled = false
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	addr := tox2.SelfGetAddress()
	friendID, err := tox.AddFriend(addr, "Test")
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Test various valid status messages
	testCases := []struct {
		name          string
		statusMessage string
	}{
		{"Empty string", ""},
		{"Short message", "Hi!"},
		{"Medium message", "Working on an interesting project today"},
		{"Long message", "This is a longer status message that contains multiple sentences. It should still be under the 1007 byte limit and should be properly stored and forwarded to the callback."},
		{"Unicode message", "Hello 世界 🌍"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset received message
			mu.Lock()
			receivedStatusMessage = ""
			mu.Unlock()

			// Receive status message update
			tox.receiveFriendStatusMessageUpdate(friendID, tc.statusMessage)

			// Brief wait for callback
			time.Sleep(10 * time.Millisecond)

			// Verify callback received correct message
			mu.Lock()
			defer mu.Unlock()

			if receivedStatusMessage != tc.statusMessage {
				t.Errorf("Expected status message '%s', got '%s'", tc.statusMessage, receivedStatusMessage)
			}

			// Also verify it was stored on the friend object
			tox.friendsMutex.RLock()
			friend := tox.friends[friendID]
			tox.friendsMutex.RUnlock()

			if friend.StatusMessage != tc.statusMessage {
				t.Errorf("Expected stored status message '%s', got '%s'", tc.statusMessage, friend.StatusMessage)
			}
		})
	}
}

// TestOnFriendStatusMessage_CallbackReplacement verifies that setting a new callback
// properly replaces the old one
func TestOnFriendStatusMessage_CallbackReplacement(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create second instance to get a valid address
	options2 := NewOptions()
	options2.UDPEnabled = false
	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	addr := tox2.SelfGetAddress()
	friendID, err := tox.AddFriend(addr, "Test")
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Track callback invocations
	var firstCallbackInvoked bool
	var secondCallbackInvoked bool
	var mu sync.Mutex

	// Set first callback
	tox.OnFriendStatusMessage(func(friendID uint32, statusMessage string) {
		mu.Lock()
		defer mu.Unlock()
		firstCallbackInvoked = true
	})

	// Replace with second callback
	tox.OnFriendStatusMessage(func(friendID uint32, statusMessage string) {
		mu.Lock()
		defer mu.Unlock()
		secondCallbackInvoked = true
	})

	// Trigger status message update
	tox.receiveFriendStatusMessageUpdate(friendID, "Test message")

	// Brief wait for callback
	time.Sleep(10 * time.Millisecond)

	// Verify only second callback was invoked
	mu.Lock()
	defer mu.Unlock()

	if firstCallbackInvoked {
		t.Error("First callback should not be invoked after replacement")
	}

	if !secondCallbackInvoked {
		t.Error("Second callback should be invoked")
	}
}

package toxcore

import (
	"sync"
	"testing"
	"time"
)

// TestOnFriendTyping_CallbackInvoked verifies that the OnFriendTyping callback
// is invoked when a friend sends typing notifications
func TestOnFriendTyping_CallbackInvoked(t *testing.T) {
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
	var receivedIsTyping bool
	var callbackInvoked bool

	tox1.OnFriendTyping(func(friendID uint32, isTyping bool) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		receivedFriendID = friendID
		receivedIsTyping = isTyping
		callbackInvoked = true
	})

	// Add tox2 as friend on tox1
	addr2 := tox2.SelfGetAddress()
	friendID, err := tox1.AddFriend(addr2, "Test friend request")
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Simulate receiving a typing notification
	tox1.receiveFriendTyping(friendID, true)

	// Verify callback was invoked with correct parameters
	callbackMu.Lock()
	defer callbackMu.Unlock()

	if !callbackInvoked {
		t.Error("Expected OnFriendTyping callback to be invoked, but it wasn't")
	}

	if receivedFriendID != friendID {
		t.Errorf("Expected friendID %d in callback, got %d", friendID, receivedFriendID)
	}

	if receivedIsTyping != true {
		t.Errorf("Expected isTyping true in callback, got %v", receivedIsTyping)
	}
}

// TestOnFriendTyping_NoCallbackSet verifies that no panic occurs when
// receiving typing notifications without a callback set
func TestOnFriendTyping_NoCallbackSet(t *testing.T) {
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
	tox.receiveFriendTyping(friendID, true)

	// If we get here without panic, test passes
}

// TestOnFriendTyping_CallbackThreadSafety verifies that the callback
// is thread-safe when multiple typing notifications arrive concurrently
func TestOnFriendTyping_CallbackThreadSafety(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create multiple friends
	var friendIDs []uint32
	for i := 0; i < 5; i++ {
		tox2, err := New(NewOptions())
		if err != nil {
			t.Fatalf("Failed to create Tox instance %d: %v", i, err)
		}
		defer tox2.Kill()

		addr := tox2.SelfGetAddress()
		friendID, err := tox.AddFriend(addr, "Test")
		if err != nil {
			t.Fatalf("Failed to add friend %d: %v", i, err)
		}
		friendIDs = append(friendIDs, friendID)
	}

	// Track callback invocations
	var callbackMu sync.Mutex
	callbackCount := 0
	callback := func(friendID uint32, isTyping bool) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackCount++
	}

	tox.OnFriendTyping(callback)

	// Send concurrent typing notifications
	var wg sync.WaitGroup
	for _, friendID := range friendIDs {
		wg.Add(1)
		go func(fid uint32) {
			defer wg.Done()
			tox.receiveFriendTyping(fid, true)
			time.Sleep(10 * time.Millisecond)
			tox.receiveFriendTyping(fid, false)
		}(friendID)
	}

	wg.Wait()

	// Verify all callbacks were invoked
	callbackMu.Lock()
	defer callbackMu.Unlock()

	expectedCount := len(friendIDs) * 2 // Each friend sends 2 notifications (true, false)
	if callbackCount != expectedCount {
		t.Errorf("Expected %d callback invocations, got %d", expectedCount, callbackCount)
	}
}

// TestOnFriendTyping_UnknownFriend verifies that typing notifications
// from unknown friends are handled gracefully
func TestOnFriendTyping_UnknownFriend(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	var callbackInvoked bool
	tox.OnFriendTyping(func(friendID uint32, isTyping bool) {
		callbackInvoked = true
	})

	// Send typing notification from non-existent friend
	tox.receiveFriendTyping(9999, true)

	// Callback should not be invoked for unknown friends
	if callbackInvoked {
		t.Error("Callback should not be invoked for unknown friend")
	}
}

// TestOnFriendTyping_StateTransitions verifies that typing state transitions
// are correctly tracked and reported
func TestOnFriendTyping_StateTransitions(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	tox2, err := New(NewOptions())
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	addr2 := tox2.SelfGetAddress()
	friendID, err := tox.AddFriend(addr2, "Test")
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Track state transitions
	var callbackMu sync.Mutex
	var transitions []bool

	tox.OnFriendTyping(func(fid uint32, isTyping bool) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		transitions = append(transitions, isTyping)
	})

	// Test state transitions: false -> true -> false -> true -> false
	testSequence := []bool{false, true, false, true, false}
	for _, state := range testSequence {
		tox.receiveFriendTyping(friendID, state)
	}

	// Verify all transitions were recorded
	callbackMu.Lock()
	defer callbackMu.Unlock()

	if len(transitions) != len(testSequence) {
		t.Errorf("Expected %d transitions, got %d", len(testSequence), len(transitions))
	}

	for i, expected := range testSequence {
		if i >= len(transitions) {
			break
		}
		if transitions[i] != expected {
			t.Errorf("Transition %d: expected %v, got %v", i, expected, transitions[i])
		}
	}

	// Verify final state in Friend struct
	tox.friendsMutex.RLock()
	friend := tox.friends[friendID]
	tox.friendsMutex.RUnlock()

	if friend.IsTyping != false {
		t.Errorf("Expected final IsTyping state to be false, got %v", friend.IsTyping)
	}
}

// TestOnFriendTyping_CallbackReplacement verifies that setting a new callback
// replaces the old one and only the new callback is invoked
func TestOnFriendTyping_CallbackReplacement(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	tox2, err := New(NewOptions())
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	addr2 := tox2.SelfGetAddress()
	friendID, err := tox.AddFriend(addr2, "Test")
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set first callback
	var firstCallbackInvoked bool
	tox.OnFriendTyping(func(fid uint32, isTyping bool) {
		firstCallbackInvoked = true
	})

	// Replace with second callback
	var secondCallbackInvoked bool
	tox.OnFriendTyping(func(fid uint32, isTyping bool) {
		secondCallbackInvoked = true
	})

	// Send typing notification
	tox.receiveFriendTyping(friendID, true)

	// Only second callback should be invoked
	if firstCallbackInvoked {
		t.Error("First callback should not be invoked after replacement")
	}

	if !secondCallbackInvoked {
		t.Error("Second callback should be invoked")
	}
}

// TestSetTyping_BasicFunctionality verifies that SetTyping sends
// typing notifications correctly
func TestSetTyping_BasicFunctionality(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	tox2, err := New(NewOptions())
	if err != nil {
		t.Fatalf("Failed to create second Tox instance: %v", err)
	}
	defer tox2.Kill()

	addr2 := tox2.SelfGetAddress()
	friendID, err := tox.AddFriend(addr2, "Test")
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Attempt to set typing for offline friend - should fail
	err = tox.SetTyping(friendID, true)
	if err == nil {
		t.Error("Expected error when setting typing for offline friend, got nil")
	}
}

// TestSetTyping_UnknownFriend verifies that SetTyping returns error
// for unknown friends
func TestSetTyping_UnknownFriend(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Attempt to set typing for non-existent friend
	err = tox.SetTyping(9999, true)
	if err == nil {
		t.Error("Expected error when setting typing for unknown friend, got nil")
	}

	expectedError := "friend not found"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

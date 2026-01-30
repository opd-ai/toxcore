package toxcore

import (
	"sync"
	"testing"
	"time"
)

// TestOnFriendConnectionStatus verifies the friend connection status callback is triggered
func TestOnFriendConnectionStatus(t *testing.T) {
	tox, err := New(NewOptionsForTesting())
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a friend
	pubKey := [32]byte{1, 2, 3}
	friendID, err := tox.AddFriendByPublicKey(pubKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set up callback to track connection status changes
	var mu sync.Mutex
	var callbackInvoked bool
	var receivedFriendID uint32
	var receivedStatus ConnectionStatus

	tox.OnFriendConnectionStatus(func(friendID uint32, status ConnectionStatus) {
		mu.Lock()
		defer mu.Unlock()
		callbackInvoked = true
		receivedFriendID = friendID
		receivedStatus = status
	})

	// Change friend connection status to UDP
	err = tox.SetFriendConnectionStatus(friendID, ConnectionUDP)
	if err != nil {
		t.Fatalf("Failed to set friend connection status: %v", err)
	}

	// Give callback time to fire
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !callbackInvoked {
		t.Error("OnFriendConnectionStatus callback was not invoked")
	}

	if receivedFriendID != friendID {
		t.Errorf("Expected friend ID %d, got %d", friendID, receivedFriendID)
	}

	if receivedStatus != ConnectionUDP {
		t.Errorf("Expected status ConnectionUDP, got %v", receivedStatus)
	}
}

// TestOnFriendConnectionStatusMultipleChanges tests multiple status transitions
func TestOnFriendConnectionStatusMultipleChanges(t *testing.T) {
	tox, err := New(NewOptionsForTesting())
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	pubKey := [32]byte{4, 5, 6}
	friendID, err := tox.AddFriendByPublicKey(pubKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Track all status changes
	var mu sync.Mutex
	var statusChanges []ConnectionStatus

	tox.OnFriendConnectionStatus(func(fid uint32, status ConnectionStatus) {
		mu.Lock()
		defer mu.Unlock()
		if fid == friendID {
			statusChanges = append(statusChanges, status)
		}
	})

	// Perform multiple status changes
	transitions := []ConnectionStatus{
		ConnectionUDP,
		ConnectionTCP,
		ConnectionNone,
		ConnectionUDP,
	}

	for _, status := range transitions {
		err := tox.SetFriendConnectionStatus(friendID, status)
		if err != nil {
			t.Fatalf("Failed to set connection status to %v: %v", status, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(statusChanges) != len(transitions) {
		t.Errorf("Expected %d status changes, got %d", len(transitions), len(statusChanges))
	}

	for i, expected := range transitions {
		if i < len(statusChanges) && statusChanges[i] != expected {
			t.Errorf("Status change %d: expected %v, got %v", i, expected, statusChanges[i])
		}
	}
}

// TestOnFriendStatusChange verifies the online/offline status change callback
func TestOnFriendStatusChange(t *testing.T) {
	tox, err := New(NewOptionsForTesting())
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	pubKey := [32]byte{7, 8, 9}
	friendID, err := tox.AddFriendByPublicKey(pubKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set up callback to track online/offline changes
	var mu sync.Mutex
	var callbackInvoked bool
	var receivedPubKey [32]byte
	var receivedOnline bool

	tox.OnFriendStatusChange(func(pk [32]byte, online bool) {
		mu.Lock()
		defer mu.Unlock()
		callbackInvoked = true
		receivedPubKey = pk
		receivedOnline = online
	})

	// Transition friend from offline to online
	err = tox.SetFriendConnectionStatus(friendID, ConnectionUDP)
	if err != nil {
		t.Fatalf("Failed to set friend connection status: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !callbackInvoked {
		t.Error("OnFriendStatusChange callback was not invoked")
	}

	if receivedPubKey != pubKey {
		t.Errorf("Expected public key %v, got %v", pubKey, receivedPubKey)
	}

	if !receivedOnline {
		t.Error("Expected online=true when transitioning to ConnectionUDP")
	}
}

// TestOnFriendStatusChangeOnlineOfflineTransitions tests both directions of status change
func TestOnFriendStatusChangeOnlineOfflineTransitions(t *testing.T) {
	tox, err := New(NewOptionsForTesting())
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	pubKey := [32]byte{10, 11, 12}
	friendID, err := tox.AddFriendByPublicKey(pubKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Track online/offline transitions
	var mu sync.Mutex
	var transitions []bool

	tox.OnFriendStatusChange(func(pk [32]byte, online bool) {
		mu.Lock()
		defer mu.Unlock()
		if pk == pubKey {
			transitions = append(transitions, online)
		}
	})

	// Go online
	err = tox.SetFriendConnectionStatus(friendID, ConnectionTCP)
	if err != nil {
		t.Fatalf("Failed to set online: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	// Go offline
	err = tox.SetFriendConnectionStatus(friendID, ConnectionNone)
	if err != nil {
		t.Fatalf("Failed to set offline: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	// Go online again
	err = tox.SetFriendConnectionStatus(friendID, ConnectionUDP)
	if err != nil {
		t.Fatalf("Failed to set online again: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	expectedTransitions := []bool{true, false, true}
	if len(transitions) != len(expectedTransitions) {
		t.Fatalf("Expected %d transitions, got %d", len(expectedTransitions), len(transitions))
	}

	for i, expected := range expectedTransitions {
		if transitions[i] != expected {
			t.Errorf("Transition %d: expected online=%v, got %v", i, expected, transitions[i])
		}
	}
}

// TestOnFriendStatusChangeNoCallbackOnSameStatus verifies callback isn't triggered for UDP->TCP
func TestOnFriendStatusChangeNoCallbackOnSameStatus(t *testing.T) {
	tox, err := New(NewOptionsForTesting())
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	pubKey := [32]byte{13, 14, 15}
	friendID, err := tox.AddFriendByPublicKey(pubKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	var mu sync.Mutex
	var callCount int

	tox.OnFriendStatusChange(func(pk [32]byte, online bool) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
	})

	// Go online with UDP
	err = tox.SetFriendConnectionStatus(friendID, ConnectionUDP)
	if err != nil {
		t.Fatalf("Failed to set ConnectionUDP: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	// Switch to TCP (still online, should not trigger OnFriendStatusChange)
	err = tox.SetFriendConnectionStatus(friendID, ConnectionTCP)
	if err != nil {
		t.Fatalf("Failed to set ConnectionTCP: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should only be called once (for the initial online transition)
	if callCount != 1 {
		t.Errorf("Expected OnFriendStatusChange to be called 1 time, got %d", callCount)
	}
}

// TestBothCallbacksTogether verifies both callbacks work independently
func TestBothCallbacksTogether(t *testing.T) {
	tox, err := New(NewOptionsForTesting())
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	pubKey := [32]byte{16, 17, 18}
	friendID, err := tox.AddFriendByPublicKey(pubKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	var mu sync.Mutex
	var connectionStatusCalls int
	var statusChangeCalls int

	tox.OnFriendConnectionStatus(func(fid uint32, status ConnectionStatus) {
		mu.Lock()
		defer mu.Unlock()
		connectionStatusCalls++
	})

	tox.OnFriendStatusChange(func(pk [32]byte, online bool) {
		mu.Lock()
		defer mu.Unlock()
		statusChangeCalls++
	})

	// Transition: None -> UDP (should trigger both)
	err = tox.SetFriendConnectionStatus(friendID, ConnectionUDP)
	if err != nil {
		t.Fatalf("Failed to set ConnectionUDP: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	// Transition: UDP -> TCP (should trigger only OnFriendConnectionStatus)
	err = tox.SetFriendConnectionStatus(friendID, ConnectionTCP)
	if err != nil {
		t.Fatalf("Failed to set ConnectionTCP: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	// Transition: TCP -> None (should trigger both)
	err = tox.SetFriendConnectionStatus(friendID, ConnectionNone)
	if err != nil {
		t.Fatalf("Failed to set ConnectionNone: %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// OnFriendConnectionStatus should be called 3 times (all transitions)
	if connectionStatusCalls != 3 {
		t.Errorf("Expected OnFriendConnectionStatus 3 calls, got %d", connectionStatusCalls)
	}

	// OnFriendStatusChange should be called 2 times (offline->online, online->offline)
	if statusChangeCalls != 2 {
		t.Errorf("Expected OnFriendStatusChange 2 calls, got %d", statusChangeCalls)
	}
}

// TestCallbacksNotCalledForNonexistentFriend ensures callbacks aren't triggered for invalid friend IDs
func TestCallbacksNotCalledForNonexistentFriend(t *testing.T) {
	tox, err := New(NewOptionsForTesting())
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	var mu sync.Mutex
	var connStatusCalled bool
	var statusChangeCalled bool

	tox.OnFriendConnectionStatus(func(fid uint32, status ConnectionStatus) {
		mu.Lock()
		defer mu.Unlock()
		connStatusCalled = true
	})

	tox.OnFriendStatusChange(func(pk [32]byte, online bool) {
		mu.Lock()
		defer mu.Unlock()
		statusChangeCalled = true
	})

	// Try to set status for non-existent friend
	err = tox.SetFriendConnectionStatus(999, ConnectionUDP)
	if err == nil {
		t.Error("Expected error when setting status for non-existent friend")
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if connStatusCalled {
		t.Error("OnFriendConnectionStatus should not be called for non-existent friend")
	}

	if statusChangeCalled {
		t.Error("OnFriendStatusChange should not be called for non-existent friend")
	}
}

// TestCallbacksClearedOnKill verifies callbacks are properly cleared
func TestCallbacksClearedOnKill(t *testing.T) {
	tox, err := New(NewOptionsForTesting())
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	// Set callbacks
	tox.OnFriendConnectionStatus(func(fid uint32, status ConnectionStatus) {})
	tox.OnFriendStatusChange(func(pk [32]byte, online bool) {})

	// Verify callbacks are set
	if tox.friendConnectionStatusCallback == nil {
		t.Error("friendConnectionStatusCallback should be set before Kill()")
	}
	if tox.friendStatusChangeCallback == nil {
		t.Error("friendStatusChangeCallback should be set before Kill()")
	}

	// Kill the instance
	tox.Kill()

	// Verify callbacks are cleared
	if tox.friendConnectionStatusCallback != nil {
		t.Error("friendConnectionStatusCallback should be nil after Kill()")
	}
	if tox.friendStatusChangeCallback != nil {
		t.Error("friendStatusChangeCallback should be nil after Kill()")
	}
}

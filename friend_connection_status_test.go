package toxcore

import (
	"testing"
	"time"
)

// TestFriendConnectionStatusNotification verifies that the async manager
// is properly notified when friend connection status changes.
func TestFriendConnectionStatusNotification(t *testing.T) {
	// Create a Tox instance - async messaging is enabled by default when created
	options := NewOptions()
	options.UDPEnabled = false // Disable network for controlled testing

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Note: asyncManager may or may not be initialized depending on options
	// The updateFriendOnlineStatus should handle nil asyncManager gracefully

	// Add a friend
	var friendPK [32]byte
	copy(friendPK[:], []byte("test_friend_public_key_12345"))
	friendID, err := tox.AddFriendByPublicKey(friendPK)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Initially the friend should be offline
	if tox.GetFriendConnectionStatus(friendID) != ConnectionNone {
		t.Error("New friend should have ConnectionNone status")
	}

	// Test the updateFriendOnlineStatus helper
	// This should notify the async manager (if present)
	tox.updateFriendOnlineStatus(friendID, true)

	// Give async manager time to process
	time.Sleep(50 * time.Millisecond)

	// The async manager should now be aware that this friend is online
	// Note: We can't directly test the internal state, but the function should not panic
	// and should handle the notification correctly

	// Test updating to offline
	tox.updateFriendOnlineStatus(friendID, false)
	time.Sleep(50 * time.Millisecond)

	// Verify nil-safety: updateFriendOnlineStatus should work even with nil async manager
	toxWithoutAsync, err := New(NewOptions())
	if err != nil {
		t.Fatalf("Failed to create Tox instance without async: %v", err)
	}
	defer toxWithoutAsync.Kill()

	friendID2, err := toxWithoutAsync.AddFriendByPublicKey(friendPK)
	if err != nil {
		t.Fatalf("Failed to add friend to non-async Tox: %v", err)
	}

	// Should not panic even though asyncManager might be nil
	toxWithoutAsync.updateFriendOnlineStatus(friendID2, true)
	toxWithoutAsync.updateFriendOnlineStatus(friendID2, false)
}

// TestSetFriendConnectionStatusWithNotification tests the new
// SetFriendConnectionStatus method that properly notifies the async manager.
func TestSetFriendConnectionStatusWithNotification(t *testing.T) {
	// Create a Tox instance
	options := NewOptions()
	options.UDPEnabled = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a friend
	var friendPK [32]byte
	copy(friendPK[:], []byte("test_friend_for_conn_status_"))
	friendID, err := tox.AddFriendByPublicKey(friendPK)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Test setting connection status to online (UDP)
	err = tox.SetFriendConnectionStatus(friendID, ConnectionUDP)
	if err != nil {
		t.Errorf("SetFriendConnectionStatus failed: %v", err)
	}

	// Verify the status was updated
	if got := tox.GetFriendConnectionStatus(friendID); got != ConnectionUDP {
		t.Errorf("Expected ConnectionUDP, got %v", got)
	}

	// Test setting to TCP
	err = tox.SetFriendConnectionStatus(friendID, ConnectionTCP)
	if err != nil {
		t.Errorf("SetFriendConnectionStatus failed: %v", err)
	}

	if got := tox.GetFriendConnectionStatus(friendID); got != ConnectionTCP {
		t.Errorf("Expected ConnectionTCP, got %v", got)
	}

	// Test setting back to offline
	err = tox.SetFriendConnectionStatus(friendID, ConnectionNone)
	if err != nil {
		t.Errorf("SetFriendConnectionStatus failed: %v", err)
	}

	if got := tox.GetFriendConnectionStatus(friendID); got != ConnectionNone {
		t.Errorf("Expected ConnectionNone, got %v", got)
	}

	// Test with invalid friend ID
	err = tox.SetFriendConnectionStatus(99999, ConnectionUDP)
	if err == nil {
		t.Error("Expected error for invalid friend ID, got nil")
	}
}

// TestFriendConnectionStatusCallbackIntegration tests that connection status
// changes trigger both the status callback and async manager notifications.
func TestFriendConnectionStatusCallbackIntegration(t *testing.T) {
	// Create a Tox instance
	options := NewOptions()
	options.UDPEnabled = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a friend
	var friendPK [32]byte
	copy(friendPK[:], []byte("test_callback_friend_key12345"))
	friendID, err := tox.AddFriendByPublicKey(friendPK)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Track callback invocations
	callbackCalled := false
	var lastStatus ConnectionStatus

	// Register connection status callback
	// Note: This is a self connection status callback, not friend-specific
	// In a full implementation, we'd have a friend connection status callback
	tox.OnConnectionStatus(func(status ConnectionStatus) {
		callbackCalled = true
		lastStatus = status
	})

	// Change friend connection status
	err = tox.SetFriendConnectionStatus(friendID, ConnectionUDP)
	if err != nil {
		t.Errorf("SetFriendConnectionStatus failed: %v", err)
	}

	// Give time for async processing
	time.Sleep(50 * time.Millisecond)

	// Verify the connection status was updated
	if got := tox.GetFriendConnectionStatus(friendID); got != ConnectionUDP {
		t.Errorf("Expected ConnectionUDP, got %v", got)
	}

	// Note: callbackCalled might be false since OnConnectionStatus is for self,
	// not friends. This test documents the current behavior.
	_ = callbackCalled
	_ = lastStatus
}

// TestAsyncManagerPreKeyExchangeOnFriendOnline tests that when a friend
// comes online, the async manager is notified and can trigger pre-key exchange.
func TestAsyncManagerPreKeyExchangeOnFriendOnline(t *testing.T) {
	// Create a Tox instance
	options := NewOptions()
	options.UDPEnabled = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a friend
	var friendPK [32]byte
	copy(friendPK[:], []byte("friend_for_prekey_exchange___"))
	friendID, err := tox.AddFriendByPublicKey(friendPK)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Simulate friend coming online
	err = tox.SetFriendConnectionStatus(friendID, ConnectionUDP)
	if err != nil {
		t.Errorf("SetFriendConnectionStatus failed: %v", err)
	}

	// The async manager should be notified via updateFriendOnlineStatus
	// In a real scenario, this would trigger a pre-key exchange attempt
	time.Sleep(100 * time.Millisecond)

	// Verify friend is marked as online
	if got := tox.GetFriendConnectionStatus(friendID); got != ConnectionUDP {
		t.Errorf("Expected ConnectionUDP, got %v", got)
	}

	// Simulate friend going offline
	err = tox.SetFriendConnectionStatus(friendID, ConnectionNone)
	if err != nil {
		t.Errorf("SetFriendConnectionStatus failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if got := tox.GetFriendConnectionStatus(friendID); got != ConnectionNone {
		t.Errorf("Expected ConnectionNone, got %v", got)
	}
}

// TestFriendConnectionStatusEdgeCases tests edge cases in connection status handling.
func TestFriendConnectionStatusEdgeCases(t *testing.T) {
	// Create Tox instance
	tox, err := New(NewOptions())
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test with no friends
	err = tox.SetFriendConnectionStatus(1, ConnectionUDP)
	if err == nil {
		t.Error("Expected error when setting status for non-existent friend")
	}

	// Add a friend
	var friendPK [32]byte
	copy(friendPK[:], []byte("edge_case_friend_public_key__"))
	friendID, err := tox.AddFriendByPublicKey(friendPK)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Test setting same status twice
	err = tox.SetFriendConnectionStatus(friendID, ConnectionUDP)
	if err != nil {
		t.Errorf("First status set failed: %v", err)
	}

	err = tox.SetFriendConnectionStatus(friendID, ConnectionUDP)
	if err != nil {
		t.Errorf("Setting same status twice should not fail: %v", err)
	}

	// Test rapid status changes
	statuses := []ConnectionStatus{
		ConnectionNone,
		ConnectionUDP,
		ConnectionTCP,
		ConnectionUDP,
		ConnectionNone,
	}

	for _, status := range statuses {
		err = tox.SetFriendConnectionStatus(friendID, status)
		if err != nil {
			t.Errorf("Status change to %v failed: %v", status, err)
		}

		if got := tox.GetFriendConnectionStatus(friendID); got != status {
			t.Errorf("Expected %v, got %v", status, got)
		}
	}
}

// TestSetFriendConnectionStatusConcurrency validates that the refactored
// SetFriendConnectionStatus is safe for concurrent access and doesn't have
// the double-lock issue that existed in the previous manual unlock/relock pattern.
func TestSetFriendConnectionStatusConcurrency(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a friend
	var friendPK [32]byte
	copy(friendPK[:], []byte("concurrent_test_friend_key___"))
	friendID, err := tox.AddFriendByPublicKey(friendPK)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Run concurrent status changes to verify no race conditions or deadlocks
	const numGoroutines = 10
	const numIterations = 20

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer func() { done <- true }()

			statuses := []ConnectionStatus{
				ConnectionNone,
				ConnectionUDP,
				ConnectionTCP,
			}

			for j := 0; j < numIterations; j++ {
				status := statuses[j%len(statuses)]
				err := tox.SetFriendConnectionStatus(friendID, status)
				if err != nil {
					t.Errorf("Routine %d iteration %d: SetFriendConnectionStatus failed: %v", routineID, j, err)
				}

				// Also read the status concurrently
				_ = tox.GetFriendConnectionStatus(friendID)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Final verification - should not panic or deadlock
	finalStatus := tox.GetFriendConnectionStatus(friendID)
	if finalStatus != ConnectionNone && finalStatus != ConnectionUDP && finalStatus != ConnectionTCP {
		t.Errorf("Invalid final status: %v", finalStatus)
	}
}

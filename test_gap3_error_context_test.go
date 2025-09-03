package toxcore

import (
	"strings"
	"testing"
)

// TestGap3SendFriendMessageErrorContext verifies that SendFriendMessage
// provides clear error messages when a friend is not connected. This is a
// regression test to ensure error messages remain user-friendly.
func TestGap3SendFriendMessageErrorContext(t *testing.T) {
	// Create a Tox instance for testing
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a friend but leave them disconnected
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Test sending to disconnected friend
	err = tox.SendFriendMessage(friendID, "Hello")
	if err == nil {
		t.Error("Expected error when sending to disconnected friend")
		return
	}

	// Verify the error message provides clear connection context
	errorMsg := err.Error()
	t.Logf("Error message: %s", errorMsg)
	
	// The error should clearly indicate the connection issue
	if !strings.Contains(errorMsg, "friend is not connected") {
		t.Errorf("Error message should mention 'friend is not connected', got: %s", errorMsg)
	}
	
	// The error should still include the underlying technical details for debugging
	if !strings.Contains(errorMsg, "no pre-keys available") {
		t.Errorf("Error message should still include technical details for debugging, got: %s", errorMsg)
	}
}

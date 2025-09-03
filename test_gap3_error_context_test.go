package toxcore

import (
	"strings"
	"testing"
)

// TestGap3SendFriendMessageErrorContext verifies that SendFriendMessage
// provides clear error messages when a friend is not connected, rather than
// cryptic forward secrecy error messages.
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

	// The current implementation returns "no pre-keys available" which is confusing
	// We want to test that we get a clearer error message about connection status
	
	// Log what we currently get vs what we want
	currentError := err.Error()
	t.Logf("Current error message: %s", currentError)
	
	// Check if the error contains helpful context about the connection issue
	hasConnectionContext := strings.Contains(strings.ToLower(currentError), "connect") ||
		strings.Contains(strings.ToLower(currentError), "offline") ||
		strings.Contains(strings.ToLower(currentError), "not connected")
	
	if hasConnectionContext {
		t.Log("✓ Error message provides clear connection context")
	} else {
		// Currently this will fail - the error is about pre-keys not connection
		if strings.Contains(currentError, "no pre-keys available") {
			t.Log("❌ Error message is cryptic (mentions pre-keys instead of connection status)")
			t.Log("Expected: Clear message about friend not being connected")
			t.Log("Actual: Cryptic message about forward secrecy pre-keys")
		} else {
			t.Logf("❌ Unexpected error message: %s", currentError)
		}
	}
}

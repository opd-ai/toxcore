package toxcore

import (
	"testing"
)

// TestRequestManagerIntegration verifies that the RequestManager is properly integrated
// into the Tox struct and functions correctly during friend request processing.
func TestRequestManagerIntegration(t *testing.T) {
	// Create a Tox instance
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify RequestManager is initialized
	if tox.RequestManager() == nil {
		t.Fatal("RequestManager should be initialized")
	}

	// Track both callback invocation and request manager state
	callbackInvoked := false
	var callbackPublicKey [32]byte
	var callbackMessage string

	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		callbackInvoked = true
		callbackPublicKey = publicKey
		callbackMessage = message
	})

	// Simulate receiving a friend request
	var senderKey [32]byte
	copy(senderKey[:], []byte("test_sender_public_key_12345678"))
	testMessage := "Hello, let's be friends!"

	// Call receiveFriendRequest directly to test the integration
	tox.receiveFriendRequest(senderKey, testMessage)

	// Verify callback was invoked
	if !callbackInvoked {
		t.Error("FriendRequestCallback should have been invoked")
	}
	if callbackPublicKey != senderKey {
		t.Error("Callback received wrong public key")
	}
	if callbackMessage != testMessage {
		t.Error("Callback received wrong message")
	}

	// Verify RequestManager tracked the request
	pendingRequests := tox.RequestManager().GetPendingRequests()
	if len(pendingRequests) != 1 {
		t.Fatalf("Expected 1 pending request in RequestManager, got %d", len(pendingRequests))
	}

	// Verify request details in RequestManager
	req := pendingRequests[0]
	if req.SenderPublicKey != senderKey {
		t.Error("RequestManager stored wrong sender public key")
	}
	if req.Message != testMessage {
		t.Error("RequestManager stored wrong message")
	}
}

// TestRequestManagerCleanup verifies that the RequestManager is properly cleaned up
// when Kill() is called.
func TestRequestManagerCleanup(t *testing.T) {
	// Create a Tox instance
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	// Verify RequestManager exists
	if tox.RequestManager() == nil {
		t.Fatal("RequestManager should be initialized")
	}

	// Kill the instance
	tox.Kill()

	// Verify RequestManager is cleaned up
	if tox.RequestManager() != nil {
		t.Error("RequestManager should be nil after Kill()")
	}
}

// TestRequestManagerDuplicateHandling verifies that duplicate friend requests
// are properly handled through the RequestManager integration.
func TestRequestManagerDuplicateHandling(t *testing.T) {
	// Create a Tox instance
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	callbackCount := 0
	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		callbackCount++
	})

	// Send same request twice
	var senderKey [32]byte
	copy(senderKey[:], []byte("duplicate_test_sender_key_123456"))

	tox.receiveFriendRequest(senderKey, "First message")
	tox.receiveFriendRequest(senderKey, "Updated message")

	// Callback should be invoked twice (application needs both notifications)
	if callbackCount != 2 {
		t.Errorf("Expected callback to be invoked 2 times, got %d", callbackCount)
	}

	// RequestManager should only have one pending request (duplicate updated)
	pendingRequests := tox.RequestManager().GetPendingRequests()
	if len(pendingRequests) != 1 {
		t.Fatalf("Expected 1 pending request in RequestManager, got %d", len(pendingRequests))
	}

	// Verify message was updated
	if pendingRequests[0].Message != "Updated message" {
		t.Errorf("Expected message to be updated to 'Updated message', got '%s'", pendingRequests[0].Message)
	}
}

// TestRequestManagerAcceptReject verifies that accept/reject operations work
// correctly through the Tox.RequestManager() interface.
func TestRequestManagerAcceptReject(t *testing.T) {
	// Create a Tox instance
	opts := NewOptionsForTesting()
	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Simulate receiving a friend request
	var senderKey [32]byte
	copy(senderKey[:], []byte("accept_reject_test_key_12345678"))
	tox.receiveFriendRequest(senderKey, "Please accept me!")

	// Accept the request through RequestManager
	accepted := tox.RequestManager().AcceptRequest(senderKey)
	if !accepted {
		t.Error("AcceptRequest should return true for pending request")
	}

	// Verify request is no longer pending
	pendingRequests := tox.RequestManager().GetPendingRequests()
	if len(pendingRequests) != 0 {
		t.Errorf("Expected 0 pending requests after accept, got %d", len(pendingRequests))
	}
}

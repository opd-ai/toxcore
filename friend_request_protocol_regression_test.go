package toxcore

import (
	"testing"
)

// TestFriendRequestProtocolRegression ensures that the friend request protocol
// remains implemented and that AddFriend() continues to send actual network packets.
// This is a regression test for Bug #3: "Friend Request Protocol Not Implemented"
func TestFriendRequestProtocolRegression(t *testing.T) {
	// Create two separate Tox instances to test cross-instance communication
	sender, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create sender Tox instance: %v", err)
	}
	defer sender.Kill()

	receiver, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create receiver Tox instance: %v", err)
	}
	defer receiver.Kill()

	// Test data
	testMessage := "Hello from regression test!"
	receiverToxID := receiver.SelfGetAddress()

	// Track if friend request was received
	var receivedMessage string
	var receivedPublicKey [32]byte
	friendRequestReceived := false

	// Set up receiver callback
	receiver.OnFriendRequest(func(publicKey [32]byte, message string) {
		friendRequestReceived = true
		receivedMessage = message
		receivedPublicKey = publicKey
	})

	// Send friend request
	friendID, err := sender.AddFriend(receiverToxID, testMessage)
	if err != nil {
		t.Fatalf("AddFriend failed: %v", err)
	}

	// Basic validation of local friend creation
	if friendID > 1000 {
		t.Errorf("Unexpected friend ID: %d", friendID)
	}

	senderFriends := sender.GetFriends()
	if len(senderFriends) != 1 {
		t.Fatalf("Expected 1 friend in sender, got %d", len(senderFriends))
	}

	// Process packet delivery (simulate network iterations)
	for i := 0; i < 20; i++ {
		sender.Iterate()
		receiver.Iterate()
	}

	// Verify friend request was transmitted and received
	if !friendRequestReceived {
		t.Fatal("REGRESSION: Friend request was not received - the protocol implementation is broken!")
	}

	// Verify transmitted data integrity
	if receivedMessage != testMessage {
		t.Errorf("Message mismatch: expected %q, got %q", testMessage, receivedMessage)
	}

	expectedPublicKey := sender.SelfGetPublicKey()
	if receivedPublicKey != expectedPublicKey {
		t.Error("Public key mismatch in received friend request")
	}

	// Verify receiver doesn't automatically add the friend (request must be accepted manually)
	receiverFriends := receiver.GetFriends()
	if len(receiverFriends) != 0 {
		t.Errorf("Expected 0 friends in receiver (before accepting), got %d", len(receiverFriends))
	}

	t.Log("✓ Friend request protocol regression test passed")
	t.Log("✓ AddFriend() correctly sends friend request packets")
	t.Log("✓ Receiver correctly processes friend request callbacks")
	t.Log("✓ Bug #3 remains fixed")
}

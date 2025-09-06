package toxcore

import (
	"testing"
)

// TestFriendRequestProtocolImplemented verifies that AddFriend() properly
// sends friend request packets over the network.
func TestFriendRequestProtocolImplemented(t *testing.T) {
	// Create two Tox instances
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

	// Get receiver's Tox ID
	receiverToxID := receiver.SelfGetAddress()

	// Set up callback on receiver to detect friend requests
	friendRequestReceived := false
	testMessage := "Hello, please add me as a friend!"

	receiver.OnFriendRequest(func(publicKey [32]byte, message string) {
		friendRequestReceived = true
		// Verify the data matches what was sent
		if message != testMessage {
			t.Errorf("Expected message %q, got %q", testMessage, message)
		}
		expectedPK := sender.SelfGetPublicKey()
		if publicKey != expectedPK {
			t.Errorf("Public key mismatch")
		}
	})

	// Send friend request from sender to receiver
	friendID, err := sender.AddFriend(receiverToxID, testMessage)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Verify friend was added locally in sender (friend ID 0 is valid)
	if friendID > 1000 {
		t.Errorf("Friend ID seems invalid: %d", friendID)
	}

	// Verify friend exists in sender's friend list
	senderFriends := sender.GetFriends()
	if len(senderFriends) != 1 {
		t.Errorf("Expected 1 friend in sender, got %d", len(senderFriends))
	}

	// Run iterations to process any pending network packets
	for i := 0; i < 10; i++ {
		sender.Iterate()
		receiver.Iterate()
	}

	// The fix: receiver should receive a friend request
	if !friendRequestReceived {
		t.Error("Friend request was not received - the protocol implementation may have a bug")
	}

	// Verify the receiver initially has no friends (before accepting the request)
	receiverFriends := receiver.GetFriends()
	if len(receiverFriends) != 0 {
		t.Errorf("Expected 0 friends in receiver before accepting, got %d", len(receiverFriends))
	}

	// This test confirms the fixed behavior:
	// 1. AddFriend() creates local friend entry in sender ✓
	// 2. Friend request packet is sent over network ✓
	// 3. Receiver gets OnFriendRequest callback ✓
	// 4. Receiver can process the friend request ✓
	t.Log("SUCCESS: AddFriend() properly sends friend request packets over the network")
}

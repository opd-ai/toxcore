package toxcore

import (
	"testing"
)

func TestFriendAccessorMethods(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false // Disable UDP for testing
	
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test GetFriendCount with no friends
	count := tox.GetFriendCount()
	if count != 0 {
		t.Errorf("Expected friend count 0, got %d", count)
	}

	// Test GetFriendList with no friends
	friendList := tox.GetFriendList()
	if len(friendList) != 0 {
		t.Errorf("Expected empty friend list, got %d friends", len(friendList))
	}

	// Test GetMessageQueueLength with empty queue
	queueLength := tox.GetMessageQueueLength()
	if queueLength != 0 {
		t.Errorf("Expected message queue length 0, got %d", queueLength)
	}

	// Add a demo friend
	var publicKey [32]byte
	for i := range publicKey {
		publicKey[i] = byte(i + 1)
	}

	friendID, err := tox.AddFriendByPublicKey(publicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Test GetFriendCount with one friend
	count = tox.GetFriendCount()
	if count != 1 {
		t.Errorf("Expected friend count 1, got %d", count)
	}

	// Test GetFriendList with one friend
	friendList = tox.GetFriendList()
	if len(friendList) != 1 {
		t.Errorf("Expected friend list length 1, got %d", len(friendList))
	}
	if friendList[0] != friendID {
		t.Errorf("Expected friend ID %d in list, got %d", friendID, friendList[0])
	}

	// Test GetFriend
	friend, err := tox.GetFriend(friendID)
	if err != nil {
		t.Errorf("Failed to get friend: %v", err)
	}
	if friend == nil {
		t.Error("GetFriend returned nil friend")
	}
	if friend.PublicKey != publicKey {
		t.Errorf("Friend public key mismatch")
	}

	// Test GetFriend with invalid ID
	_, err = tox.GetFriend(999)
	if err == nil {
		t.Error("Expected error for invalid friend ID, got nil")
	}

	// Test UpdateFriendName
	tox.UpdateFriendName(friendID, "Test Friend")
	friend, _ = tox.GetFriend(friendID)
	if friend.Name != "Test Friend" {
		t.Errorf("Expected friend name 'Test Friend', got '%s'", friend.Name)
	}

	// Test UpdateFriendStatusMessage
	tox.UpdateFriendStatusMessage(friendID, "Test Status")
	friend, _ = tox.GetFriend(friendID)
	if friend.StatusMessage != "Test Status" {
		t.Errorf("Expected friend status message 'Test Status', got '%s'", friend.StatusMessage)
	}
}

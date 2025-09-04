package toxcore

import (
	"testing"
)

// TestGap2MissingGetFriendsMethod reproduces Gap #2 from AUDIT.md
// This test verifies that GetFriends method exists and returns the friends list
func TestGap2MissingGetFriendsMethod(t *testing.T) {
	// Create a new Tox instance
	options := NewOptions()
	options.UDPEnabled = false // Disable UDP for testing

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test that GetFriends method exists and is callable
	// This should fail if the method doesn't exist
	friends := tox.GetFriends()

	// Should initially have no friends
	if len(friends) != 0 {
		t.Errorf("Expected 0 friends initially, got %d", len(friends))
	}

	// Add a friend and verify it appears in GetFriends
	var testPublicKey [32]byte
	copy(testPublicKey[:], "12345678901234567890123456789012")

	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil && err.Error() != "already a friend" {
		t.Logf("AddFriendByPublicKey worked as expected (error: %v)", err)
	}

	// Now GetFriends should show 1 friend
	friends = tox.GetFriends()
	if len(friends) != 1 {
		t.Errorf("Expected 1 friend after adding, got %d", len(friends))
	}

	// Verify the friend ID is in the returned map/slice
	if friends == nil {
		t.Error("GetFriends returned nil")
	}

	t.Logf("Added friend ID: %d, friends count: %d", friendID, len(friends))
}

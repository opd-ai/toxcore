package toxcore

import (
	"testing"
	"time"
)

// TestGetFriendsEncapsulation verifies that GetFriends returns deep copies
// and external modifications don't affect internal state (AUDIT.md Priority 4)
func TestGetFriendsEncapsulation(t *testing.T) {
	// Create a new Tox instance
	options := NewOptions()
	options.UDPEnabled = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a friend
	var testPublicKey [32]byte
	copy(testPublicKey[:], testPublicKeyString)

	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil && err.Error() != "already a friend" {
		t.Logf("AddFriendByPublicKey: %v", err)
	}

	// Get initial friends list
	friends1 := tox.GetFriends()
	if len(friends1) != 1 {
		t.Fatalf("Expected 1 friend, got %d", len(friends1))
	}

	// Store initial values
	initialName := friends1[friendID].Name
	initialStatus := friends1[friendID].Status
	initialStatusMsg := friends1[friendID].StatusMessage
	initialLastSeen := friends1[friendID].LastSeen

	// Attempt to modify the returned Friend object
	friends1[friendID].Name = "Modified Name"
	friends1[friendID].Status = FriendStatusBusy
	friends1[friendID].StatusMessage = "Modified Status"
	friends1[friendID].LastSeen = time.Now().Add(24 * time.Hour)

	// Get friends list again and verify internal state wasn't modified
	friends2 := tox.GetFriends()
	if len(friends2) != 1 {
		t.Fatalf("Expected 1 friend in second retrieval, got %d", len(friends2))
	}

	// Verify internal state is unchanged
	if friends2[friendID].Name != initialName {
		t.Errorf("Internal Name was modified: expected %q, got %q", initialName, friends2[friendID].Name)
	}
	if friends2[friendID].Status != initialStatus {
		t.Errorf("Internal Status was modified: expected %v, got %v", initialStatus, friends2[friendID].Status)
	}
	if friends2[friendID].StatusMessage != initialStatusMsg {
		t.Errorf("Internal StatusMessage was modified: expected %q, got %q", initialStatusMsg, friends2[friendID].StatusMessage)
	}
	if !friends2[friendID].LastSeen.Equal(initialLastSeen) {
		t.Errorf("Internal LastSeen was modified: expected %v, got %v", initialLastSeen, friends2[friendID].LastSeen)
	}

	t.Log("✓ GetFriends properly returns deep copies - internal state protected")
}

// TestGetFriendsMultipleCallsIndependent verifies that multiple calls to GetFriends
// return independent copies that don't affect each other
func TestGetFriendsMultipleCallsIndependent(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a friend
	var testPublicKey [32]byte
	copy(testPublicKey[:], testPublicKeyString)

	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil && err.Error() != "already a friend" {
		t.Logf("AddFriendByPublicKey: %v", err)
	}

	// Get two independent copies
	friends1 := tox.GetFriends()
	friends2 := tox.GetFriends()

	// Modify first copy
	friends1[friendID].Name = "Modified in Copy 1"
	friends1[friendID].Status = FriendStatusAway

	// Verify second copy is unaffected
	if friends2[friendID].Name == "Modified in Copy 1" {
		t.Error("Modification to first copy affected second copy")
	}
	if friends2[friendID].Status == FriendStatusAway {
		t.Error("Status modification to first copy affected second copy")
	}

	// Modify second copy
	friends2[friendID].Name = "Modified in Copy 2"
	friends2[friendID].Status = FriendStatusBusy

	// Verify first copy retains its modifications
	if friends1[friendID].Name != "Modified in Copy 1" {
		t.Error("First copy was affected by second copy modification")
	}
	if friends1[friendID].Status != FriendStatusAway {
		t.Error("First copy status was affected by second copy modification")
	}

	t.Log("✓ Multiple GetFriends calls return independent copies")
}

// TestGetFriendsEmptyMap verifies behavior when there are no friends
func TestGetFriendsEmptyMap(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	friends := tox.GetFriends()

	// Should return empty map, not nil
	if friends == nil {
		t.Error("GetFriends returned nil instead of empty map")
	}

	if len(friends) != 0 {
		t.Errorf("Expected 0 friends, got %d", len(friends))
	}

	t.Log("✓ GetFriends returns empty map when no friends exist")
}

// TestGetFriendsPublicKeyIntegrity verifies that PublicKey arrays are properly copied
func TestGetFriendsPublicKeyIntegrity(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Add a friend with a specific public key
	var testPublicKey [32]byte
	for i := 0; i < 32; i++ {
		testPublicKey[i] = byte(i)
	}

	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil && err.Error() != "already a friend" {
		t.Logf("AddFriendByPublicKey: %v", err)
	}

	// Get friends and verify public key
	friends := tox.GetFriends()
	if len(friends) != 1 {
		t.Fatalf("Expected 1 friend, got %d", len(friends))
	}

	// Verify public key matches
	for i := 0; i < 32; i++ {
		if friends[friendID].PublicKey[i] != byte(i) {
			t.Errorf("PublicKey byte %d: expected %d, got %d", i, i, friends[friendID].PublicKey[i])
		}
	}

	// Store original public key
	originalKey := friends[friendID].PublicKey

	// Attempt to modify the public key in the returned copy
	friends[friendID].PublicKey[0] = 255

	// Get friends again and verify internal public key is unchanged
	friends2 := tox.GetFriends()
	if friends2[friendID].PublicKey[0] != originalKey[0] {
		t.Error("Internal PublicKey was modified through returned copy")
	}

	t.Log("✓ PublicKey arrays are properly deep copied")
}

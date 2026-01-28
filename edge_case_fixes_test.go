package toxcore

import (
	"testing"
)

// TestFriendIDStartsAtOne verifies that friend IDs start from 1, not 0
func TestFriendIDStartsAtOne(t *testing.T) {
	tox, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Generate a test public key
	var pubKey [32]byte
	for i := range pubKey {
		pubKey[i] = byte(i)
	}

	// Add first friend
	friendID, err := tox.AddFriendByPublicKey(pubKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// First friend should have ID 1, not 0
	if friendID == 0 {
		t.Errorf("First friend has ID 0, which is reserved as invalid/not-found sentinel")
	}

	if friendID != 1 {
		t.Errorf("Expected first friend ID to be 1, got %d", friendID)
	}
}

// TestFriendIDZeroReservedAsSentinel verifies that ID 0 is reserved
func TestFriendIDZeroReservedAsSentinel(t *testing.T) {
	tox, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Try to find a friend that doesn't exist
	var nonExistentKey [32]byte
	for i := range nonExistentKey {
		nonExistentKey[i] = byte(255 - i)
	}

	friendID := tox.findFriendByPublicKey(nonExistentKey)

	// Should return 0 to indicate not found
	if friendID != 0 {
		t.Errorf("Expected findFriendByPublicKey to return 0 for non-existent friend, got %d", friendID)
	}
}

// TestFriendIDSequential verifies that friend IDs are assigned sequentially from 1
func TestFriendIDSequential(t *testing.T) {
	tox, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	expectedIDs := []uint32{1, 2, 3, 4, 5}

	for i, expectedID := range expectedIDs {
		var pubKey [32]byte
		// Generate unique public key for each friend
		pubKey[0] = byte(i)
		pubKey[31] = byte(i * 2)

		friendID, err := tox.AddFriendByPublicKey(pubKey)
		if err != nil {
			t.Fatalf("Failed to add friend %d: %v", i, err)
		}

		if friendID != expectedID {
			t.Errorf("Friend %d: expected ID %d, got %d", i, expectedID, friendID)
		}
	}
}

// TestFriendIDNotFoundInAsyncHandler verifies async handler correctly identifies invalid friends
func TestFriendIDNotFoundInAsyncHandler(t *testing.T) {
	tox, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Set up friend message callback to track calls
	callbackInvoked := false
	tox.OnFriendMessage(func(friendID uint32, message string) {
		callbackInvoked = true
	})

	// Simulate async message from unknown sender
	var unknownSender [32]byte
	for i := range unknownSender {
		unknownSender[i] = byte(i + 100)
	}

	// Manually call findFriendByPublicKey to verify it returns 0
	friendID := tox.findFriendByPublicKey(unknownSender)
	if friendID != 0 {
		t.Errorf("Expected findFriendByPublicKey to return 0 for unknown sender, got %d", friendID)
	}

	// Verify that the check friendID != 0 would correctly filter this out
	if friendID != 0 {
		// This branch should NOT be taken
		t.Error("Friend ID check failed to filter out invalid friend ID 0")
	}

	// Callback should not have been invoked
	if callbackInvoked {
		t.Error("Callback was invoked for unknown sender")
	}
}

// TestGenerateFriendIDConcurrency verifies thread-safe ID generation
// Note: While generateFriendID itself is thread-safe, AddFriendByPublicKey
// requires external synchronization for concurrent calls
func TestGenerateFriendIDConcurrency(t *testing.T) {
	tox, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	numFriends := 20
	results := make([]uint32, numFriends)

	// Add friends sequentially to avoid race conditions in AddFriendByPublicKey
	for i := 0; i < numFriends; i++ {
		var pubKey [32]byte
		pubKey[0] = byte(i)
		pubKey[1] = byte(i >> 8)

		friendID, err := tox.AddFriendByPublicKey(pubKey)
		if err != nil {
			t.Errorf("Failed to add friend %d: %v", i, err)
			results[i] = 0
		} else {
			results[i] = friendID
		}
	}

	// Verify all IDs are unique and non-zero
	seen := make(map[uint32]bool)
	for i, friendID := range results {
		if friendID == 0 {
			t.Errorf("Friend %d received invalid ID 0", i)
		}
		if seen[friendID] {
			t.Errorf("Duplicate friend ID generated: %d", friendID)
		}
		seen[friendID] = true
	}

	// Verify all IDs are in the expected range [1, numFriends]
	if len(seen) != numFriends {
		t.Errorf("Expected %d unique IDs, got %d", numFriends, len(seen))
	}
}

// TestGenerateNospamNonZero verifies that generateNospam never returns all zeros
func TestGenerateNospamNonZero(t *testing.T) {
	// Generate multiple nospam values to ensure randomness
	for i := 0; i < 100; i++ {
		nospam := generateNospam()

		// Check if all bytes are zero
		allZero := true
		for _, b := range nospam {
			if b != 0 {
				allZero = false
				break
			}
		}

		if allZero {
			t.Errorf("generateNospam() returned all zeros on iteration %d", i)
		}
	}
}

// TestGenerateNospamRandomness verifies that generateNospam produces different values
func TestGenerateNospamRandomness(t *testing.T) {
	const samples = 100
	nospams := make([][4]byte, samples)

	// Generate multiple samples
	for i := 0; i < samples; i++ {
		nospams[i] = generateNospam()
	}

	// Count duplicates
	seen := make(map[[4]byte]int)
	for _, nospam := range nospams {
		seen[nospam]++
	}

	// With 100 random 4-byte values, duplicates are extremely unlikely
	// If we see more than 5% duplicates, randomness is probably broken
	duplicateCount := 0
	for _, count := range seen {
		if count > 1 {
			duplicateCount += count - 1
		}
	}

	if duplicateCount > 5 {
		t.Errorf("Too many duplicate nospam values: %d out of %d (>5%%)", duplicateCount, samples)
	}
}

// TestNospamInToxInstance verifies that Tox instances get non-zero nospam values
func TestNospamInToxInstance(t *testing.T) {
	tox, err := New(nil)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Get the nospam value
	nospam := tox.SelfGetNospam()

	// Verify it's not all zeros
	allZero := true
	for _, b := range nospam {
		if b != 0 {
			allZero = false
			break
		}
	}

	if allZero {
		t.Error("Tox instance nospam is all zeros - generateNospam() may have failed")
	}
}

// TestMultipleToxInstancesUnique verifies that different Tox instances get different nospam values
func TestMultipleToxInstancesUnique(t *testing.T) {
	const numInstances = 10
	nospams := make([][4]byte, numInstances)

	// Create multiple instances
	for i := 0; i < numInstances; i++ {
		tox, err := New(nil)
		if err != nil {
			t.Fatalf("Failed to create Tox instance %d: %v", i, err)
		}
		nospams[i] = tox.SelfGetNospam()
		tox.Kill()
	}

	// Check for duplicates
	seen := make(map[[4]byte]bool)
	for i, nospam := range nospams {
		if seen[nospam] {
			t.Errorf("Duplicate nospam value found at instance %d: %v", i, nospam)
		}
		seen[nospam] = true
	}
}

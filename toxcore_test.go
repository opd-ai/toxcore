package toxcore

import (
	"testing"
	"time"
)

func TestIterateBasic(t *testing.T) {
	// Create a new Tox instance
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test that Iterate() doesn't panic or return errors
	tox.Iterate()

	// Test multiple iterations
	for i := 0; i < 5; i++ {
		tox.Iterate()
		time.Sleep(10 * time.Millisecond)
	}
}

func TestIterationInterval(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	interval := tox.IterationInterval()
	if interval <= 0 {
		t.Errorf("IterationInterval() should return positive duration, got %v", interval)
	}

	// Should be reasonable (between 1ms and 1s)
	if interval < time.Millisecond || interval > time.Second {
		t.Errorf("IterationInterval() returned unreasonable value: %v", interval)
	}
}

func TestIsRunning(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	// Should be running initially
	if !tox.IsRunning() {
		t.Error("Tox should be running after creation")
	}

	// Should stop running after Kill()
	tox.Kill()
	if tox.IsRunning() {
		t.Error("Tox should not be running after Kill()")
	}
}

func TestToxIDGeneration(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Should have valid Tox ID
	toxID := tox.SelfGetAddress()
	if len(toxID) != 76 { // 38 bytes * 2 hex chars = 76 chars
		t.Errorf("Expected Tox ID length 76, got %d", len(toxID))
	}

	// Should have valid public key
	pubKey := tox.SelfGetPublicKey()
	if pubKey == [32]byte{} {
		t.Error("Public key should not be zero")
	}
}

func TestAddFriendAPI(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test the documented AddFriend API from README.md
	var friendPubKey [32]byte
	for i := range friendPubKey {
		friendPubKey[i] = byte(i + 1) // Create dummy public key
	}

	// This tests the API signature documented in README
	friendID, err := tox.AddFriendByPublicKey(friendPubKey)
	if err != nil {
		t.Errorf("AddFriendByPublicKey() failed: %v", err)
	}

	// Verify friend was added
	if !tox.FriendExists(friendID) {
		t.Error("Friend should exist after AddFriend()")
	}
}

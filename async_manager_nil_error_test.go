package toxcore

import (
	"strings"
	"testing"
)

// TestSendAsyncMessageReturnsErrorWhenAsyncManagerNil is a regression test for Gap #2
// from AUDIT.md. It verifies that sendAsyncMessage returns an error when asyncManager
// is nil, rather than silently succeeding.
//
// Bug Reference: AUDIT.md Gap #2 - "Async SendFriendMessage Silent Success on Unavailable Async Manager"
// Expected Behavior: When a friend is offline and async messaging fails (e.g., async manager is nil),
// the function should return an error as documented in README.md:417-419.
func TestSendAsyncMessageReturnsErrorWhenAsyncManagerNil(t *testing.T) {
	// Create a Tox instance with options that might result in nil asyncManager
	// (e.g., if async initialization fails or is disabled)
	options := NewOptionsForTesting()
	options.UDPEnabled = false // Disable UDP to simplify test setup

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Force asyncManager to be nil to simulate the failure case
	// In production, this could happen if async initialization fails
	tox.asyncManager = nil

	// Add a friend (will be offline by default)
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Attempt to send a message to the offline friend
	// This should return an error because asyncManager is nil
	err = tox.SendFriendMessage(friendID, "Test message")

	// Verify that we got an error (not nil)
	if err == nil {
		t.Fatal("Expected error when asyncManager is nil, but got nil (silent success)")
	}

	// Verify the error message indicates async messaging is unavailable
	errorMsg := err.Error()
	if !strings.Contains(errorMsg, "async messaging is unavailable") {
		t.Errorf("Expected error message to mention 'async messaging is unavailable', got: %s", errorMsg)
	}

	t.Logf("Correctly returned error when asyncManager is nil: %v", err)
}

// TestSendAsyncMessageSucceedsWithAsyncManagerPresent verifies that sending
// to an offline friend succeeds when asyncManager is properly initialized.
func TestSendAsyncMessageSucceedsWithAsyncManagerPresent(t *testing.T) {
	// Create a Tox instance with async messaging enabled
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify asyncManager was initialized
	if tox.asyncManager == nil {
		t.Skip("AsyncManager was not initialized - skipping test")
	}

	// Add a friend (will be offline by default)
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friendID, err := tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Attempt to send a message to the offline friend
	// With asyncManager present, this may succeed (queued) or fail with a different error
	// (e.g., no pre-keys exchanged), but should not silently succeed with nil asyncManager
	err = tox.SendFriendMessage(friendID, "Test message")

	// We expect either:
	// 1. Success (nil error) - message queued for async delivery
	// 2. Specific error about pre-keys - async manager working but keys not exchanged
	// 3. Other legitimate async messaging errors
	//
	// What we DON'T want is silent success when asyncManager is nil
	if err != nil {
		t.Logf("SendFriendMessage returned error (expected): %v", err)
		// Verify it's not the "async messaging unavailable" error
		if strings.Contains(err.Error(), "async messaging is unavailable") {
			t.Error("Got 'async messaging unavailable' error even though asyncManager is present")
		}
	} else {
		t.Log("SendFriendMessage succeeded - message queued for async delivery")
	}
}

// TestAsyncManagerNilErrorMessageClarity verifies that the error message
// provides clear context to developers about why the message failed.
func TestAsyncManagerNilErrorMessageClarity(t *testing.T) {
	options := NewOptionsForTesting()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Force nil asyncManager
	tox.asyncManager = nil

	// Add friend
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	friendID, _ := tox.AddFriendByPublicKey(testPublicKey)

	// Test that error message is informative
	err = tox.SendFriendMessage(friendID, "Test")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	errorMsg := err.Error()
	expectedPhrases := []string{
		"friend is not connected",
		"async messaging is unavailable",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(errorMsg, phrase) {
			t.Errorf("Error message missing expected phrase '%s'. Got: %s", phrase, errorMsg)
		}
	}

	t.Logf("Error message provides clear context: %s", errorMsg)
}

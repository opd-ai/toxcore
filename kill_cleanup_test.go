package toxcore

import (
	"testing"
	"time"
)

// TestKillCleanup tests that Kill() properly cleans up all resources
func TestKillCleanup(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	// Add a friend to test cleanup
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	_, err = tox.AddFriendByPublicKey(testPublicKey)
	if err != nil {
		t.Fatalf("Failed to add friend: %v", err)
	}

	// Set some callbacks to test cleanup
	tox.OnFriendMessage(func(friendID uint32, message string) {
		// Test callback
	})

	tox.OnFriendRequest(func(publicKey [32]byte, message string) {
		// Test callback
	})

	// Verify initial state
	if len(tox.friends) == 0 {
		t.Error("Expected friend to be added")
	}

	if tox.simpleFriendMessageCallback == nil {
		t.Error("Expected message callback to be set")
	}

	if tox.friendRequestCallback == nil {
		t.Error("Expected request callback to be set")
	}

	// Kill the instance
	tox.Kill()

	// Give a moment for cleanup to complete
	time.Sleep(10 * time.Millisecond)

	// Verify cleanup
	if tox.running {
		t.Error("Expected running to be false after Kill()")
	}

	if tox.friends != nil {
		t.Error("Expected friends map to be nil after Kill()")
	}

	if tox.friendRequestCallback != nil {
		t.Error("Expected friend request callback to be nil after Kill()")
	}

	if tox.simpleFriendMessageCallback != nil {
		t.Error("Expected friend message callback to be nil after Kill()")
	}

	if tox.messageManager != nil {
		t.Error("Expected message manager to be nil after Kill()")
	}

	if tox.dht != nil {
		t.Error("Expected DHT to be nil after Kill()")
	}

	if tox.bootstrapManager != nil {
		t.Error("Expected bootstrap manager to be nil after Kill()")
	}
}

// TestKillIdempotent tests that calling Kill() multiple times is safe
func TestKillIdempotent(t *testing.T) {
	options := NewOptions()
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}

	// Kill multiple times - should not panic or cause issues
	tox.Kill()
	tox.Kill()
	tox.Kill()

	// Verify state is still cleaned up
	if tox.running {
		t.Error("Expected running to be false after multiple Kill() calls")
	}
}

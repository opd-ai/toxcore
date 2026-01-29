package toxcore

import (
	"testing"

	"github.com/opd-ai/toxcore/async"
)

// TestOnAsyncMessageAPI tests that OnAsyncMessage is exposed through main Tox interface
func TestOnAsyncMessageAPI(t *testing.T) {
	// Create Tox instance
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Test that OnAsyncMessage method exists and can be called
	tox.OnAsyncMessage(func(senderPK [32]byte, message string, messageType async.MessageType) {
		// Handler would be called when async messages are received
		t.Logf("Async message handler set successfully")
	})

	// This test verifies the API exists - actual async functionality would require
	// a full integration test with multiple instances
	t.Log("OnAsyncMessage API is available on main Tox interface")
}

// TestIsAsyncMessagingAvailable tests that applications can check async availability
func TestIsAsyncMessagingAvailable(t *testing.T) {
	// Create Tox instance with UDP enabled (async should initialize)
	options := NewOptions()
	options.UDPEnabled = true

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify IsAsyncMessagingAvailable method exists and returns a boolean
	available := tox.IsAsyncMessagingAvailable()
	t.Logf("Async messaging available: %v", available)

	// The method should return true or false without panicking
	// Actual availability depends on async manager initialization success
	if available {
		// If available, GetAsyncStorageStats should not return nil
		stats := tox.GetAsyncStorageStats()
		if stats == nil {
			t.Errorf("IsAsyncMessagingAvailable returned true but GetAsyncStorageStats returned nil")
		}
	} else {
		// If not available, GetAsyncStorageStats should return nil
		stats := tox.GetAsyncStorageStats()
		if stats != nil {
			t.Errorf("IsAsyncMessagingAvailable returned false but GetAsyncStorageStats returned non-nil")
		}
	}
}

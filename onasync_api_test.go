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

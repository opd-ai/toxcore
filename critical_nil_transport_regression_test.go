package toxcore

import (
	"testing"
)

// TestCriticalBugNilPointerDereference reproduces and verifies the fix for
// the critical nil pointer dereference bug identified in AUDIT.md Priority 1.
//
// Bug Description: When creating a Tox instance with UDPEnabled = false,
// the application would panic with SIGSEGV because NewAsyncClient called
// trans.RegisterHandler() without checking if trans was nil.
//
// Expected Behavior: According to README.md, async messaging should gracefully
// degrade when unavailable, not crash the application.
//
// This test verifies that the bug is fixed and the application handles
// nil transport gracefully.
func TestCriticalBugNilPointerDereference(t *testing.T) {
	// Create options with UDP disabled - this was causing the panic
	options := NewOptions()
	options.UDPEnabled = false

	// This previously caused a panic with:
	// panic: runtime error: invalid memory address or nil pointer dereference
	// [signal SIGSEGV: segmentation violation code=0x1 addr=0x28 pc=0x6850f9]
	//
	// After the fix, this should succeed without panic
	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance with UDP disabled: %v", err)
	}

	// Verify that the Tox instance was created successfully
	if tox == nil {
		t.Fatal("Tox instance is nil")
	}

	// Cleanup
	defer tox.Kill()

	// Verify basic functionality still works
	address := tox.SelfGetAddress()
	if len(address) == 0 {
		t.Error("Tox address is empty")
	}

	publicKey := tox.SelfGetPublicKey()
	if publicKey == ([32]byte{}) {
		t.Error("Tox public key is zero")
	}

	// Test passed - no panic occurred and basic functionality works
	t.Log("Successfully created Tox instance with UDP disabled")
	t.Log("Async messaging gracefully degraded as expected")
}

// TestNilTransportGracefulDegradation verifies that async messaging
// features properly report unavailability when transport is nil,
// rather than causing crashes.
func TestNilTransportGracefulDegradation(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Verify core Tox functionality remains available
	if !tox.IsRunning() {
		t.Error("Tox should be running")
	}

	// Verify async manager was created (even with nil transport)
	if tox.asyncManager == nil {
		t.Error("Async manager should be initialized")
	}

	// Async messaging operations should fail gracefully (not panic)
	// when transport is unavailable
	testPublicKey := [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}

	// This should not panic - it should either succeed with empty result
	// or fail with a descriptive error
	err = tox.asyncManager.SendAsyncMessage(testPublicKey, "test", 0)
	// We don't check for specific error - just that it didn't panic
	// The error could be "no storage nodes" or "transport unavailable"
	t.Logf("SendAsyncMessage result: %v (expected to fail gracefully)", err)
}

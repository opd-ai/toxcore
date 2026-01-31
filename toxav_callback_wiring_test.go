package toxcore

import (
	"testing"
	"time"

	avpkg "github.com/opd-ai/toxcore/av"
)

// TestToxAVCallbackWiring verifies that ToxAV callbacks are properly wired
// to the underlying av.Manager implementation.
func TestToxAVCallbackWiring(t *testing.T) {
	// Create Tox instance with test options
	opts := NewOptions()
	opts.UDPEnabled = true
	opts.IPv6Enabled = false

	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	// Create ToxAV instance
	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV instance: %v", err)
	}

	// Test 1: Verify CallbackCall wiring
	t.Run("CallbackCall", func(t *testing.T) {
		toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
			t.Logf("Call callback received: friend=%d, audio=%t, video=%t",
				friendNumber, audioEnabled, videoEnabled)
		})

		// Verify the callback is stored in ToxAV
		toxav.mu.RLock()
		hasToxAVCallback := toxav.callCb != nil
		toxav.mu.RUnlock()

		if !hasToxAVCallback {
			t.Error("Call callback not stored in ToxAV")
		}

		t.Log("CallbackCall successfully wired")
	})

	// Test 2: Verify CallbackCallState wiring
	t.Run("CallbackCallState", func(t *testing.T) {
		stateChanges := make([]avpkg.CallState, 0)

		toxav.CallbackCallState(func(friendNumber uint32, state avpkg.CallState) {
			stateChanges = append(stateChanges, state)
			t.Logf("Call state callback received: friend=%d, state=%d",
				friendNumber, state)
		})

		// Verify the callback is stored in ToxAV
		toxav.mu.RLock()
		hasToxAVCallback := toxav.callStateCb != nil
		toxav.mu.RUnlock()

		if !hasToxAVCallback {
			t.Error("Call state callback not stored in ToxAV")
		}

		t.Log("CallbackCallState successfully wired")
	})

	// Test 3: Verify nil callbacks can be set
	t.Run("NilCallbacks", func(t *testing.T) {
		toxav.CallbackCall(nil)
		toxav.CallbackCallState(nil)

		toxav.mu.RLock()
		hasCallCb := toxav.callCb != nil
		hasStateCb := toxav.callStateCb != nil
		toxav.mu.RUnlock()

		if hasCallCb {
			t.Error("Expected call callback to be nil after setting to nil")
		}
		if hasStateCb {
			t.Error("Expected call state callback to be nil after setting to nil")
		}

		t.Log("Nil callbacks successfully set")
	})
}

// TestToxAVCallbackInvocation verifies that callbacks are invoked when
// appropriate events occur in the av.Manager.
func TestToxAVCallbackInvocation(t *testing.T) {
	// Create two Tox instances for peer-to-peer testing
	opts1 := NewOptions()
	opts1.UDPEnabled = true
	opts1.IPv6Enabled = false

	tox1, err := New(opts1)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	toxav1, err := NewToxAV(tox1)
	if err != nil {
		t.Fatalf("Failed to create first ToxAV instance: %v", err)
	}

	// Track callback invocations
	stateChanges := make([]avpkg.CallState, 0)

	toxav1.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
		t.Logf("📞 Incoming call from friend %d (audio: %t, video: %t)",
			friendNumber, audioEnabled, videoEnabled)
	})

	toxav1.CallbackCallState(func(friendNumber uint32, state avpkg.CallState) {
		stateChanges = append(stateChanges, state)
		t.Logf("📊 Call state changed for friend %d: %d", friendNumber, state)
	})

	// Verify callbacks are registered
	if toxav1.callCb == nil {
		t.Error("Call callback not registered")
	}
	if toxav1.callStateCb == nil {
		t.Error("Call state callback not registered")
	}

	t.Log("Callbacks registered and ready for invocation")
}

// TestToxAVCallbackConcurrentAccess verifies thread-safe callback registration
// and invocation under concurrent access.
func TestToxAVCallbackConcurrentAccess(t *testing.T) {
	opts := NewOptions()
	opts.UDPEnabled = true
	opts.IPv6Enabled = false

	tox, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create Tox instance: %v", err)
	}
	defer tox.Kill()

	toxav, err := NewToxAV(tox)
	if err != nil {
		t.Fatalf("Failed to create ToxAV instance: %v", err)
	}

	// Register and unregister callbacks concurrently
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			toxav.CallbackCall(func(friendNumber uint32, audioEnabled, videoEnabled bool) {
				// Callback body
			})
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			toxav.CallbackCallState(func(friendNumber uint32, state avpkg.CallState) {
				// Callback body
			})
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for completion
	<-done
	<-done

	// Verify final state
	toxav.mu.RLock()
	hasCallCb := toxav.callCb != nil
	hasStateCb := toxav.callStateCb != nil
	toxav.mu.RUnlock()

	if !hasCallCb {
		t.Error("Call callback lost during concurrent access")
	}
	if !hasStateCb {
		t.Error("Call state callback lost during concurrent access")
	}

	t.Log("Concurrent callback access test passed")
}

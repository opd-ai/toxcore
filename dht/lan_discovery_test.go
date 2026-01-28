package dht

import (
	"net"
	"sync"
	"testing"
)

// TestLANDiscoveryStubCompilation verifies that the LANDiscovery stub
// implementation compiles and provides the expected interface.
func TestLANDiscoveryStubCompilation(t *testing.T) {
	// Create a test public key
	var publicKey [32]byte
	for i := range publicKey {
		publicKey[i] = byte(i)
	}

	// Create LAN discovery instance
	ld := NewLANDiscovery(publicKey, 33445)

	if ld == nil {
		t.Fatal("NewLANDiscovery should not return nil")
	}

	t.Log("LANDiscovery stub compiles and instantiates correctly")
}

// TestLANDiscoveryStartStop verifies the Start and Stop methods are idempotent.
func TestLANDiscoveryStartStop(t *testing.T) {
	var publicKey [32]byte
	ld := NewLANDiscovery(publicKey, 33445)

	// Initially should not be running
	if ld.IsRunning() {
		t.Error("LANDiscovery should not be running initially")
	}

	// Start should succeed
	err := ld.Start()
	if err != nil {
		t.Errorf("Start() should return nil, got: %v", err)
	}

	// Should be running now
	if !ld.IsRunning() {
		t.Error("LANDiscovery should be running after Start()")
	}

	// Start again should be idempotent (no error)
	err = ld.Start()
	if err != nil {
		t.Errorf("Second Start() should return nil (idempotent), got: %v", err)
	}

	// Should still be running
	if !ld.IsRunning() {
		t.Error("LANDiscovery should still be running after second Start()")
	}

	// Stop should work
	ld.Stop()
	if ld.IsRunning() {
		t.Error("LANDiscovery should not be running after Stop()")
	}

	// Stop again should be idempotent
	ld.Stop()
	if ld.IsRunning() {
		t.Error("LANDiscovery should not be running after second Stop()")
	}

	t.Log("LANDiscovery Start/Stop are idempotent")
}

// TestLANDiscoveryOnPeerCallback verifies callback registration.
func TestLANDiscoveryOnPeerCallback(t *testing.T) {
	var publicKey [32]byte
	ld := NewLANDiscovery(publicKey, 33445)

	callback := func(pk [32]byte, addr net.Addr) {
		// This callback is stored but never called in the stub
	}

	// Register callback
	ld.OnPeer(callback)

	// Verify callback was stored (we check internal state)
	ld.mu.RLock()
	hasCallback := ld.callback != nil
	ld.mu.RUnlock()

	if !hasCallback {
		t.Error("Callback should be registered after OnPeer()")
	}

	// Note: In the stub implementation, the callback is never called
	// since no actual discovery happens. This just verifies registration.
	t.Log("LANDiscovery callback registration works correctly")
}

// TestLANDiscoveryThreadSafety verifies thread-safe operation with concurrent calls.
func TestLANDiscoveryThreadSafety(t *testing.T) {
	var publicKey [32]byte
	ld := NewLANDiscovery(publicKey, 33445)

	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // Start, Stop, and IsRunning goroutines

	// Concurrent Start() calls
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = ld.Start()
			}
		}()
	}

	// Concurrent Stop() calls
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				ld.Stop()
			}
		}()
	}

	// Concurrent IsRunning() calls
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = ld.IsRunning()
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// If we got here without deadlock or panic, the test passed
	t.Log("LANDiscovery is thread-safe under concurrent access")
}

// TestLANDiscoveryNoOpBehavior verifies the stub performs no actual discovery.
func TestLANDiscoveryNoOpBehavior(t *testing.T) {
	var publicKey [32]byte
	ld := NewLANDiscovery(publicKey, 33445)

	callbackCount := 0
	ld.OnPeer(func(pk [32]byte, addr net.Addr) {
		callbackCount++
	})

	// Start the stub
	err := ld.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// The stub should not discover any peers (no-op behavior)
	// We verify this by checking the callback was never invoked
	if callbackCount != 0 {
		t.Errorf("Stub should not invoke callback, but got %d invocations", callbackCount)
	}

	// Stop the stub
	ld.Stop()

	// Callback should still be zero
	if callbackCount != 0 {
		t.Errorf("Callback count should still be 0 after Stop(), got %d", callbackCount)
	}

	t.Log("LANDiscovery stub correctly performs no actual discovery")
}

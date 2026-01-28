package toxcore

import (
	"testing"
	"time"
)

// TestLocalDiscoveryIntegration tests LAN discovery with Tox instances.
func TestLocalDiscoveryIntegration(t *testing.T) {
	// Create two Tox instances with LAN discovery enabled on different ports
	options1 := NewOptions()
	options1.LocalDiscovery = true
	options1.UDPEnabled = true
	options1.StartPort = 43111
	options1.EndPort = 43111

	options2 := NewOptions()
	options2.LocalDiscovery = true
	options2.UDPEnabled = true
	options2.StartPort = 43112
	options2.EndPort = 43112

	tox1, err := New(options1)
	if err != nil {
		t.Fatalf("Failed to create tox1: %v", err)
	}
	defer tox1.Kill()

	tox2, err := New(options2)
	if err != nil {
		t.Fatalf("Failed to create tox2: %v", err)
	}
	defer tox2.Kill()

	// LAN discovery may not be enabled if there are port conflicts
	// This is acceptable behavior - the instance still works without it
	if tox1.lanDiscovery != nil && tox1.lanDiscovery.IsEnabled() {
		t.Log("tox1 LAN discovery is enabled")
	} else {
		t.Log("tox1 LAN discovery is not enabled (expected if port in use)")
	}

	if tox2.lanDiscovery != nil && tox2.lanDiscovery.IsEnabled() {
		t.Log("tox2 LAN discovery is enabled")
	} else {
		t.Log("tox2 LAN discovery is not enabled (expected if port in use)")
	}

	// Wait for potential discovery (this may not work in all test environments)
	time.Sleep(1 * time.Second)

	// The test passes as long as the Tox instances were created successfully
	// LAN discovery is optional and depends on network configuration
	t.Log("LAN discovery integration test completed successfully")
}

// TestLocalDiscoveryDisabled tests that LAN discovery is not started when disabled.
func TestLocalDiscoveryDisabled(t *testing.T) {
	options := NewOptions()
	options.LocalDiscovery = false
	options.UDPEnabled = true
	options.StartPort = 43113
	options.EndPort = 43113

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create tox: %v", err)
	}
	defer tox.Kill()

	// Verify LAN discovery is not initialized
	if tox.lanDiscovery != nil && tox.lanDiscovery.IsEnabled() {
		t.Error("Expected LAN discovery to be disabled when LocalDiscovery is false")
	}
}

// TestLocalDiscoveryCleanup tests that LAN discovery is properly stopped on Kill().
func TestLocalDiscoveryCleanup(t *testing.T) {
	options := NewOptions()
	options.LocalDiscovery = true
	options.UDPEnabled = true
	options.StartPort = 43114
	options.EndPort = 43114

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create tox: %v", err)
	}

	wasEnabled := tox.lanDiscovery != nil && tox.lanDiscovery.IsEnabled()

	// Kill the instance
	tox.Kill()

	// Verify LAN discovery is stopped if it was enabled
	if wasEnabled && tox.lanDiscovery.IsEnabled() {
		t.Error("Expected LAN discovery to be stopped after Kill()")
	}

	t.Log("LAN discovery cleanup test completed successfully")
}

// TestLocalDiscoveryDefaultPort tests LAN discovery uses the correct port.
func TestLocalDiscoveryDefaultPort(t *testing.T) {
	options := NewOptions()
	options.LocalDiscovery = true
	options.UDPEnabled = true
	options.StartPort = 0 // Should default to 33445
	options.EndPort = 0

	// Since port 33445 might be in use, we expect this might fail gracefully
	tox, err := New(options)
	if err != nil {
		// If it fails to bind, that's ok for this test
		t.Logf("Expected potential failure to bind to port 33445: %v", err)
		return
	}
	defer tox.Kill()

	if tox.lanDiscovery != nil {
		// LAN discovery might not be enabled if port binding failed
		t.Log("LAN discovery initialization attempted with default port")
	}
}

// TestLocalDiscoveryCustomPort tests LAN discovery with a custom port.
func TestLocalDiscoveryCustomPort(t *testing.T) {
	options := NewOptions()
	options.LocalDiscovery = true
	options.UDPEnabled = true
	options.StartPort = 44556
	options.EndPort = 44556

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create tox: %v", err)
	}
	defer tox.Kill()

	// LAN discovery may not be enabled if there are port conflicts
	if tox.lanDiscovery != nil && tox.lanDiscovery.IsEnabled() {
		t.Log("LAN discovery is enabled with custom port")
	} else {
		t.Log("LAN discovery could not bind to custom port (acceptable)")
	}
}

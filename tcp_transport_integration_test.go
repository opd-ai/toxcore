package toxcore

import (
	"testing"
	"time"
)

// TestTCPTransportIntegration verifies TCP transport is properly initialized and integrated.
func TestTCPTransportIntegration(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false // Disable UDP to test TCP only
	options.TCPPort = testTCPPortBase // Use non-standard port for testing

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox with TCP transport: %v", err)
	}
	defer tox.Kill()

	// Verify TCP transport was created
	if tox.tcpTransport == nil {
		t.Fatal("TCP transport should be initialized when TCPPort is set")
	}

	// Verify TCP transport has correct local address
	localAddr := tox.tcpTransport.LocalAddr()
	if localAddr == nil {
		t.Fatal("TCP transport LocalAddr should not be nil")
	}

	t.Logf("TCP transport listening on: %s", localAddr.String())
}

// TestTCPTransportDisabled verifies TCP transport is not created when port is 0.
func TestTCPTransportDisabled(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	options.TCPPort = 0 // Explicitly disable TCP

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox: %v", err)
	}
	defer tox.Kill()

	// Verify TCP transport was not created
	if tox.tcpTransport != nil {
		t.Fatal("TCP transport should not be initialized when TCPPort is 0")
	}
}

// TestBothTransportsEnabled verifies UDP and TCP can coexist.
func TestBothTransportsEnabled(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = true
	options.TCPPort = testTCPPortBase + 1

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox with both transports: %v", err)
	}
	defer tox.Kill()

	// Verify both transports are initialized
	if tox.udpTransport == nil {
		t.Fatal("UDP transport should be initialized")
	}
	if tox.tcpTransport == nil {
		t.Fatal("TCP transport should be initialized")
	}

	t.Logf("UDP transport: %s", tox.udpTransport.LocalAddr().String())
	t.Logf("TCP transport: %s", tox.tcpTransport.LocalAddr().String())
}

// TestTCPTransportCleanup verifies TCP transport is properly closed.
func TestTCPTransportCleanup(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	options.TCPPort = testTCPPortBase + 2

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox: %v", err)
	}

	if tox.tcpTransport == nil {
		t.Fatal("TCP transport should be initialized")
	}

	// Kill should properly clean up TCP transport
	tox.Kill()

	// Give cleanup time to complete
	time.Sleep(100 * time.Millisecond)

	// After Kill, creating another Tox on same port should work
	tox2, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create second Tox instance (TCP port may not have been released): %v", err)
	}
	defer tox2.Kill()
}

// TestTCPPortConflict verifies error handling when port is already in use.
func TestTCPPortConflict(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	options.TCPPort = testTCPPortBase + 3

	tox1, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create first Tox instance: %v", err)
	}
	defer tox1.Kill()

	// Try to create another instance on same TCP port
	tox2, err := New(options)
	if err == nil {
		tox2.Kill()
		t.Fatal("Expected error when TCP port is already in use")
	}

	t.Logf("Correctly received error for port conflict: %v", err)
}

// TestTCPTransportHandlers verifies packet handlers are registered.
func TestTCPTransportHandlers(t *testing.T) {
	options := NewOptions()
	options.UDPEnabled = false
	options.TCPPort = testTCPPortBase + 4

	tox, err := New(options)
	if err != nil {
		t.Fatalf("Failed to create Tox: %v", err)
	}
	defer tox.Kill()

	// The registerTCPHandlers method should have been called during initialization
	// We can't directly verify handlers are registered without accessing internal state,
	// but we can verify the transport exists and basic operations work
	if tox.tcpTransport == nil {
		t.Fatal("TCP transport should be initialized")
	}

	// Verify Tox instance is running
	if !tox.IsRunning() {
		t.Fatal("Tox instance should be running")
	}
}

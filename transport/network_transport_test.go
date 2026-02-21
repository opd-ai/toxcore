package transport

import (
	"strings"
	"testing"
)

// TestNetworkTransportInterface ensures all transport implementations satisfy the interface
func TestNetworkTransportInterface(t *testing.T) {
	tests := []struct {
		name      string
		transport NetworkTransport
	}{
		{"IPTransport", NewIPTransport()},
		{"TorTransport", NewTorTransport()},
		{"I2PTransport", NewI2PTransport()},
		{"NymTransport", NewNymTransport()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the transport implements the interface
			var _ NetworkTransport = tt.transport

			// Test SupportedNetworks returns non-empty slice
			networks := tt.transport.SupportedNetworks()
			if len(networks) == 0 {
				t.Errorf("%s.SupportedNetworks() returned empty slice", tt.name)
			}

			// Test Close doesn't return error for fresh transport
			if err := tt.transport.Close(); err != nil {
				t.Errorf("%s.Close() returned error: %v", tt.name, err)
			}
		})
	}
}

// TestIPTransportOperations tests the fully functional IP transport
func TestIPTransportOperations(t *testing.T) {
	transport := NewIPTransport()
	defer transport.Close()

	// Test SupportedNetworks
	networks := transport.SupportedNetworks()
	expectedNetworks := []string{"tcp", "udp", "tcp4", "tcp6", "udp4", "udp6"}

	if len(networks) != len(expectedNetworks) {
		t.Errorf("Expected %d networks, got %d", len(expectedNetworks), len(networks))
	}

	for _, expected := range expectedNetworks {
		found := false
		for _, network := range networks {
			if network == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected network %s not found in supported networks", expected)
		}
	}

	// Test TCP Listen on localhost
	listener, err := transport.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create TCP listener: %v", err)
	}
	defer listener.Close()

	// Verify listener address
	addr := listener.Addr()
	if addr.Network() != "tcp" {
		t.Errorf("Expected TCP listener, got %s", addr.Network())
	}

	// Test UDP DialPacket
	conn, err := transport.DialPacket("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}
	defer conn.Close()

	// Verify packet connection
	if !strings.Contains(conn.LocalAddr().Network(), "udp") {
		t.Errorf("Expected UDP connection, got %s", conn.LocalAddr().Network())
	}
}

// TestPrivacyTransportPlaceholders tests that privacy transports return appropriate errors
func TestPrivacyTransportPlaceholders(t *testing.T) {
	tests := []struct {
		name      string
		transport NetworkTransport
		address   string
	}{
		{"TorTransport", NewTorTransport(), "test.onion:8080"},
		{"I2PTransport", NewI2PTransport(), "test.b32.i2p:8080"},
		{"NymTransport", NewNymTransport(), "test.nym:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Listen returns error (not implemented or initialization failure)
			_, err := tt.transport.Listen(tt.address)
			if err == nil {
				t.Errorf("%s.Listen() should return error when service is unavailable", tt.name)
			}
			if !strings.Contains(err.Error(), "not yet implemented") &&
				!strings.Contains(err.Error(), "not supported") &&
				!strings.Contains(err.Error(), "failed") {
				t.Errorf("%s.Listen() should return error, got: %v", tt.name, err)
			}

			// Test Dial returns not implemented error
			_, err = tt.transport.Dial(tt.address)
			if err == nil {
				t.Errorf("%s.Dial() should return error for unimplemented transport", tt.name)
			}

			// Test invalid address formats
			invalidAddress := "invalid-address:8080"
			_, err = tt.transport.Listen(invalidAddress)
			if err == nil {
				t.Errorf("%s.Listen() should return error for invalid address format", tt.name)
			}
			if !strings.Contains(err.Error(), "invalid") {
				t.Errorf("%s.Listen() should return 'invalid' error for wrong format, got: %v", tt.name, err)
			}
		})
	}
}

// TestMultiTransportCreation tests the creation and initialization of MultiTransport
func TestMultiTransportCreation(t *testing.T) {
	mt := NewMultiTransport()
	defer mt.Close()

	// Test that all expected transports are registered
	expectedNetworks := []string{"ip", "tor", "i2p", "nym"}
	for _, network := range expectedNetworks {
		transport, exists := mt.GetTransport(network)
		if !exists {
			t.Errorf("Expected transport for %s to be registered", network)
		}
		if transport == nil {
			t.Errorf("Transport for %s should not be nil", network)
		}
	}

	// Test GetSupportedNetworks returns comprehensive list
	supportedNetworks := mt.GetSupportedNetworks()
	if len(supportedNetworks) == 0 {
		t.Error("GetSupportedNetworks() should return non-empty list")
	}

	// Should include IP network types
	hasIP := false
	for _, network := range supportedNetworks {
		if network == "tcp" || network == "udp" {
			hasIP = true
			break
		}
	}
	if !hasIP {
		t.Error("Supported networks should include IP networks (tcp/udp)")
	}
}

// TestMultiTransportSelection tests automatic transport selection based on address format
func TestMultiTransportSelection(t *testing.T) {
	mt := NewMultiTransport()
	defer mt.Close()

	tests := []struct {
		name            string
		address         string
		expectedNetwork string
		shouldFail      bool
	}{
		{"IPv4 address", "127.0.0.1:0", "ip", false},            // Use port 0 for auto-assignment
		{"IPv6 address", "[::1]:0", "ip", false},                // Use port 0 for auto-assignment
		{"Hostname", "localhost:0", "ip", false},                // Use port 0 for auto-assignment
		{"Tor onion", "3g2upl4pq6kufc4m.onion:80", "tor", true}, // Should fail as not implemented
		{"I2P address", "example.b32.i2p:80", "i2p", true},      // Should fail without SAM bridge
		{"Nym address", "example.nym:80", "nym", true},          // Should fail as not implemented
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test transport selection through Listen
			listener, err := mt.Listen(tt.address)

			if tt.shouldFail {
				if err == nil {
					if listener != nil {
						listener.Close()
					}
					t.Errorf("Expected Listen(%s) to fail for unimplemented transport", tt.address)
				}
			} else {
				// For IP addresses, Listen should work
				if err != nil {
					t.Errorf("Expected Listen(%s) to succeed for IP transport, got: %v", tt.address, err)
				} else if listener != nil {
					// Clean up the listener
					listener.Close()
				}
			}
		})
	}
}

// TestMultiTransportRegisterTransport tests dynamic transport registration
func TestMultiTransportRegisterTransport(t *testing.T) {
	mt := NewMultiTransport()
	defer mt.Close()

	// Create a custom transport for testing
	customTransport := NewIPTransport()

	// Register the custom transport
	mt.RegisterTransport("custom", customTransport)

	// Verify it was registered
	transport, exists := mt.GetTransport("custom")
	if !exists {
		t.Error("Custom transport should be registered")
	}
	if transport != customTransport {
		t.Error("Retrieved transport should be the same instance")
	}

	// Verify it appears in supported networks
	supportedNetworks := mt.GetSupportedNetworks()
	hasCustomNetworks := false
	for _, network := range supportedNetworks {
		if network == "tcp" || network == "udp" { // Custom transport is IP-based
			hasCustomNetworks = true
			break
		}
	}
	if !hasCustomNetworks {
		t.Error("Custom transport networks should appear in supported networks")
	}
}

// TestMultiTransportConcurrentOperations tests thread safety of MultiTransport
func TestMultiTransportConcurrentOperations(t *testing.T) {
	mt := NewMultiTransport()
	defer mt.Close()

	// Test concurrent GetTransport calls
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			transport, exists := mt.GetTransport("ip")
			if !exists {
				t.Error("IP transport should exist")
				return
			}
			if transport == nil {
				t.Error("IP transport should not be nil")
				return
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Test concurrent GetSupportedNetworks calls
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			networks := mt.GetSupportedNetworks()
			if len(networks) == 0 {
				t.Error("Should have supported networks")
				return
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestMultiTransportErrorHandling tests error conditions and edge cases
func TestMultiTransportErrorHandling(t *testing.T) {
	mt := NewMultiTransport()
	defer mt.Close()

	// Test invalid network type
	_, exists := mt.GetTransport("nonexistent")
	if exists {
		t.Error("Should not find transport for nonexistent network type")
	}

	// Test empty address - should return error for clearly invalid addresses
	_, err := mt.Listen(":99999") // Use an invalid port instead of empty string
	if err == nil {
		t.Error("Should return error for invalid port number")
	}

	// Test malformed address
	_, err = mt.Listen("not-an-address")
	if err == nil {
		t.Error("Should return error for malformed address")
	}
}

// BenchmarkMultiTransportSelection benchmarks transport selection performance
func BenchmarkMultiTransportSelection(b *testing.B) {
	mt := NewMultiTransport()
	defer mt.Close()

	addresses := []string{
		"127.0.0.1:8080",
		"test.onion:80",
		"example.b32.i2p:80",
		"service.nym:80",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		address := addresses[i%len(addresses)]
		// Only test selection, not actual connection
		mt.selectTransport(address)
	}
}

// BenchmarkIPTransportOperations benchmarks IP transport performance
func BenchmarkIPTransportOperations(b *testing.B) {
	transport := NewIPTransport()
	defer transport.Close()

	b.Run("SupportedNetworks", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			transport.SupportedNetworks()
		}
	})
}

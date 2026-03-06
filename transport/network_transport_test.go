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
			// Test Listen - should either work if service is available OR return proper error
			listener, err := tt.transport.Listen(tt.address)
			if err != nil {
				// Service unavailable or not implemented - verify error message is appropriate
				if !strings.Contains(err.Error(), "not yet implemented") &&
					!strings.Contains(err.Error(), "not supported") &&
					!strings.Contains(err.Error(), "failed") {
					t.Errorf("%s.Listen() should return descriptive error, got: %v", tt.name, err)
				}
			} else {
				// Service is available - listener should be valid
				if listener == nil {
					t.Errorf("%s.Listen() returned nil listener without error", tt.name)
				} else {
					listener.Close()
				}
			}

			// Test Dial - may succeed if service is available
			conn, err := tt.transport.Dial(tt.address)
			if conn != nil {
				conn.Close()
			}
			// If error, that's expected for unavailable services or invalid addresses

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
		mustSucceed     bool
	}{
		{"IPv4 address", "127.0.0.1:0", "ip", true},              // Use port 0 for auto-assignment
		{"IPv6 address", "[::1]:0", "ip", true},                  // Use port 0 for auto-assignment
		{"Hostname", "localhost:0", "ip", true},                  // Use port 0 for auto-assignment
		{"Tor onion", "3g2upl4pq6kufc4m.onion:80", "tor", false}, // May succeed if Tor daemon is running
		{"I2P address", "example.b32.i2p:80", "i2p", false},      // May succeed if SAM bridge is running
		{"Nym address", "example.nym:80", "nym", false},          // May succeed if Nym client is running
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test transport selection through Listen
			listener, err := mt.Listen(tt.address)

			if tt.mustSucceed {
				// IP addresses must always work
				if err != nil {
					t.Errorf("Expected Listen(%s) to succeed for IP transport, got: %v", tt.address, err)
				} else if listener != nil {
					listener.Close()
				}
			} else {
				// Privacy network transports: success depends on daemon availability
				if listener != nil {
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

// TestMultiTransportSelectPacketTransport verifies the packet transport selection logic.
// When Tor and I2P are both registered, selectPacketTransport should return the I2P transport
// for any address (since Tor is TCP-only and cannot handle UDP/datagram traffic).
func TestMultiTransportSelectPacketTransport(t *testing.T) {
	tests := []struct {
		name              string
		address           string
		registerTor       bool
		registerI2P       bool
		registerIP        bool
		expectI2P         bool
		expectError       bool
	}{
		{
			name:        "i2p address always routes to I2P",
			address:     "example.b32.i2p:80",
			registerTor: false,
			registerI2P: true,
			registerIP:  false,
			expectI2P:   true,
		},
		{
			name:        "clearnet address with Tor+I2P routes to I2P",
			address:     "127.0.0.1:8080",
			registerTor: true,
			registerI2P: true,
			registerIP:  false,
			expectI2P:   true,
		},
		{
			name:        "onion address with Tor+I2P routes to I2P",
			address:     "test.onion:80",
			registerTor: true,
			registerI2P: true,
			registerIP:  false,
			expectI2P:   true,
		},
		{
			name:        "clearnet address with IP only routes to IP",
			address:     "127.0.0.1:8080",
			registerTor: false,
			registerI2P: false,
			registerIP:  true,
			expectI2P:   false,
		},
		{
			name:        "no transport for address returns error",
			address:     "127.0.0.1:8080",
			registerTor: false,
			registerI2P: false,
			registerIP:  false,
			expectError: true,
		},
		{
			name:        "i2p address without I2P transport returns error not IP error",
			address:     "example.b32.i2p:80",
			registerTor: false,
			registerI2P: false,
			registerIP:  true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := &MultiTransport{
				transports: make(map[string]NetworkTransport),
			}
			defer mt.Close()

			if tt.registerTor {
				mt.transports["tor"] = NewTorTransport()
			}
			if tt.registerI2P {
				mt.transports["i2p"] = NewI2PTransport()
			}
			if tt.registerIP {
				mt.transports["ip"] = NewIPTransport()
			}

			selected, err := mt.selectPacketTransport(tt.address)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for address %q with no matching transport", tt.address)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error selecting packet transport: %v", err)
			}

			selectedNetworks := selected.SupportedNetworks()
			isI2P := false
			for _, n := range selectedNetworks {
				if n == "i2p" {
					isI2P = true
					break
				}
			}
			if tt.expectI2P && !isI2P {
				t.Errorf("Expected I2P transport for address %q, got transport supporting: %v", tt.address, selectedNetworks)
			}
			if !tt.expectI2P && isI2P {
				t.Errorf("Expected non-I2P transport for address %q, but got I2P transport", tt.address)
			}
		})
	}
}

// TestMultiTransportDialPacketTorI2PRouting verifies that DialPacket routes through I2P
// when both Tor and I2P are registered. Clearnet and .onion addresses should be rejected
// by I2P (which only accepts .i2p addresses), confirming the routing is correct.
func TestMultiTransportDialPacketTorI2PRouting(t *testing.T) {
	// Build a MultiTransport with only Tor and I2P (no direct IP access).
	mt := &MultiTransport{
		transports: make(map[string]NetworkTransport),
	}
	defer mt.Close()
	mt.transports["tor"] = NewTorTransport()
	mt.transports["i2p"] = NewI2PTransport()

	addresses := []struct {
		address     string
		description string
	}{
		{"127.0.0.1:8080", "clearnet address"},
		{"example.onion:80", ".onion address"},
	}

	for _, tc := range addresses {
		t.Run(tc.description, func(t *testing.T) {
			// DialPacket routes through I2P; I2P rejects non-.i2p addresses.
			conn, err := mt.DialPacket(tc.address)
			if conn != nil {
				conn.Close()
				t.Errorf("Expected DialPacket(%q) to fail when routed through I2P, but got a connection", tc.address)
				return
			}
			if err == nil {
				t.Errorf("Expected error from DialPacket(%q) in Tor+I2P mode", tc.address)
			}
		})
	}
}

// TestMultiTransportDialPacketNoTransportError verifies that DialPacket returns a descriptive
// error when no transport is registered for the requested network type.
func TestMultiTransportDialPacketNoTransportError(t *testing.T) {
	// Build a MultiTransport with no transports registered at all.
	mt := &MultiTransport{
		transports: make(map[string]NetworkTransport),
	}
	defer mt.Close()

	conn, err := mt.DialPacket("127.0.0.1:8080")
	if conn != nil {
		conn.Close()
		t.Error("Expected DialPacket to return nil connection when no transport is registered")
	}
	if err == nil {
		t.Fatal("Expected error when no transport is registered, got nil")
	}
	if !strings.Contains(err.Error(), "no transport registered") &&
		!strings.Contains(err.Error(), "transport selection failed") {
		t.Errorf("Expected 'no transport registered' or 'transport selection failed' in error, got: %v", err)
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

// BenchmarkMultiTransportPacketSelection benchmarks packet transport selection performance.
func BenchmarkMultiTransportPacketSelection(b *testing.B) {
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
		mt.selectPacketTransport(address)
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

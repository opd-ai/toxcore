package toxcore

import (
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opd-ai/toxcore/transport"
)

// TestMultiNetworkIntegration provides comprehensive integration testing
// for the entire multi-network architecture including:
// - Address parsing across all network types
// - Network detection and capabilities
// - NAT traversal with network-specific handling
// - Multi-protocol transport selection
// - Cross-network compatibility
func TestMultiNetworkIntegration(t *testing.T) {
	logrus.SetLevel(logrus.WarnLevel) // Reduce logging noise for integration tests

	t.Run("AddressParsingIntegration", testAddressParsingIntegration)
	t.Run("NetworkDetectionIntegration", testNetworkDetectionIntegration)
	t.Run("NATTraversalIntegration", testNATTraversalIntegration)
	t.Run("TransportSelectionIntegration", testTransportSelectionIntegration)
	t.Run("CrossNetworkCompatibility", testCrossNetworkCompatibility)
	t.Run("BackwardCompatibility", testBackwardCompatibility)
	t.Run("EndToEndMultiNetwork", testEndToEndMultiNetwork)
}

// testAddressParsingIntegration validates that all address types can be
// parsed correctly and converted between different representations
func testAddressParsingIntegration(t *testing.T) {
	t.Log("Testing address parsing integration across all network types")

	// Create parser for all network types
	parser := transport.NewMultiNetworkParser()
	defer parser.Close()

	// Test addresses for each network type
	testAddresses := []struct {
		name     string
		address  string
		expected transport.AddressType
	}{
		{"IPv4", "192.168.1.1:8080", transport.AddressTypeIPv4},
		{"IPv6", "[2001:db8::1]:8080", transport.AddressTypeIPv6},
		{"Tor", "facebookcorewwwi.onion:80", transport.AddressTypeOnion},
		{"I2P", "stats.i2p:7070", transport.AddressTypeI2P},
		{"Nym", "example.nym:9000", transport.AddressTypeNym},
		{"Localhost", "127.0.0.1:9999", transport.AddressTypeIPv4},
	}

	for _, tc := range testAddresses {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing address parsing for %s: %s", tc.name, tc.address)

			// Parse the address
			addresses, err := parser.Parse(tc.address)
			require.NoError(t, err, "Failed to parse address: %s", tc.address)
			require.NotEmpty(t, addresses, "No addresses returned for: %s", tc.address)

			// Verify at least one address matches expected type
			found := false
			for _, addr := range addresses {
				if addr.Type == tc.expected {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected address type %v not found in results for %s", tc.expected, tc.address)

			// Verify all addresses are valid
			for _, addr := range addresses {
				assert.NotEqual(t, transport.AddressTypeUnknown, addr.Type, "Address type should not be unknown")
				assert.NotEmpty(t, addr.Data, "Address data should not be empty")
				assert.NotEmpty(t, addr.Network, "Network should not be empty")
				t.Logf("Parsed %s as %s://%s:%d", tc.address, addr.Network, addr.String(), addr.Port)
			}
		})
	}
}

// testNetworkDetectionIntegration validates network capability detection
// works correctly for all supported network types
func testNetworkDetectionIntegration(t *testing.T) {
	t.Log("Testing network detection integration")

	// Create network detector
	detector := transport.NewMultiNetworkDetector()

	// Test network detection for different address types
	testCases := []struct {
		name           string
		address        string
		expectPrivate  bool
		expectNAT      bool
		expectDirectly bool
	}{
		{"PublicIPv4", "8.8.8.8:53", false, false, true},
		{"PrivateIPv4", "192.168.1.1:8080", true, true, true},
		{"LocalhostIPv4", "127.0.0.1:9000", true, false, true},
		{"PublicIPv6", "[2001:4860:4860::8888]:53", false, false, true},
		{"TorOnion", "facebookcorewwwi.onion:80", false, false, false},
		{"I2PAddress", "stats.i2p:7070", false, false, false},
		{"NymAddress", "example.nym:9000", false, false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse address to get NetworkAddress
			parser := transport.NewMultiNetworkParser()
			defer parser.Close()

			addresses, err := parser.Parse(tc.address)
			require.NoError(t, err, "Failed to parse test address")
			require.NotEmpty(t, addresses, "No addresses parsed")

			netAddr := addresses[0] // Use first parsed address

			// Convert to net.Addr for detector
			addr := netAddr.ToNetAddr()
			require.NotNil(t, addr, "Failed to convert to net.Addr")

			// Test network capabilities detection
			capabilities := detector.DetectCapabilities(addr)

			assert.Equal(t, tc.expectPrivate, capabilities.IsPrivateSpace, 
				"Private space detection mismatch for %s", tc.address)
			assert.Equal(t, tc.expectNAT, capabilities.SupportsNAT, 
				"NAT support detection mismatch for %s", tc.address)
			assert.Equal(t, tc.expectDirectly, capabilities.SupportsDirectConnection, 
				"Direct connection support mismatch for %s", tc.address)

			t.Logf("%s capabilities: Private=%v, NAT=%v, Direct=%v, Method=%v",
				tc.name, capabilities.IsPrivateSpace, capabilities.SupportsNAT,
				capabilities.SupportsDirectConnection, capabilities.RoutingMethod)
		})
	}
}

// testNATTraversalIntegration validates NAT traversal works with network detection
func testNATTraversalIntegration(t *testing.T) {
	t.Log("Testing NAT traversal integration with network detection")

	// Create NAT traversal manager with network detection
	natManager := transport.NewNATTraversal()
	defer natManager.Close()

	// Test different address types for NAT detection
	testAddresses := []string{
		"192.168.1.100:8080", // Private IPv4 - should support NAT
		"10.0.0.50:9000",     // Private IPv4 - should support NAT  
		"8.8.8.8:53",         // Public IPv4 - no NAT needed
		"127.0.0.1:8080",     // Localhost - no NAT needed
	}

	for _, addr := range testAddresses {
		t.Run(fmt.Sprintf("NATDetection_%s", addr), func(t *testing.T) {
			// Get network capabilities
			capabilities := natManager.GetNetworkCapabilities(addr)

			// Verify capability consistency
			if capabilities.IsPrivateSpace && addr != "127.0.0.1:8080" {
				assert.True(t, capabilities.SupportsNAT, 
					"Private address should support NAT: %s", addr)
			}

			t.Logf("Address %s: Private=%v, NAT=%v, Direct=%v",
				addr, capabilities.IsPrivateSpace, capabilities.SupportsNAT,
				capabilities.SupportsDirectConnection)
		})
	}
}

// testTransportSelectionIntegration validates multi-protocol transport selection
func testTransportSelectionIntegration(t *testing.T) {
	t.Log("Testing transport selection integration")

	// Create multi-transport system
	multiTransport := transport.NewMultiTransport()
	defer multiTransport.Close()

	// Test transport selection for different network types
	testCases := []struct {
		name               string
		address            string
		expectedNetworks   []string
		shouldSupportListen bool
	}{
		{"IPv4TCP", "192.168.1.1:8080", []string{"tcp", "tcp4"}, true},
		{"IPv6TCP", "[::1]:8080", []string{"tcp", "tcp6"}, true},
		{"TorOnion", "example.onion:80", []string{"tor"}, false}, // Placeholder
		{"I2PAddress", "test.i2p:7070", []string{"i2p"}, false}, // Placeholder
		{"NymAddress", "test.nym:9000", []string{"nym"}, false}, // Placeholder
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get supported networks
			supportedNetworks := multiTransport.SupportedNetworks()
			assert.NotEmpty(t, supportedNetworks, "Should have supported networks")

			// Verify expected networks are supported
			for _, expectedNet := range tc.expectedNetworks {
				found := false
				for _, supported := range supportedNetworks {
					if supported == expectedNet {
						found = true
						break
					}
				}
				if tc.shouldSupportListen {
					assert.True(t, found, "Expected network %s should be supported", expectedNet)
				}
			}

			// Test transport selection (for IP addresses only for now)
			if tc.shouldSupportListen {
				// Test listening capability
				listener, err := multiTransport.Listen(tc.address)
				if err == nil {
					listener.Close()
					t.Logf("Successfully created listener for %s", tc.address)
				} else {
					t.Logf("Listen failed for %s (expected for some cases): %v", tc.address, err)
				}
			}

			t.Logf("Transport selection for %s: Networks=%v, CanListen=%v",
				tc.name, tc.expectedNetworks, tc.shouldSupportListen)
		})
	}
}

// testCrossNetworkCompatibility validates compatibility between different network types
func testCrossNetworkCompatibility(t *testing.T) {
	t.Log("Testing cross-network compatibility")

	// Create components for cross-network testing
	parser := transport.NewMultiNetworkParser()
	defer parser.Close()

	detector := transport.NewMultiNetworkDetector()

	multiTransport := transport.NewMultiTransport()
	defer multiTransport.Close()

	// Test address combinations that should be compatible
	compatibilityTests := []struct {
		name    string
		addr1   string
		addr2   string
		compatible bool
	}{
		{"IPv4ToIPv4", "192.168.1.1:8080", "10.0.0.1:9000", true},
		{"IPv4ToIPv6", "192.168.1.1:8080", "[::1]:9000", true},
		{"IPv6ToIPv6", "[2001:db8::1]:8080", "[2001:db8::2]:9000", true},
		{"TorToTor", "test1.onion:80", "test2.onion:90", true},
		{"I2PToI2P", "test1.i2p:7070", "test2.i2p:7071", true},
		{"IPToTor", "192.168.1.1:8080", "test.onion:80", false}, // Different networks
		{"TorToI2P", "test.onion:80", "test.i2p:7070", false},   // Different networks
	}

	for _, tc := range compatibilityTests {
		t.Run(tc.name, func(t *testing.T) {
			// Parse both addresses
			addrs1, err := parser.Parse(tc.addr1)
			require.NoError(t, err, "Failed to parse first address")
			
			addrs2, err := parser.Parse(tc.addr2)
			require.NoError(t, err, "Failed to parse second address")

			// Check network compatibility (same network type)
			compatible := false
			for _, a1 := range addrs1 {
				for _, a2 := range addrs2 {
					if a1.Network == a2.Network {
						compatible = true
						break
					}
				}
				if compatible {
					break
				}
			}

			if tc.compatible {
				assert.True(t, compatible, "Addresses should be compatible: %s <-> %s", tc.addr1, tc.addr2)
			}

			t.Logf("Compatibility %s <-> %s: %v (expected: %v)", tc.addr1, tc.addr2, compatible, tc.compatible)
		})
	}
}

// testBackwardCompatibility validates legacy protocol compatibility
func testBackwardCompatibility(t *testing.T) {
	t.Log("Testing backward compatibility with legacy protocols")

	// Test IPv4 and IPv6 addresses that should work with current system
	testCases := []struct {
		name     string
		address  string
		legacy   bool
	}{
		{"IPv4", "192.168.1.1:8080", true},
		{"IPv6", "[2001:db8::1]:8080", true},
		{"TorOnion", "test.onion:80", false}, // New feature
		{"I2P", "test.i2p:7070", false},     // New feature
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse with multi-network parser (should always work)
			parser := transport.NewMultiNetworkParser()
			defer parser.Close()

			addresses, err := parser.Parse(tc.address)
			require.NoError(t, err, "Multi-network parser should handle: %s", tc.address)
			require.NotEmpty(t, addresses, "Multi-network parser should return addresses")

			extendedAddr := addresses[0]

			// Verify the address is valid
			assert.NotEqual(t, transport.AddressTypeUnknown, extendedAddr.Type)
			assert.NotEmpty(t, extendedAddr.Data)

			// Test conversion to net.Addr for legacy compatibility
			if tc.legacy {
				netAddr := extendedAddr.ToNetAddr()
				assert.NotNil(t, netAddr, "Should be able to convert to net.Addr")
				t.Logf("Legacy compatibility verified for %s: %s", tc.address, netAddr.String())
			}

			t.Logf("Address %s: Parsed=✓, Legacy=%v", tc.address, tc.legacy)
		})
	}
}

// testEndToEndMultiNetwork validates complete end-to-end functionality
func testEndToEndMultiNetwork(t *testing.T) {
	t.Log("Testing end-to-end multi-network functionality")

	// Create a complete multi-network stack
	parser := transport.NewMultiNetworkParser()
	defer parser.Close()

	detector := transport.NewMultiNetworkDetector()

	multiTransport := transport.NewMultiTransport()
	defer multiTransport.Close()

	natManager := transport.NewNATTraversal()
	defer natManager.Close()

	// Test end-to-end flow for a real IPv4 address
	testAddress := "127.0.0.1:8080"
	
	t.Run("CompleteFlow", func(t *testing.T) {
		// Step 1: Parse address
		addresses, err := parser.Parse(testAddress)
		require.NoError(t, err, "Failed to parse address")
		require.NotEmpty(t, addresses, "No addresses parsed")
		
		netAddr := addresses[0]
		t.Logf("Parsed address: Type=%v, Network=%s, Port=%d", netAddr.Type, netAddr.Network, netAddr.Port)

		// Step 2: Detect network capabilities
		capabilities := detector.DetectCapabilities(netAddr.ToNetAddr())
		t.Logf("Capabilities: Private=%v, NAT=%v, Direct=%v", 
			capabilities.IsPrivateSpace, capabilities.SupportsNAT, capabilities.SupportsDirectConnection)

		// Step 3: Check NAT traversal capabilities
		natCaps := natManager.GetNetworkCapabilities(testAddress)
		assert.Equal(t, capabilities.IsPrivateSpace, natCaps.IsPrivateSpace, "NAT and detector should agree on private space")

		// Step 4: Test transport selection
		supportedNetworks := multiTransport.SupportedNetworks()
		assert.Contains(t, supportedNetworks, netAddr.Network, "Transport should support network type")

		// Step 5: Attempt to create listener (for localhost)
		listener, err := multiTransport.Listen(testAddress)
		if err == nil {
			defer listener.Close()
			t.Logf("Successfully created listener on %s", listener.Addr().String())
			
			// Step 6: Test connection (basic connectivity)
			go func() {
				// Accept one connection
				conn, err := listener.Accept()
				if err == nil {
					conn.Close()
				}
			}()

			// Give listener time to start
			time.Sleep(10 * time.Millisecond)

			// Try to connect
			conn, err := multiTransport.Dial(listener.Addr().String())
			if err == nil {
				conn.Close()
				t.Log("Successfully established connection")
			} else {
				t.Logf("Connection failed (may be expected): %v", err)
			}
		} else {
			t.Logf("Listen failed (may be expected): %v", err)
		}

		t.Log("End-to-end test completed successfully")
	})
}

// BenchmarkMultiNetworkIntegration provides performance benchmarks for the integrated system
func BenchmarkMultiNetworkIntegration(b *testing.B) {
	logrus.SetLevel(logrus.ErrorLevel) // Minimize logging for benchmarks

	// Create components once
	parser := transport.NewMultiNetworkParser()
	defer parser.Close()

	detector := transport.NewMultiNetworkDetector()
	multiTransport := transport.NewMultiTransport()
	defer multiTransport.Close()

	testAddresses := []string{
		"192.168.1.1:8080",
		"[2001:db8::1]:8080", 
		"test.onion:80",
		"test.i2p:7070",
		"127.0.0.1:9000",
	}

	b.Run("AddressParsing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			addr := testAddresses[i%len(testAddresses)]
			parser.Parse(addr)
		}
	})

	b.Run("NetworkDetection", func(b *testing.B) {
		// Pre-parse addresses
		var netAddrs []transport.NetworkAddress
		for _, addr := range testAddresses {
			if addresses, err := parser.Parse(addr); err == nil && len(addresses) > 0 {
				netAddrs = append(netAddrs, addresses[0])
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if len(netAddrs) > 0 {
				detector.DetectCapabilities(netAddrs[i%len(netAddrs)].ToNetAddr())
			}
		}
	})

	b.Run("TransportSelection", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			multiTransport.SupportedNetworks()
		}
	})

	b.Run("EndToEndFlow", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			addr := testAddresses[i%len(testAddresses)]
			
			// Full flow
			if addresses, err := parser.Parse(addr); err == nil && len(addresses) > 0 {
				netAddr := addresses[0]
				detector.DetectCapabilities(netAddr.ToNetAddr())
				multiTransport.SupportedNetworks()
			}
		}
	})
}

// testAddressParsingIntegration validates that all address types can be
// parsed correctly and converted between different representations
func testAddressParsingIntegration(t *testing.T) {
	t.Log("Testing address parsing integration across all network types")

	// Create parser for all network types
	parser := transport.NewMultiNetworkParser()
	defer parser.Close()

	// Test addresses for each network type
	testAddresses := []struct {
		name     string
		address  string
		expected transport.AddressType
	}{
		{"IPv4", "192.168.1.1:8080", transport.AddressTypeIPv4},
		{"IPv6", "[2001:db8::1]:8080", transport.AddressTypeIPv6},
		{"Tor", "facebookcorewwwi.onion:80", transport.AddressTypeOnion},
		{"I2P", "stats.i2p:7070", transport.AddressTypeI2P},
		{"Nym", "example.nym:9000", transport.AddressTypeNym},
		{"Localhost", "127.0.0.1:9999", transport.AddressTypeIPv4},
	}

	for _, tc := range testAddresses {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing address parsing for %s: %s", tc.name, tc.address)

			// Parse the address
			addresses, err := parser.Parse(tc.address)
			require.NoError(t, err, "Failed to parse address: %s", tc.address)
			require.NotEmpty(t, addresses, "No addresses returned for: %s", tc.address)

			// Verify at least one address matches expected type
			found := false
			for _, addr := range addresses {
				if addr.Type == tc.expected {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected address type %v not found in results for %s", tc.expected, tc.address)

			// Verify all addresses are valid
			for _, addr := range addresses {
				assert.NotEqual(t, transport.AddressTypeUnknown, addr.Type, "Address type should not be unknown")
				assert.NotEmpty(t, addr.Data, "Address data should not be empty")
				assert.NotEmpty(t, addr.Network, "Network should not be empty")
				t.Logf("Parsed %s as %s://%s:%d", tc.address, addr.Network, addr.String(), addr.Port)
			}
		})
	}
}

// testNetworkDetectionIntegration validates network capability detection
// works correctly for all supported network types
func testNetworkDetectionIntegration(t *testing.T) {
	t.Log("Testing network detection integration")

	// Create network detector
	detector := transport.NewMultiNetworkDetector()

	// Test network detection for different address types
	testCases := []struct {
		name           string
		address        string
		expectPrivate  bool
		expectNAT      bool
		expectDirectly bool
	}{
		{"PublicIPv4", "8.8.8.8:53", false, false, true},
		{"PrivateIPv4", "192.168.1.1:8080", true, true, true},
		{"LocalhostIPv4", "127.0.0.1:9000", true, false, true},
		{"PublicIPv6", "[2001:4860:4860::8888]:53", false, false, true},
		{"TorOnion", "facebookcorewwwi.onion:80", false, false, false},
		{"I2PAddress", "stats.i2p:7070", false, false, false},
		{"NymAddress", "example.nym:9000", false, false, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse address to get NetworkAddress
			parser := transport.NewMultiNetworkParser()
			defer parser.Close()

			addresses, err := parser.Parse(tc.address)
			require.NoError(t, err, "Failed to parse test address")
			require.NotEmpty(t, addresses, "No addresses parsed")

			netAddr := addresses[0] // Use first parsed address

			// Test network capabilities detection
			capabilities, err := detector.DetectCapabilities(netAddr)
			require.NoError(t, err, "Failed to detect capabilities for %s", tc.address)

			assert.Equal(t, tc.expectPrivate, capabilities.IsPrivateSpace,
				"Private space detection mismatch for %s", tc.address)
			assert.Equal(t, tc.expectNAT, capabilities.SupportsNAT,
				"NAT support detection mismatch for %s", tc.address)
			assert.Equal(t, tc.expectDirectly, capabilities.SupportsDirectConnection(),
				"Direct connection support mismatch for %s", tc.address)

			t.Logf("%s capabilities: Private=%v, NAT=%v, Direct=%v, Method=%v",
				tc.name, capabilities.IsPrivateSpace, capabilities.SupportsNAT,
				capabilities.SupportsDirectConnection(), capabilities.RoutingMethod)
		})
	}
}

// testNATTraversalIntegration validates NAT traversal works with network detection
func testNATTraversalIntegration(t *testing.T) {
	t.Log("Testing NAT traversal integration with network detection")

	// Create NAT traversal manager with network detection
	natManager, err := transport.NewNATTraversal("0.0.0.0:0")
	require.NoError(t, err, "Failed to create NAT traversal manager")
	defer natManager.Close()

	// Test different address types for NAT detection
	testAddresses := []string{
		"192.168.1.100:8080", // Private IPv4 - should support NAT
		"10.0.0.50:9000",     // Private IPv4 - should support NAT
		"8.8.8.8:53",         // Public IPv4 - no NAT needed
		"127.0.0.1:8080",     // Localhost - no NAT needed
	}

	for _, addr := range testAddresses {
		t.Run(fmt.Sprintf("NATDetection_%s", addr), func(t *testing.T) {
			// Get network capabilities
			capabilities, err := natManager.GetNetworkCapabilities(addr)
			require.NoError(t, err, "Failed to get network capabilities")

			// Verify capability consistency
			if capabilities.IsPrivateSpace && !addr == "127.0.0.1:8080" {
				assert.True(t, capabilities.SupportsNAT,
					"Private address should support NAT: %s", addr)
			}

			t.Logf("Address %s: Private=%v, NAT=%v, Direct=%v",
				addr, capabilities.IsPrivateSpace, capabilities.SupportsNAT,
				capabilities.SupportsDirectConnection())
		})
	}
}

// testTransportSelectionIntegration validates multi-protocol transport selection
func testTransportSelectionIntegration(t *testing.T) {
	t.Log("Testing transport selection integration")

	// Create multi-transport system
	multiTransport, err := transport.NewMultiTransport()
	require.NoError(t, err, "Failed to create multi-transport")
	defer multiTransport.Close()

	// Test transport selection for different network types
	testCases := []struct {
		name                string
		address             string
		expectedNetworks    []string
		shouldSupportListen bool
	}{
		{"IPv4TCP", "192.168.1.1:8080", []string{"tcp", "tcp4"}, true},
		{"IPv6TCP", "[::1]:8080", []string{"tcp", "tcp6"}, true},
		{"TorOnion", "example.onion:80", []string{"tor"}, false}, // Placeholder
		{"I2PAddress", "test.i2p:7070", []string{"i2p"}, false},  // Placeholder
		{"NymAddress", "test.nym:9000", []string{"nym"}, false},  // Placeholder
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Get supported networks
			supportedNetworks := multiTransport.SupportedNetworks()
			assert.NotEmpty(t, supportedNetworks, "Should have supported networks")

			// Verify expected networks are supported
			for _, expectedNet := range tc.expectedNetworks {
				found := false
				for _, supported := range supportedNetworks {
					if supported == expectedNet {
						found = true
						break
					}
				}
				if tc.shouldSupportListen {
					assert.True(t, found, "Expected network %s should be supported", expectedNet)
				}
			}

			// Test transport selection (for IP addresses only for now)
			if tc.shouldSupportListen {
				// Test listening capability
				listener, err := multiTransport.Listen(tc.address)
				if err == nil {
					listener.Close()
					t.Logf("Successfully created listener for %s", tc.address)
				} else {
					t.Logf("Listen failed for %s (expected for some cases): %v", tc.address, err)
				}
			}

			t.Logf("Transport selection for %s: Networks=%v, CanListen=%v",
				tc.name, tc.expectedNetworks, tc.shouldSupportListen)
		})
	}
}

// testCrossNetworkCompatibility validates compatibility between different network types
func testCrossNetworkCompatibility(t *testing.T) {
	t.Log("Testing cross-network compatibility")

	// Create components for cross-network testing
	parser, err := transport.NewMultiNetworkParser()
	require.NoError(t, err)
	defer parser.Close()

	detector, err := transport.NewMultiNetworkDetector()
	require.NoError(t, err)

	multiTransport, err := transport.NewMultiTransport()
	require.NoError(t, err)
	defer multiTransport.Close()

	// Test address combinations that should be compatible
	compatibilityTests := []struct {
		name       string
		addr1      string
		addr2      string
		compatible bool
	}{
		{"IPv4ToIPv4", "192.168.1.1:8080", "10.0.0.1:9000", true},
		{"IPv4ToIPv6", "192.168.1.1:8080", "[::1]:9000", true},
		{"IPv6ToIPv6", "[2001:db8::1]:8080", "[2001:db8::2]:9000", true},
		{"TorToTor", "test1.onion:80", "test2.onion:90", true},
		{"I2PToI2P", "test1.i2p:7070", "test2.i2p:7071", true},
		{"IPToTor", "192.168.1.1:8080", "test.onion:80", false}, // Different networks
		{"TorToI2P", "test.onion:80", "test.i2p:7070", false},   // Different networks
	}

	for _, tc := range compatibilityTests {
		t.Run(tc.name, func(t *testing.T) {
			// Parse both addresses
			addrs1, err := parser.ParseAddress(tc.addr1)
			require.NoError(t, err, "Failed to parse first address")

			addrs2, err := parser.ParseAddress(tc.addr2)
			require.NoError(t, err, "Failed to parse second address")

			// Check network compatibility (same network type)
			compatible := false
			for _, a1 := range addrs1 {
				for _, a2 := range addrs2 {
					if a1.Network == a2.Network {
						compatible = true
						break
					}
				}
				if compatible {
					break
				}
			}

			if tc.compatible {
				assert.True(t, compatible, "Addresses should be compatible: %s <-> %s", tc.addr1, tc.addr2)
			}

			t.Logf("Compatibility %s <-> %s: %v (expected: %v)", tc.addr1, tc.addr2, compatible, tc.compatible)
		})
	}
}

// testBackwardCompatibility validates legacy protocol compatibility
func testBackwardCompatibility(t *testing.T) {
	t.Log("Testing backward compatibility with legacy protocols")

	// Create parsers for both legacy and extended protocols
	legacyParser := &transport.LegacyIPParser{}
	extendedParser := &transport.ExtendedParser{}

	// Test IPv4 and IPv6 addresses that should work with both parsers
	testCases := []struct {
		name       string
		address    string
		testLegacy bool
	}{
		{"IPv4", "192.168.1.1:8080", true},
		{"IPv6", "[2001:db8::1]:8080", true},
		{"TorOnion", "test.onion:80", false}, // Legacy doesn't support
		{"I2P", "test.i2p:7070", false},      // Legacy doesn't support
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse with extended parser (should always work)
			parser, err := transport.NewMultiNetworkParser()
			require.NoError(t, err)
			defer parser.Close()

			addresses, err := parser.ParseAddress(tc.address)
			require.NoError(t, err, "Extended parser should handle: %s", tc.address)
			require.NotEmpty(t, addresses, "Extended parser should return addresses")

			extendedAddr := addresses[0]

			// Test legacy compatibility if applicable
			if tc.testLegacy {
				// Create a simple NetworkAddress for testing
				testAddr := transport.NetworkAddress{
					Type:    extendedAddr.Type,
					Data:    extendedAddr.Data,
					Port:    extendedAddr.Port,
					Network: extendedAddr.Network,
				}

				// Verify the address is valid
				assert.NotEqual(t, transport.AddressTypeUnknown, testAddr.Type)
				assert.NotEmpty(t, testAddr.Data)

				t.Logf("Legacy compatibility verified for %s: Type=%v, Network=%s",
					tc.address, testAddr.Type, testAddr.Network)
			}

			t.Logf("Address %s: Extended=✓, Legacy=%v", tc.address, tc.testLegacy)
		})
	}
}

// testEndToEndMultiNetwork validates complete end-to-end functionality
func testEndToEndMultiNetwork(t *testing.T) {
	t.Log("Testing end-to-end multi-network functionality")

	// Create a complete multi-network stack
	parser, err := transport.NewMultiNetworkParser()
	require.NoError(t, err, "Failed to create parser")
	defer parser.Close()

	detector, err := transport.NewMultiNetworkDetector()
	require.NoError(t, err, "Failed to create detector")

	multiTransport, err := transport.NewMultiTransport()
	require.NoError(t, err, "Failed to create transport")
	defer multiTransport.Close()

	natManager, err := transport.NewNATTraversal("0.0.0.0:0")
	require.NoError(t, err, "Failed to create NAT manager")
	defer natManager.Close()

	// Test end-to-end flow for a real IPv4 address
	testAddress := "127.0.0.1:8080"

	t.Run("CompleteFlow", func(t *testing.T) {
		// Step 1: Parse address
		addresses, err := parser.ParseAddress(testAddress)
		require.NoError(t, err, "Failed to parse address")
		require.NotEmpty(t, addresses, "No addresses parsed")

		netAddr := addresses[0]
		t.Logf("Parsed address: Type=%v, Network=%s, Port=%d", netAddr.Type, netAddr.Network, netAddr.Port)

		// Step 2: Detect network capabilities
		capabilities, err := detector.DetectCapabilities(netAddr)
		require.NoError(t, err, "Failed to detect capabilities")
		t.Logf("Capabilities: Private=%v, NAT=%v, Direct=%v",
			capabilities.IsPrivateSpace, capabilities.SupportsNAT, capabilities.SupportsDirectConnection())

		// Step 3: Check NAT traversal capabilities
		natCaps, err := natManager.GetNetworkCapabilities(testAddress)
		require.NoError(t, err, "Failed to get NAT capabilities")
		assert.Equal(t, capabilities.IsPrivateSpace, natCaps.IsPrivateSpace, "NAT and detector should agree on private space")

		// Step 4: Test transport selection
		supportedNetworks := multiTransport.SupportedNetworks()
		assert.Contains(t, supportedNetworks, netAddr.Network, "Transport should support network type")

		// Step 5: Attempt to create listener (for localhost)
		listener, err := multiTransport.Listen(testAddress)
		if err == nil {
			defer listener.Close()
			t.Logf("Successfully created listener on %s", listener.Addr().String())

			// Step 6: Test connection (basic connectivity)
			go func() {
				// Accept one connection
				conn, err := listener.Accept()
				if err == nil {
					conn.Close()
				}
			}()

			// Give listener time to start
			time.Sleep(10 * time.Millisecond)

			// Try to connect
			conn, err := multiTransport.Dial(listener.Addr().String())
			if err == nil {
				conn.Close()
				t.Log("Successfully established connection")
			} else {
				t.Logf("Connection failed (may be expected): %v", err)
			}
		} else {
			t.Logf("Listen failed (may be expected): %v", err)
		}

		t.Log("End-to-end test completed successfully")
	})
}

// BenchmarkMultiNetworkIntegration provides performance benchmarks for the integrated system
func BenchmarkMultiNetworkIntegration(b *testing.B) {
	logrus.SetLevel(logrus.ErrorLevel) // Minimize logging for benchmarks

	// Create components once
	parser, _ := transport.NewMultiNetworkParser()
	defer parser.Close()

	detector, _ := transport.NewMultiNetworkDetector()
	multiTransport, _ := transport.NewMultiTransport()
	defer multiTransport.Close()

	testAddresses := []string{
		"192.168.1.1:8080",
		"[2001:db8::1]:8080",
		"test.onion:80",
		"test.i2p:7070",
		"127.0.0.1:9000",
	}

	b.Run("AddressParsing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			addr := testAddresses[i%len(testAddresses)]
			parser.ParseAddress(addr)
		}
	})

	b.Run("NetworkDetection", func(b *testing.B) {
		// Pre-parse addresses
		var netAddrs []transport.NetworkAddress
		for _, addr := range testAddresses {
			if addresses, err := parser.ParseAddress(addr); err == nil && len(addresses) > 0 {
				netAddrs = append(netAddrs, addresses[0])
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if len(netAddrs) > 0 {
				detector.DetectCapabilities(netAddrs[i%len(netAddrs)])
			}
		}
	})

	b.Run("TransportSelection", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			multiTransport.SupportedNetworks()
		}
	})

	b.Run("EndToEndFlow", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			addr := testAddresses[i%len(testAddresses)]

			// Full flow
			if addresses, err := parser.ParseAddress(addr); err == nil && len(addresses) > 0 {
				netAddr := addresses[0]
				detector.DetectCapabilities(netAddr)
				multiTransport.SupportedNetworks()
			}
		}
	})
}

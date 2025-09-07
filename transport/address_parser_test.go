package transport

import (
	"strings"
	"testing"
)

// TestMultiNetworkParserCreation tests the creation and initialization of MultiNetworkParser
func TestMultiNetworkParserCreation(t *testing.T) {
	parser := NewMultiNetworkParser()
	defer parser.Close()

	// Test basic functionality
	networks := parser.GetSupportedNetworks()
	if len(networks) == 0 {
		t.Error("Parser should support at least one network")
	}

	// Check that default parsers are registered
	expectedNetworks := map[string]bool{
		"ip":  true,
		"tor": true,
		"i2p": true,
		"nym": true,
	}

	for _, network := range networks {
		if !expectedNetworks[network] {
			t.Errorf("Unexpected network type: %s", network)
		}
		delete(expectedNetworks, network)
	}

	if len(expectedNetworks) > 0 {
		t.Errorf("Missing expected networks: %v", expectedNetworks)
	}
}

// TestIPAddressParserOperations tests the IP address parser functionality
func TestIPAddressParserOperations(t *testing.T) {
	parser := NewIPAddressParser()

	tests := []struct {
		name        string
		address     string
		shouldParse bool
		expectType  AddressType
	}{
		{"IPv4 localhost", "127.0.0.1:8080", true, AddressTypeIPv4},
		{"IPv4 public", "8.8.8.8:53", true, AddressTypeIPv4},
		{"IPv6 localhost", "[::1]:8080", true, AddressTypeIPv6},
		{"Hostname", "localhost:8080", true, AddressTypeIPv4}, // Should resolve to IPv4
		{"Invalid format", "not-an-address", false, AddressTypeUnknown},
		{"Missing port", "127.0.0.1", false, AddressTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test CanParse
			canParse := parser.CanParse(tt.address)
			if canParse != tt.shouldParse {
				t.Errorf("CanParse(%s) = %v, want %v", tt.address, canParse, tt.shouldParse)
			}

			if tt.shouldParse {
				// Test ParseAddress
				netAddr, err := parser.ParseAddress(tt.address)
				if err != nil {
					t.Errorf("ParseAddress(%s) failed: %v", tt.address, err)
					return
				}

				if netAddr.Type != tt.expectType {
					t.Errorf("ParseAddress(%s) type = %v, want %v", tt.address, netAddr.Type, tt.expectType)
				}

				if netAddr.Network != "ip" {
					t.Errorf("ParseAddress(%s) network = %s, want ip", tt.address, netAddr.Network)
				}

				// Test ValidateAddress
				if err := parser.ValidateAddress(netAddr); err != nil {
					t.Errorf("ValidateAddress failed for %s: %v", tt.address, err)
				}
			}
		})
	}
}

// TestTorAddressParserOperations tests the Tor address parser functionality
func TestTorAddressParserOperations(t *testing.T) {
	parser := NewTorAddressParser()

	tests := []struct {
		name        string
		address     string
		shouldParse bool
		expectValid bool
	}{
		{"Valid v2 onion", "3g2upl4pq6kufc4m.onion:80", true, true},
		{"Valid v3 onion", "duckduckgogg42ts72uq75htqjyqmp6u2mtpd6d7lw6wpfrdz0ycxhhfakid.onion:443", true, true},
		{"Missing .onion", "example:80", false, false},
		{"Invalid format", "not-an-address", false, false},
		{"Regular IP", "127.0.0.1:8080", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test CanParse
			canParse := parser.CanParse(tt.address)
			if canParse != tt.shouldParse {
				t.Errorf("CanParse(%s) = %v, want %v", tt.address, canParse, tt.shouldParse)
			}

			if tt.shouldParse {
				// Test ParseAddress
				netAddr, err := parser.ParseAddress(tt.address)
				if err != nil {
					t.Errorf("ParseAddress(%s) failed: %v", tt.address, err)
					return
				}

				if netAddr.Type != AddressTypeOnion {
					t.Errorf("ParseAddress(%s) type = %v, want %v", tt.address, netAddr.Type, AddressTypeOnion)
				}

				if netAddr.Network != "tor" {
					t.Errorf("ParseAddress(%s) network = %s, want tor", tt.address, netAddr.Network)
				}

				// Test ValidateAddress
				err = parser.ValidateAddress(netAddr)
				if tt.expectValid && err != nil {
					t.Errorf("ValidateAddress failed for valid address %s: %v", tt.address, err)
				} else if !tt.expectValid && err == nil {
					t.Errorf("ValidateAddress should have failed for invalid address %s", tt.address)
				}
			}
		})
	}
}

// TestI2PAddressParserOperations tests the I2P address parser functionality
func TestI2PAddressParserOperations(t *testing.T) {
	parser := NewI2PAddressParser()

	tests := []struct {
		name        string
		address     string
		shouldParse bool
	}{
		{"Valid I2P address", "example.b32.i2p:80", true},
		{"Valid I2P with long name", "verylongexampleaddressname.b32.i2p:443", true},
		{"Missing .i2p", "example:80", false},
		{"Regular IP", "127.0.0.1:8080", false},
		{"Tor address", "example.onion:80", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test CanParse
			canParse := parser.CanParse(tt.address)
			if canParse != tt.shouldParse {
				t.Errorf("CanParse(%s) = %v, want %v", tt.address, canParse, tt.shouldParse)
			}

			if tt.shouldParse {
				// Test ParseAddress
				netAddr, err := parser.ParseAddress(tt.address)
				if err != nil {
					t.Errorf("ParseAddress(%s) failed: %v", tt.address, err)
					return
				}

				if netAddr.Type != AddressTypeI2P {
					t.Errorf("ParseAddress(%s) type = %v, want %v", tt.address, netAddr.Type, AddressTypeI2P)
				}

				if netAddr.Network != "i2p" {
					t.Errorf("ParseAddress(%s) network = %s, want i2p", tt.address, netAddr.Network)
				}

				// Test ValidateAddress
				if err := parser.ValidateAddress(netAddr); err != nil {
					t.Errorf("ValidateAddress failed for %s: %v", tt.address, err)
				}
			}
		})
	}
}

// TestNymAddressParserOperations tests the Nym address parser functionality
func TestNymAddressParserOperations(t *testing.T) {
	parser := NewNymAddressParser()

	tests := []struct {
		name        string
		address     string
		shouldParse bool
	}{
		{"Valid Nym address", "service.nym:80", true},
		{"Valid Nym with subdomain", "example.service.nym:443", true},
		{"Missing .nym", "service:80", false},
		{"Regular IP", "127.0.0.1:8080", false},
		{"Tor address", "example.onion:80", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test CanParse
			canParse := parser.CanParse(tt.address)
			if canParse != tt.shouldParse {
				t.Errorf("CanParse(%s) = %v, want %v", tt.address, canParse, tt.shouldParse)
			}

			if tt.shouldParse {
				// Test ParseAddress
				netAddr, err := parser.ParseAddress(tt.address)
				if err != nil {
					t.Errorf("ParseAddress(%s) failed: %v", tt.address, err)
					return
				}

				if netAddr.Type != AddressTypeNym {
					t.Errorf("ParseAddress(%s) type = %v, want %v", tt.address, netAddr.Type, AddressTypeNym)
				}

				if netAddr.Network != "nym" {
					t.Errorf("ParseAddress(%s) network = %s, want nym", tt.address, netAddr.Network)
				}

				// Test ValidateAddress
				if err := parser.ValidateAddress(netAddr); err != nil {
					t.Errorf("ValidateAddress failed for %s: %v", tt.address, err)
				}
			}
		})
	}
}

// TestMultiNetworkParserParsing tests the multi-network parser coordination
func TestMultiNetworkParserParsing(t *testing.T) {
	parser := NewMultiNetworkParser()
	defer parser.Close()

	tests := []struct {
		name           string
		address        string
		expectSuccess  bool
		expectNetworks []string
	}{
		{"IPv4 address", "127.0.0.1:8080", true, []string{"ip"}},
		{"IPv6 address", "[::1]:8080", true, []string{"ip"}},
		{"Tor onion", "3g2upl4pq6kufc4m.onion:80", true, []string{"tor"}},
		{"I2P address", "example.b32.i2p:80", true, []string{"i2p"}},
		{"Nym address", "service.nym:80", true, []string{"nym"}},
		{"Invalid address", "not-an-address", false, []string{}},
		{"Missing port", "127.0.0.1", false, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := parser.Parse(tt.address)

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("Parse(%s) failed: %v", tt.address, err)
					return
				}

				if len(results) != len(tt.expectNetworks) {
					t.Errorf("Parse(%s) returned %d results, want %d", tt.address, len(results), len(tt.expectNetworks))
					return
				}

				// Check that we got the expected network types
				for i, result := range results {
					if result.Network != tt.expectNetworks[i] {
						t.Errorf("Parse(%s)[%d] network = %s, want %s", tt.address, i, result.Network, tt.expectNetworks[i])
					}
				}
			} else {
				if err == nil {
					t.Errorf("Parse(%s) should have failed but succeeded", tt.address)
				}
			}
		})
	}
}

// TestMultiNetworkParserRegistration tests custom parser registration
func TestMultiNetworkParserRegistration(t *testing.T) {
	parser := NewMultiNetworkParser()
	defer parser.Close()

	// Create a custom parser for testing
	customParser := NewTorAddressParser()

	// Register custom parser
	parser.RegisterNetwork("custom", customParser)

	// Verify registration
	networks := parser.GetSupportedNetworks()
	found := false
	for _, network := range networks {
		if network == "custom" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Custom network should be registered")
	}

	// Test GetParser
	retrieved, exists := parser.GetParser("custom")
	if !exists {
		t.Error("Should find custom parser")
	}
	if retrieved != customParser {
		t.Error("Retrieved parser should be the same instance")
	}

	// Test GetParser for non-existent parser
	_, exists = parser.GetParser("nonexistent")
	if exists {
		t.Error("Should not find non-existent parser")
	}
}

// TestAddressParsingErrorHandling tests error cases and edge conditions
func TestAddressParsingErrorHandling(t *testing.T) {
	parser := NewMultiNetworkParser()
	defer parser.Close()

	errorCases := []string{
		"",                    // Empty address
		"no-port",            // Missing port
		"256.256.256.256:80", // Invalid IP
		"invalid.onion",       // Missing port for onion
		"invalid.i2p",         // Missing port for i2p
		"invalid.nym",         // Missing port for nym
		"[invalid-ipv6]:80",   // Invalid IPv6
	}

	for _, address := range errorCases {
		t.Run(address, func(t *testing.T) {
			_, err := parser.Parse(address)
			if err == nil {
				t.Errorf("Parse(%s) should have failed", address)
			}
		})
	}
}

// TestNetworkParserInterfaces tests that all parsers implement required interfaces
func TestNetworkParserInterfaces(t *testing.T) {
	parsers := []NetworkParser{
		NewIPAddressParser(),
		NewTorAddressParser(),
		NewI2PAddressParser(),
		NewNymAddressParser(),
	}

	for i, parser := range parsers {
		t.Run(parser.GetNetworkType(), func(t *testing.T) {
			// Test interface compliance
			var _ NetworkParser = parsers[i]

			// Test GetNetworkType returns non-empty string
			networkType := parser.GetNetworkType()
			if networkType == "" {
				t.Error("GetNetworkType() should return non-empty string")
			}
		})
	}
}

// BenchmarkMultiNetworkParserParsing benchmarks address parsing performance
func BenchmarkMultiNetworkParserParsing(b *testing.B) {
	parser := NewMultiNetworkParser()
	defer parser.Close()

	addresses := []string{
		"127.0.0.1:8080",
		"[::1]:8080",
		"3g2upl4pq6kufc4m.onion:80",
		"example.b32.i2p:80",
		"service.nym:80",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		address := addresses[i%len(addresses)]
		parser.Parse(address)
	}
}

// BenchmarkIPAddressParsing benchmarks IP address parsing performance
func BenchmarkIPAddressParsing(b *testing.B) {
	parser := NewIPAddressParser()
	address := "127.0.0.1:8080"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.ParseAddress(address)
	}
}

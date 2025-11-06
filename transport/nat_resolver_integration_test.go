package transport

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNATTraversal_DetectPublicAddress(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	// Test public address detection
	publicAddr, err := nt.detectPublicAddress()

	// Note: This test may fail in environments without network interfaces
	// or where no public IP is available, which is expected behavior
	if err != nil {
		// Check that error messages are helpful
		// Accept multiple possible error messages that indicate no public address
		errStr := err.Error()
		hasAcceptableError := strings.Contains(errStr, "no suitable local address found") ||
			strings.Contains(errStr, "failed to resolve public address") ||
			strings.Contains(errStr, "failed to resolve public IP address using all available methods")

		assert.True(t, hasAcceptableError,
			"Expected error about no suitable address, got: %v", err)
		t.Logf("Expected error in test environment: %v", err)
	} else {
		// If we got an address, it should be valid
		assert.NotNil(t, publicAddr)
		t.Logf("Detected public address: %v", publicAddr)

		// Verify the detected address is supported by our resolvers
		networks := nt.addressResolver.GetSupportedNetworks()
		networkType := publicAddr.Network()

		supported := false
		for _, network := range networks {
			if network == networkType {
				supported = true
				break
			}
		}
		assert.True(t, supported, "Detected address network type %s should be supported", networkType)
	}
}

func TestNATTraversal_DetectPublicAddress_Integration(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	// Test that address resolver and network detector work together
	assert.NotNil(t, nt.addressResolver)
	assert.NotNil(t, nt.networkDetector)

	// Verify supported networks are consistent
	resolverNetworks := nt.addressResolver.GetSupportedNetworks()
	assert.NotEmpty(t, resolverNetworks)

	// Key networks should be supported
	expectedNetworks := []string{"tcp", "udp", "tor", "i2p", "nym", "loki"}
	for _, expected := range expectedNetworks {
		assert.Contains(t, resolverNetworks, expected,
			"Address resolver should support %s network", expected)
	}
}

func TestNATTraversal_AddressResolverIntegration(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	// Test with various address types to ensure resolver integration works
	testAddresses := []struct {
		name string
		addr net.Addr
	}{
		{
			name: "UDP IPv4 address",
			addr: &net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 53},
		},
		{
			name: "TCP IPv4 address",
			addr: &net.TCPAddr{IP: net.ParseIP("1.1.1.1"), Port: 80},
		},
		{
			name: "Tor address",
			addr: &mockAddr{network: "tor", address: "3g2upl4pq6kufc4m.onion:80"},
		},
		{
			name: "I2P address",
			addr: &mockAddr{network: "i2p", address: "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80"},
		},
	}

	for _, tt := range testAddresses {
		t.Run(tt.name, func(t *testing.T) {
			// Test network capabilities detection
			capabilities := nt.networkDetector.DetectCapabilities(tt.addr)
			assert.NotNil(t, capabilities)

			// Test address scoring
			score := nt.calculateAddressScore(capabilities)
			assert.GreaterOrEqual(t, score, 0)

			// For privacy networks, verify they get appropriate scores
			if capabilities.RequiresProxy {
				// Privacy networks should get lower scores for direct connection preference
				assert.LessOrEqual(t, score, 100,
					"Proxy networks should not get maximum score for direct connection")
			}
		})
	}
}

func TestNATTraversal_CalculateAddressScore_WithResolver(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	tests := []struct {
		name             string
		capabilities     NetworkCapabilities
		expectedMinScore int
		expectedMaxScore int
	}{
		{
			name: "Public IP with direct connection",
			capabilities: NetworkCapabilities{
				SupportsDirectConnection: true,
				IsPrivateSpace:           false,
				RequiresProxy:            false,
				SupportsNAT:              true,
			},
			expectedMinScore: 190, // 100 + 50 + 30 + 10
			expectedMaxScore: 190,
		},
		{
			name: "Private IP with NAT",
			capabilities: NetworkCapabilities{
				SupportsDirectConnection: false,
				IsPrivateSpace:           true,
				RequiresProxy:            false,
				SupportsNAT:              true,
			},
			expectedMinScore: 40, // 30 + 10
			expectedMaxScore: 40,
		},
		{
			name: "Tor address",
			capabilities: NetworkCapabilities{
				SupportsDirectConnection: false,
				IsPrivateSpace:           false,
				RequiresProxy:            true,
				SupportsNAT:              false,
			},
			expectedMinScore: 50, // Just the public space bonus
			expectedMaxScore: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := nt.calculateAddressScore(tt.capabilities)
			assert.GreaterOrEqual(t, score, tt.expectedMinScore)
			assert.LessOrEqual(t, score, tt.expectedMaxScore)
		})
	}
}

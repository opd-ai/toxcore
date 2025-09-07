package transport

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNATTraversal_NetworkDetectorIntegration(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()
	assert.NotNil(t, nt.networkDetector, "NetworkDetector should be initialized")
}

func TestNATTraversal_GetNetworkCapabilities(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	tests := []struct {
		name     string
		addr     net.Addr
		expected NetworkCapabilities
	}{
		{
			name: "Private IPv4 address",
			addr: &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445},
			expected: NetworkCapabilities{
				SupportsNAT:              true,
				SupportsUPnP:             true,
				IsPrivateSpace:           true,
				RoutingMethod:            RoutingNAT,
				SupportsDirectConnection: false,
				RequiresProxy:            false,
			},
		},
		{
			name: "Public IPv4 address",
			addr: &net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 53},
			expected: NetworkCapabilities{
				SupportsNAT:              false,
				SupportsUPnP:             false,
				IsPrivateSpace:           false,
				RoutingMethod:            RoutingDirect,
				SupportsDirectConnection: true,
				RequiresProxy:            false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capabilities := nt.GetNetworkCapabilities(tt.addr)
			assert.Equal(t, tt.expected, capabilities)
		})
	}
}

func TestNATTraversal_IsPrivateSpace(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	tests := []struct {
		name     string
		addr     net.Addr
		expected bool
	}{
		{
			name:     "Private IPv4",
			addr:     &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445},
			expected: true,
		},
		{
			name:     "Public IPv4",
			addr:     &net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 53},
			expected: false,
		},
		{
			name:     "Tor address",
			addr:     &mockAddr{network: "tor", address: "3g2upl4pq6kufc4m.onion:80"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nt.IsPrivateSpace(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNATTraversal_SupportsDirectConnection(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	tests := []struct {
		name     string
		addr     net.Addr
		expected bool
	}{
		{
			name:     "Private IPv4 (no direct connection)",
			addr:     &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445},
			expected: false,
		},
		{
			name:     "Public IPv4 (direct connection)",
			addr:     &net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 53},
			expected: true,
		},
		{
			name:     "Tor address (no direct connection)",
			addr:     &mockAddr{network: "tor", address: "3g2upl4pq6kufc4m.onion:80"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nt.SupportsDirectConnection(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNATTraversal_RequiresProxy(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	tests := []struct {
		name     string
		addr     net.Addr
		expected bool
	}{
		{
			name:     "IPv4 address (no proxy required)",
			addr:     &net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 53},
			expected: false,
		},
		{
			name:     "Tor address (proxy required)",
			addr:     &mockAddr{network: "tor", address: "3g2upl4pq6kufc4m.onion:80"},
			expected: true,
		},
		{
			name:     "I2P address (proxy required)",
			addr:     &mockAddr{network: "i2p", address: "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nt.RequiresProxy(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNATTraversal_DeprecatedIsPrivateAddr(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	// Test that the deprecated method still works but uses the new detector
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445}

	// Both methods should return the same result
	oldResult := nt.isPrivateAddr(addr)
	newResult := nt.IsPrivateSpace(addr)

	assert.Equal(t, newResult, oldResult, "Deprecated method should return same result as new method")
	assert.True(t, oldResult, "192.168.1.1 should be detected as private")
}

func TestNATTraversal_CalculateAddressScore(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	tests := []struct {
		name         string
		capabilities NetworkCapabilities
		expectedMin  int // Minimum expected score
	}{
		{
			name: "Public address with direct connection",
			capabilities: NetworkCapabilities{
				SupportsDirectConnection: true,
				IsPrivateSpace:           false,
				RequiresProxy:            false,
				SupportsNAT:              false,
			},
			expectedMin: 180, // 100 + 50 + 30 + 0
		},
		{
			name: "Private address with NAT support",
			capabilities: NetworkCapabilities{
				SupportsDirectConnection: false,
				IsPrivateSpace:           true,
				RequiresProxy:            false,
				SupportsNAT:              true,
			},
			expectedMin: 40, // 0 + 0 + 30 + 10
		},
		{
			name: "Proxy address (Tor/I2P)",
			capabilities: NetworkCapabilities{
				SupportsDirectConnection: false,
				IsPrivateSpace:           true,
				RequiresProxy:            true,
				SupportsNAT:              false,
			},
			expectedMin: 0, // 0 + 0 + 0 + 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := nt.calculateAddressScore(tt.capabilities)
			assert.GreaterOrEqual(t, score, tt.expectedMin, "Score should meet minimum expectation")
		})
	}
}

func TestNATTraversal_GetAddressFromInterface(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	// Create a mock interface with addresses
	iface := net.Interface{
		Index:        1,
		MTU:          1500,
		Name:         "eth0",
		HardwareAddr: nil,
		Flags:        net.FlagUp | net.FlagBroadcast,
	}

	// This test is limited by our ability to mock net.Interface.Addrs()
	// In a real scenario, this would return actual interface addresses
	addr := nt.getAddressFromInterface(iface)

	// The function should handle the case where no addresses are found
	// without panicking or returning an error
	t.Logf("Address from interface: %v", addr)
}

// Benchmark the new capability-based detection
func BenchmarkNATTraversal_GetNetworkCapabilities(b *testing.B) {
	nt := NewNATTraversal()
	defer nt.Close()
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nt.GetNetworkCapabilities(addr)
	}
}

func BenchmarkNATTraversal_IsPrivateSpace(b *testing.B) {
	nt := NewNATTraversal()
	defer nt.Close()
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nt.IsPrivateSpace(addr)
	}
}

func BenchmarkNATTraversal_DeprecatedIsPrivateAddr(b *testing.B) {
	nt := NewNATTraversal()
	defer nt.Close()
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nt.isPrivateAddr(addr)
	}
}

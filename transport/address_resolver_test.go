package transport

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMultiNetworkResolver(t *testing.T) {
	resolver := NewMultiNetworkResolver()

	assert.NotNil(t, resolver)
	assert.NotEmpty(t, resolver.resolvers)
	assert.Equal(t, 30*time.Second, resolver.defaultTimeout)
}

func TestMultiNetworkResolver_ResolvePublicAddress(t *testing.T) {
	resolver := NewMultiNetworkResolver()
	ctx := context.Background()

	tests := []struct {
		name        string
		addr        net.Addr
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "nil address",
			addr:        nil,
			shouldError: true,
			errorMsg:    "local address cannot be nil",
		},
		{
			name:        "unsupported network",
			addr:        &mockAddr{network: "unknown", address: "test:123"},
			shouldError: true,
			errorMsg:    "no resolver available for network type: unknown",
		},
		{
			name:        "valid UDP address",
			addr:        &net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 53},
			shouldError: false,
		},
		{
			name:        "tor address",
			addr:        &mockAddr{network: "tor", address: "3g2upl4pq6kufc4m.onion:80"},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.ResolvePublicAddress(ctx, tt.addr)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestMultiNetworkResolver_ResolvePublicAddress_WithTimeout(t *testing.T) {
	resolver := NewMultiNetworkResolver()

	// Test with nil context (should use default timeout)
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445}
	result, err := resolver.ResolvePublicAddress(nil, addr)

	// Private IPs may fail to resolve to public addresses if STUN/UPnP are unavailable
	// This is expected behavior in test environments
	if err != nil {
		// Should be a resolution failure error
		assert.Contains(t, err.Error(), "failed to resolve")
		t.Logf("Expected resolution failure for private IP in test environment: %v", err)
	} else {
		// If resolution succeeds (e.g., if STUN/UPnP work), result should be valid
		assert.NotNil(t, result)
		t.Logf("Successfully resolved private IP to: %v", result)
	}
}

func TestMultiNetworkResolver_selectResolver(t *testing.T) {
	resolver := NewMultiNetworkResolver()

	tests := []struct {
		name     string
		network  string
		expected bool // whether a resolver should be found
	}{
		{"TCP network", "tcp", true},
		{"UDP network", "udp", true},
		{"Tor network", "tor", true},
		{"I2P network", "i2p", true},
		{"Nym network", "nym", true},
		{"Loki network", "loki", true},
		{"Unknown network", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.selectResolver(tt.network)

			if tt.expected {
				assert.NotNil(t, result)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestMultiNetworkResolver_GetSupportedNetworks(t *testing.T) {
	resolver := NewMultiNetworkResolver()
	networks := resolver.GetSupportedNetworks()

	assert.NotEmpty(t, networks)

	// Check that key network types are supported
	expectedNetworks := []string{"tcp", "udp", "ip", "tor", "onion", "i2p", "nym", "loki"}
	for _, expected := range expectedNetworks {
		assert.Contains(t, networks, expected, "Expected network %s to be supported", expected)
	}
}

func TestIPResolver_ResolvePublicAddress(t *testing.T) {
	resolver := NewIPResolver()
	ctx := context.Background()

	tests := []struct {
		name        string
		addr        net.Addr
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "public IPv4 address",
			addr:        &net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 53},
			shouldError: false,
		},
		{
			name:        "private IPv4 address",
			addr:        &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445},
			shouldError: false, // Should find a public IP from interfaces or error gracefully
		},
		{
			name:        "TCP address",
			addr:        &net.TCPAddr{IP: net.ParseIP("8.8.8.8"), Port: 80},
			shouldError: false,
		},
		{
			name:        "unsupported address type",
			addr:        &mockAddr{network: "unknown", address: "test"},
			shouldError: true,
			errorMsg:    "unsupported address type for IP resolution",
		},
		{
			name:        "nil IP address",
			addr:        &net.UDPAddr{IP: nil, Port: 80},
			shouldError: true,
			errorMsg:    "invalid IP address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.ResolvePublicAddress(ctx, tt.addr)

			if tt.shouldError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				// Note: This test may fail if no public IP is available
				// In a real environment, this is expected behavior
				if err != nil {
					// If it errors, it should be a "no public IP found" or "failed to resolve" error
					assert.True(t, 
						strings.Contains(err.Error(), "failed to find public IP") ||
						strings.Contains(err.Error(), "failed to resolve public IP address using all available methods"),
						"Expected error about no public IP, got: %v", err)
				} else {
					assert.NotNil(t, result)
				}
			}
		})
	}
}

func TestIPResolver_SupportsNetwork(t *testing.T) {
	resolver := NewIPResolver()

	tests := []struct {
		name     string
		network  string
		expected bool
	}{
		{"TCP", "tcp", true},
		{"UDP", "udp", true},
		{"IP", "ip", true},
		{"TCP4", "tcp4", true},
		{"TCP6", "tcp6", true},
		{"UDP4", "udp4", true},
		{"UDP6", "udp6", true},
		{"TCP uppercase", "TCP", true},
		{"Unknown", "unknown", false},
		{"Tor", "tor", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.SupportsNetwork(tt.network)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIPResolver_GetResolverName(t *testing.T) {
	resolver := NewIPResolver()
	assert.Equal(t, "IP Resolver", resolver.GetResolverName())
}

func TestIPResolver_isPrivateIP(t *testing.T) {
	resolver := NewIPResolver()

	tests := []struct {
		name     string
		ip       net.IP
		expected bool
	}{
		{"Private 192.168.x.x", net.ParseIP("192.168.1.1"), true},
		{"Private 10.x.x.x", net.ParseIP("10.0.0.1"), true},
		{"Private 172.16-31.x.x", net.ParseIP("172.20.1.1"), true},
		{"Localhost", net.ParseIP("127.0.0.1"), true},
		{"Public IPv4", net.ParseIP("8.8.8.8"), false},
		{"IPv6 loopback", net.ParseIP("::1"), true},
		{"IPv6 link-local", net.ParseIP("fe80::1"), true},
		{"IPv6 public", net.ParseIP("2001:db8::1"), false},
		{"Nil IP", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.isPrivateIP(tt.ip)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTorResolver_ResolvePublicAddress(t *testing.T) {
	resolver := &TorResolver{}
	ctx := context.Background()

	tests := []struct {
		name        string
		addr        net.Addr
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "valid tor address",
			addr:        &mockAddr{network: "tor", address: "3g2upl4pq6kufc4m.onion:80"},
			shouldError: false,
		},
		{
			name:        "valid onion address",
			addr:        &mockAddr{network: "onion", address: "3g2upl4pq6kufc4m.onion:80"},
			shouldError: false,
		},
		{
			name:        "unsupported network",
			addr:        &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
			shouldError: true,
			errorMsg:    "unsupported network type for Tor resolution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolver.ResolvePublicAddress(ctx, tt.addr)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.addr, result) // Should return the same address
			}
		})
	}
}

func TestTorResolver_SupportsNetwork(t *testing.T) {
	resolver := &TorResolver{}

	tests := []struct {
		name     string
		network  string
		expected bool
	}{
		{"Tor", "tor", true},
		{"Onion", "onion", true},
		{"TOR uppercase", "TOR", true},
		{"TCP", "tcp", false},
		{"Unknown", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.SupportsNetwork(tt.network)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestI2PResolver_ResolvePublicAddress(t *testing.T) {
	resolver := &I2PResolver{}
	ctx := context.Background()

	addr := &mockAddr{network: "i2p", address: "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80"}
	result, err := resolver.ResolvePublicAddress(ctx, addr)

	assert.NoError(t, err)
	assert.Equal(t, addr, result)
}

func TestI2PResolver_SupportsNetwork(t *testing.T) {
	resolver := &I2PResolver{}

	assert.True(t, resolver.SupportsNetwork("i2p"))
	assert.True(t, resolver.SupportsNetwork("I2P"))
	assert.False(t, resolver.SupportsNetwork("tcp"))
}

func TestNymResolver_ResolvePublicAddress(t *testing.T) {
	resolver := &NymResolver{}
	ctx := context.Background()

	addr := &mockAddr{network: "nym", address: "example.nym:80"}
	result, err := resolver.ResolvePublicAddress(ctx, addr)

	assert.NoError(t, err)
	assert.Equal(t, addr, result)
}

func TestNymResolver_SupportsNetwork(t *testing.T) {
	resolver := &NymResolver{}

	assert.True(t, resolver.SupportsNetwork("nym"))
	assert.True(t, resolver.SupportsNetwork("NYM"))
	assert.False(t, resolver.SupportsNetwork("tcp"))
}

func TestLokiResolver_ResolvePublicAddress(t *testing.T) {
	resolver := &LokiResolver{}
	ctx := context.Background()

	addr := &mockAddr{network: "loki", address: "example.loki:80"}
	result, err := resolver.ResolvePublicAddress(ctx, addr)

	assert.NoError(t, err)
	assert.Equal(t, addr, result)
}

func TestLokiResolver_SupportsNetwork(t *testing.T) {
	resolver := &LokiResolver{}

	assert.True(t, resolver.SupportsNetwork("loki"))
	assert.True(t, resolver.SupportsNetwork("LOKI"))
	assert.False(t, resolver.SupportsNetwork("tcp"))
}

func TestAllResolvers_GetResolverName(t *testing.T) {
	resolvers := []struct {
		resolver PublicAddressResolver
		expected string
	}{
		{NewIPResolver(), "IP Resolver"},
		{&TorResolver{}, "Tor Resolver"},
		{&I2PResolver{}, "I2P Resolver"},
		{&NymResolver{}, "Nym Resolver"},
		{&LokiResolver{}, "Loki Resolver"},
	}

	for _, test := range resolvers {
		t.Run(test.expected, func(t *testing.T) {
			assert.Equal(t, test.expected, test.resolver.GetResolverName())
		})
	}
}

func TestIPResolver_findPublicIPFromInterfaces(t *testing.T) {
	resolver := NewIPResolver()

	// This test may fail if no public IP is available, which is expected
	result, err := resolver.findPublicIPFromInterfaces()

	if err != nil {
		// Expected in many test environments
		assert.Contains(t, err.Error(), "no public IP address found")
	} else {
		// If we found a public IP, it should be valid
		assert.NotNil(t, result)

		// Verify it's actually a public IP
		switch addr := result.(type) {
		case *net.UDPAddr:
			assert.False(t, resolver.isPrivateIP(addr.IP))
		default:
			t.Errorf("Unexpected address type: %T", result)
		}
	}
}

// Benchmark tests for performance validation
func BenchmarkMultiNetworkResolver_ResolvePublicAddress(b *testing.B) {
	resolver := NewMultiNetworkResolver()
	ctx := context.Background()
	addr := &net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 53}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolver.ResolvePublicAddress(ctx, addr)
	}
}

func BenchmarkIPResolver_isPrivateIP(b *testing.B) {
	resolver := NewIPResolver()
	ip := net.ParseIP("192.168.1.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolver.isPrivateIP(ip)
	}
}

func BenchmarkMultiNetworkResolver_selectResolver(b *testing.B) {
	resolver := NewMultiNetworkResolver()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolver.selectResolver("tcp")
	}
}

// Test with context cancellation
func TestMultiNetworkResolver_ResolvePublicAddress_ContextCancellation(t *testing.T) {
	resolver := NewMultiNetworkResolver()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445}

	// The resolver should handle cancelled context gracefully
	// Note: The current implementation doesn't explicitly check for context cancellation
	// in all resolvers, so this test mainly ensures no panic occurs
	_, err := resolver.ResolvePublicAddress(ctx, addr)
	// Error is expected, but should not panic
	if err != nil {
		t.Logf("Expected error with cancelled context: %v", err)
	}
}

// Integration test combining address resolver with network detector
func TestAddressResolver_NetworkDetectorIntegration(t *testing.T) {
	resolver := NewMultiNetworkResolver()
	detector := NewMultiNetworkDetector()
	ctx := context.Background()

	tests := []struct {
		name string
		addr net.Addr
	}{
		{
			name: "Public IP",
			addr: &net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 53},
		},
		{
			name: "Tor address",
			addr: &mockAddr{network: "tor", address: "3g2upl4pq6kufc4m.onion:80"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First detect capabilities
			capabilities := detector.DetectCapabilities(tt.addr)

			// Then resolve public address
			publicAddr, err := resolver.ResolvePublicAddress(ctx, tt.addr)

			if capabilities.RequiresProxy {
				// Proxy networks should return the same address
				assert.NoError(t, err)
				assert.Equal(t, tt.addr, publicAddr)
			} else {
				// IP networks may succeed or fail based on environment
				if err == nil {
					assert.NotNil(t, publicAddr)
				}
			}
		})
	}
}

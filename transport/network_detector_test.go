package transport

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoutingMethod_String(t *testing.T) {
	tests := []struct {
		name     string
		method   RoutingMethod
		expected string
	}{
		{"Direct routing", RoutingDirect, "Direct"},
		{"NAT routing", RoutingNAT, "NAT"},
		{"Proxy routing", RoutingProxy, "Proxy"},
		{"Mixed routing", RoutingMixed, "Mixed"},
		{"Unknown routing", RoutingMethod(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.method.String())
		})
	}
}

func TestIPNetworkDetector_DetectCapabilities(t *testing.T) {
	detector := &IPNetworkDetector{}

	tests := []struct {
		name     string
		addr     net.Addr
		expected NetworkCapabilities
	}{
		{
			name: "Private IPv4 - 192.168.x.x",
			addr: &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 33445},
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
			name: "Private IPv4 - 10.x.x.x",
			addr: &net.UDPAddr{IP: net.ParseIP("10.0.0.50"), Port: 33445},
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
			name: "Private IPv4 - 172.16-31.x.x",
			addr: &net.UDPAddr{IP: net.ParseIP("172.20.1.100"), Port: 33445},
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
			name: "Public IPv4 - 8.8.8.8",
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
		{
			name: "Localhost IPv4",
			addr: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
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
			name: "IPv6 loopback",
			addr: &net.UDPAddr{IP: net.ParseIP("::1"), Port: 8080},
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
			name: "IPv6 public",
			addr: &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 33445},
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
			capabilities := detector.DetectCapabilities(tt.addr)
			assert.Equal(t, tt.expected, capabilities)
		})
	}
}

func TestIPNetworkDetector_CanDetect(t *testing.T) {
	detector := &IPNetworkDetector{}

	tests := []struct {
		name     string
		addr     net.Addr
		expected bool
	}{
		{"UDP address", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, true},
		{"TCP address", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}, true},
		{"IP address", &net.IPAddr{IP: net.ParseIP("127.0.0.1")}, true},
		{"Unknown address", &mockAddr{network: "unknown", address: "test"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.CanDetect(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTorNetworkDetector_DetectCapabilities(t *testing.T) {
	detector := &TorNetworkDetector{}
	addr := &mockAddr{network: "tor", address: "3g2upl4pq6kufc4m.onion:80"}

	capabilities := detector.DetectCapabilities(addr)

	expected := NetworkCapabilities{
		SupportsNAT:              false,
		SupportsUPnP:             false,
		IsPrivateSpace:           true,
		RoutingMethod:            RoutingProxy,
		SupportsDirectConnection: false,
		RequiresProxy:            true,
	}

	assert.Equal(t, expected, capabilities)
}

func TestTorNetworkDetector_CanDetect(t *testing.T) {
	detector := &TorNetworkDetector{}

	tests := []struct {
		name     string
		addr     net.Addr
		expected bool
	}{
		{
			name:     "Tor network type",
			addr:     &mockAddr{network: "tor", address: "3g2upl4pq6kufc4m.onion:80"},
			expected: true,
		},
		{
			name:     "Onion network type",
			addr:     &mockAddr{network: "onion", address: "3g2upl4pq6kufc4m.onion:80"},
			expected: true,
		},
		{
			name:     "Address with .onion suffix",
			addr:     &mockAddr{network: "tcp", address: "3g2upl4pq6kufc4m.onion:80"},
			expected: true,
		},
		{
			name:     "Regular IP address",
			addr:     &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.CanDetect(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestI2PNetworkDetector_DetectCapabilities(t *testing.T) {
	detector := &I2PNetworkDetector{}
	addr := &mockAddr{network: "i2p", address: "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80"}

	capabilities := detector.DetectCapabilities(addr)

	expected := NetworkCapabilities{
		SupportsNAT:              false,
		SupportsUPnP:             false,
		IsPrivateSpace:           true,
		RoutingMethod:            RoutingProxy,
		SupportsDirectConnection: false,
		RequiresProxy:            true,
	}

	assert.Equal(t, expected, capabilities)
}

func TestI2PNetworkDetector_CanDetect(t *testing.T) {
	detector := &I2PNetworkDetector{}

	tests := []struct {
		name     string
		addr     net.Addr
		expected bool
	}{
		{
			name:     "I2P network type",
			addr:     &mockAddr{network: "i2p", address: "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80"},
			expected: true,
		},
		{
			name:     "Address with .i2p suffix",
			addr:     &mockAddr{network: "tcp", address: "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80"},
			expected: true,
		},
		{
			name:     "Regular IP address",
			addr:     &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.CanDetect(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNymNetworkDetector_CanDetect(t *testing.T) {
	detector := &NymNetworkDetector{}

	tests := []struct {
		name     string
		addr     net.Addr
		expected bool
	}{
		{
			name:     "Nym network type",
			addr:     &mockAddr{network: "nym", address: "example.nym:80"},
			expected: true,
		},
		{
			name:     "Address with .nym suffix",
			addr:     &mockAddr{network: "tcp", address: "example.nym:80"},
			expected: true,
		},
		{
			name:     "Regular IP address",
			addr:     &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.CanDetect(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLokiNetworkDetector_CanDetect(t *testing.T) {
	detector := &LokiNetworkDetector{}

	tests := []struct {
		name     string
		addr     net.Addr
		expected bool
	}{
		{
			name:     "Loki network type",
			addr:     &mockAddr{network: "loki", address: "example.loki:80"},
			expected: true,
		},
		{
			name:     "Address with .loki suffix",
			addr:     &mockAddr{network: "tcp", address: "example.loki:80"},
			expected: true,
		},
		{
			name:     "Regular IP address",
			addr:     &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.CanDetect(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMultiNetworkDetector_DetectCapabilities(t *testing.T) {
	detector := NewMultiNetworkDetector()

	tests := []struct {
		name     string
		addr     net.Addr
		expected NetworkCapabilities
	}{
		{
			name: "IPv4 private address",
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
			name: "Tor .onion address",
			addr: &mockAddr{network: "tor", address: "3g2upl4pq6kufc4m.onion:80"},
			expected: NetworkCapabilities{
				SupportsNAT:              false,
				SupportsUPnP:             false,
				IsPrivateSpace:           true,
				RoutingMethod:            RoutingProxy,
				SupportsDirectConnection: false,
				RequiresProxy:            true,
			},
		},
		{
			name: "I2P address",
			addr: &mockAddr{network: "i2p", address: "ukeu3k5oycgaauneqgtnvselmt4yemvoilkln7jpvamvfx7dnkdq.b32.i2p:80"},
			expected: NetworkCapabilities{
				SupportsNAT:              false,
				SupportsUPnP:             false,
				IsPrivateSpace:           true,
				RoutingMethod:            RoutingProxy,
				SupportsDirectConnection: false,
				RequiresProxy:            true,
			},
		},
		{
			name: "Unknown network type (fallback)",
			addr: &mockAddr{network: "unknown", address: "example.unknown:80"},
			expected: NetworkCapabilities{
				SupportsNAT:              false,
				SupportsUPnP:             false,
				IsPrivateSpace:           true,
				RoutingMethod:            RoutingDirect,
				SupportsDirectConnection: false,
				RequiresProxy:            false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capabilities := detector.DetectCapabilities(tt.addr)
			assert.Equal(t, tt.expected, capabilities)
		})
	}
}

func TestMultiNetworkDetector_SupportedNetworks(t *testing.T) {
	detector := NewMultiNetworkDetector()
	networks := detector.SupportedNetworks()

	// Check that all expected network types are supported
	expectedNetworks := []string{
		// IP detector
		"tcp", "udp", "ip", "tcp4", "tcp6", "udp4", "udp6",
		// Tor detector
		"tor", "onion",
		// I2P detector
		"i2p",
		// Nym detector
		"nym",
		// Loki detector
		"loki",
	}

	for _, expected := range expectedNetworks {
		assert.Contains(t, networks, expected, "Expected network type %s to be supported", expected)
	}
}

func TestIPNetworkDetector_isPrivateIP(t *testing.T) {
	detector := &IPNetworkDetector{}

	tests := []struct {
		name     string
		ip       net.IP
		expected bool
	}{
		{"IPv4 private 192.168.x.x", net.ParseIP("192.168.1.1"), true},
		{"IPv4 private 10.x.x.x", net.ParseIP("10.0.0.1"), true},
		{"IPv4 private 172.16-31.x.x", net.ParseIP("172.20.1.1"), true},
		{"IPv4 localhost", net.ParseIP("127.0.0.1"), true},
		{"IPv4 public", net.ParseIP("8.8.8.8"), false},
		{"IPv4 edge case 172.15.x.x (not private)", net.ParseIP("172.15.1.1"), false},
		{"IPv4 edge case 172.32.x.x (not private)", net.ParseIP("172.32.1.1"), false},
		{"IPv6 loopback", net.ParseIP("::1"), true},
		{"IPv6 link-local", net.ParseIP("fe80::1"), true},
		{"IPv6 public", net.ParseIP("2001:db8::1"), false},
		{"Nil IP", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.isPrivateIP(tt.ip)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIPNetworkDetector_SupportedNetworks(t *testing.T) {
	detector := &IPNetworkDetector{}
	networks := detector.SupportedNetworks()

	expected := []string{"tcp", "udp", "ip", "tcp4", "tcp6", "udp4", "udp6"}
	assert.Equal(t, expected, networks)
}

func TestTorNetworkDetector_SupportedNetworks(t *testing.T) {
	detector := &TorNetworkDetector{}
	networks := detector.SupportedNetworks()

	expected := []string{"tor", "onion"}
	assert.Equal(t, expected, networks)
}

func TestI2PNetworkDetector_SupportedNetworks(t *testing.T) {
	detector := &I2PNetworkDetector{}
	networks := detector.SupportedNetworks()

	expected := []string{"i2p"}
	assert.Equal(t, expected, networks)
}

func TestNymNetworkDetector_SupportedNetworks(t *testing.T) {
	detector := &NymNetworkDetector{}
	networks := detector.SupportedNetworks()

	expected := []string{"nym"}
	assert.Equal(t, expected, networks)
}

func TestLokiNetworkDetector_SupportedNetworks(t *testing.T) {
	detector := &LokiNetworkDetector{}
	networks := detector.SupportedNetworks()

	expected := []string{"loki"}
	assert.Equal(t, expected, networks)
}

func TestIPNetworkDetector_DetectCapabilities_EdgeCases(t *testing.T) {
	detector := &IPNetworkDetector{}

	tests := []struct {
		name     string
		addr     net.Addr
		expected NetworkCapabilities
	}{
		{
			name: "Unparseable address",
			addr: &mockAddr{network: "udp", address: "invalid-ip:8080"},
			expected: NetworkCapabilities{
				SupportsNAT:              false,
				SupportsUPnP:             false,
				IsPrivateSpace:           false,
				RoutingMethod:            RoutingDirect,
				SupportsDirectConnection: true,
				RequiresProxy:            false,
			},
		},
		{
			name: "TCP address",
			addr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 8080},
			expected: NetworkCapabilities{
				SupportsNAT:              true,
				SupportsUPnP:             true,
				IsPrivateSpace:           true,
				RoutingMethod:            RoutingNAT,
				SupportsDirectConnection: false,
				RequiresProxy:            false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capabilities := detector.DetectCapabilities(tt.addr)
			assert.Equal(t, tt.expected, capabilities)
		})
	}
}

// mockAddr implements net.Addr for testing
type mockAddr struct {
	network string
	address string
}

func (m *mockAddr) Network() string { return m.network }
func (m *mockAddr) String() string  { return m.address }

// Benchmark tests for performance validation
func BenchmarkIPNetworkDetector_DetectCapabilities(b *testing.B) {
	detector := &IPNetworkDetector{}
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectCapabilities(addr)
	}
}

func BenchmarkMultiNetworkDetector_DetectCapabilities(b *testing.B) {
	detector := NewMultiNetworkDetector()
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 33445}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectCapabilities(addr)
	}
}

func BenchmarkTorNetworkDetector_DetectCapabilities(b *testing.B) {
	detector := &TorNetworkDetector{}
	addr := &mockAddr{network: "tor", address: "3g2upl4pq6kufc4m.onion:80"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectCapabilities(addr)
	}
}

// TestNymNetworkDetector_DetectCapabilities tests Nym capability detection.
func TestNymNetworkDetector_DetectCapabilities(t *testing.T) {
	detector := &NymNetworkDetector{}
	addr := &mockAddr{network: "nym", address: "client.nym:1977"}

	capabilities := detector.DetectCapabilities(addr)

	expected := NetworkCapabilities{
		SupportsNAT:              false,
		SupportsUPnP:             false,
		IsPrivateSpace:           true,
		RoutingMethod:            RoutingProxy,
		SupportsDirectConnection: false,
		RequiresProxy:            true,
	}

	assert.Equal(t, expected, capabilities)
}

// TestLokiNetworkDetector_DetectCapabilities tests Loki capability detection.
func TestLokiNetworkDetector_DetectCapabilities(t *testing.T) {
	detector := &LokiNetworkDetector{}
	addr := &mockAddr{network: "loki", address: "service.loki:443"}

	capabilities := detector.DetectCapabilities(addr)

	expected := NetworkCapabilities{
		SupportsNAT:              false,
		SupportsUPnP:             false,
		IsPrivateSpace:           true,
		RoutingMethod:            RoutingProxy,
		SupportsDirectConnection: false,
		RequiresProxy:            true,
	}

	assert.Equal(t, expected, capabilities)
}

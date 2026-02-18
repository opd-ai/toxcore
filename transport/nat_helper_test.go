package transport

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNATTypeToString tests the NAT type string conversion function.
func TestNATTypeToString(t *testing.T) {
	tests := []struct {
		name     string
		natType  NATType
		expected string
	}{
		{
			name:     "Unknown NAT",
			natType:  NATTypeUnknown,
			expected: "Unknown",
		},
		{
			name:     "No NAT (Public IP)",
			natType:  NATTypeNone,
			expected: "None (Public IP)",
		},
		{
			name:     "Symmetric NAT",
			natType:  NATTypeSymmetric,
			expected: "Symmetric NAT",
		},
		{
			name:     "Restricted NAT",
			natType:  NATTypeRestricted,
			expected: "Restricted NAT",
		},
		{
			name:     "Port-Restricted NAT",
			natType:  NATTypePortRestricted,
			expected: "Port-Restricted NAT",
		},
		{
			name:     "Full Cone NAT",
			natType:  NATTypeCone,
			expected: "Full Cone NAT",
		},
		{
			name:     "Invalid NAT type",
			natType:  NATType(99),
			expected: "Invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NATTypeToString(tt.natType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNATTraversal_SetGetSTUNServers tests STUN server configuration.
func TestNATTraversal_SetGetSTUNServers(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	// Test setting STUN servers
	servers := []string{
		"stun.l.google.com:19302",
		"stun1.l.google.com:19302",
		"stun.services.mozilla.com:3478",
	}
	nt.SetSTUNServers(servers)

	// Test getting STUN servers
	retrieved := nt.GetSTUNServers()
	assert.Equal(t, len(servers), len(retrieved))
	for i, s := range servers {
		assert.Equal(t, s, retrieved[i])
	}

	// Verify that the returned slice is a copy (modifying it doesn't affect internal state)
	retrieved[0] = "modified.server:1234"
	reRetrieved := nt.GetSTUNServers()
	assert.Equal(t, "stun.l.google.com:19302", reRetrieved[0])
}

// TestNATTraversal_SetSTUNServers_Empty tests setting empty STUN server list.
func TestNATTraversal_SetSTUNServers_Empty(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	// Set empty server list
	nt.SetSTUNServers([]string{})

	// Verify empty list is returned
	servers := nt.GetSTUNServers()
	assert.Empty(t, servers)
}

// TestNATTraversal_NetworkCapabilities_Helper tests network capability detection (helper variant).
func TestNATTraversal_NetworkCapabilities_Helper(t *testing.T) {
	tests := []struct {
		name         string
		addr         net.Addr
		expectNAT    bool
		expectUPnP   bool
		expectProxy  bool
		expectDirect bool
	}{
		{
			name:         "Private IPv4",
			addr:         &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 8080},
			expectNAT:    true,
			expectUPnP:   true,
			expectProxy:  false,
			expectDirect: false,
		},
		{
			name:         "Public IPv4",
			addr:         &net.UDPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 8080},
			expectNAT:    false,
			expectUPnP:   false,
			expectProxy:  false,
			expectDirect: true,
		},
	}

	nt := NewNATTraversal()
	defer nt.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := nt.GetNetworkCapabilities(tt.addr)

			assert.Equal(t, tt.expectNAT, caps.SupportsNAT)
			assert.Equal(t, tt.expectUPnP, caps.SupportsUPnP)
			assert.Equal(t, tt.expectProxy, caps.RequiresProxy)
			assert.Equal(t, tt.expectDirect, caps.SupportsDirectConnection)
		})
	}
}

// TestNATTraversal_IsPrivateSpace_Helper tests private space detection (helper variant).
func TestNATTraversal_IsPrivateSpace_Helper(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	tests := []struct {
		name     string
		addr     net.Addr
		expected bool
	}{
		{
			name:     "Private 192.168.x.x",
			addr:     &net.UDPAddr{IP: net.IPv4(192, 168, 0, 1), Port: 0},
			expected: true,
		},
		{
			name:     "Private 10.x.x.x",
			addr:     &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 0},
			expected: true,
		},
		{
			name:     "Private 172.16-31.x.x",
			addr:     &net.UDPAddr{IP: net.IPv4(172, 16, 0, 1), Port: 0},
			expected: true,
		},
		{
			name:     "Public IP",
			addr:     &net.UDPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 0},
			expected: false,
		},
		{
			name:     "Localhost",
			addr:     &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0},
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

// TestNATTraversal_SupportsDirectConnection_Helper tests direct connection support detection (helper variant).
func TestNATTraversal_SupportsDirectConnection_Helper(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	tests := []struct {
		name     string
		addr     net.Addr
		expected bool
	}{
		{
			name:     "Private IP - no direct",
			addr:     &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 0},
			expected: false,
		},
		{
			name:     "Public IP - direct supported",
			addr:     &net.UDPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 0},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nt.SupportsDirectConnection(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNATTraversal_RequiresProxy_Helper tests proxy requirement detection (helper variant).
func TestNATTraversal_RequiresProxy_Helper(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	// Regular IP addresses don't require proxy
	addr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 8080}
	result := nt.RequiresProxy(addr)
	assert.False(t, result)
}

// TestNATTraversal_isPrivateAddr tests private address detection helper.
func TestNATTraversal_isPrivateAddr(t *testing.T) {
	nt := NewNATTraversal()
	defer nt.Close()

	tests := []struct {
		name     string
		addr     net.Addr
		expected bool
	}{
		{
			name:     "RFC1918 10.x.x.x",
			addr:     &net.UDPAddr{IP: net.IPv4(10, 255, 255, 255), Port: 0},
			expected: true,
		},
		{
			name:     "RFC1918 172.16.x.x",
			addr:     &net.UDPAddr{IP: net.IPv4(172, 31, 255, 255), Port: 0},
			expected: true,
		},
		{
			name:     "Non-private 172.32.x.x",
			addr:     &net.UDPAddr{IP: net.IPv4(172, 32, 0, 1), Port: 0},
			expected: false,
		},
		{
			name:     "RFC1918 192.168.x.x",
			addr:     &net.UDPAddr{IP: net.IPv4(192, 168, 255, 255), Port: 0},
			expected: true,
		},
		{
			name:     "Public Google DNS",
			addr:     &net.UDPAddr{IP: net.IPv4(8, 8, 4, 4), Port: 0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nt.isPrivateAddr(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

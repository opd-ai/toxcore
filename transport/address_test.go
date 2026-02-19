package transport

import (
	"net"
	"strings"
	"testing"
)

// TestAddressType_String tests the string representation of AddressType values.
func TestAddressType_String(t *testing.T) {
	tests := []struct {
		name     string
		addrType AddressType
		expected string
	}{
		{"IPv4", AddressTypeIPv4, "IPv4"},
		{"IPv6", AddressTypeIPv6, "IPv6"},
		{"Onion", AddressTypeOnion, "Onion"},
		{"I2P", AddressTypeI2P, "I2P"},
		{"Nym", AddressTypeNym, "Nym"},
		{"Loki", AddressTypeLoki, "Loki"},
		{"Unknown", AddressTypeUnknown, "Unknown"},
		{"Invalid", AddressType(99), "AddressType(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addrType.String()
			if result != tt.expected {
				t.Errorf("AddressType.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestNetworkAddress_String tests the string representation of NetworkAddress.
func TestNetworkAddress_String(t *testing.T) {
	tests := []struct {
		name     string
		addr     NetworkAddress
		expected string
	}{
		{
			name: "IPv4 with port",
			addr: NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    []byte{192, 168, 1, 1},
				Port:    8080,
				Network: "tcp",
			},
			expected: "IPv4://192.168.1.1:8080",
		},
		{
			name: "IPv6 without port",
			addr: NetworkAddress{
				Type:    AddressTypeIPv6,
				Data:    []byte("example.onion"),
				Port:    0,
				Network: "tcp",
			},
			expected: "IPv6://example.onion",
		},
		{
			name: "Onion address",
			addr: NetworkAddress{
				Type:    AddressTypeOnion,
				Data:    []byte("example.onion"),
				Port:    8080,
				Network: "tcp",
			},
			expected: "Onion://example.onion",
		},
		{
			name: "I2P address",
			addr: NetworkAddress{
				Type:    AddressTypeI2P,
				Data:    []byte("example.b32.i2p"),
				Port:    8080,
				Network: "tcp",
			},
			expected: "I2P://example.b32.i2p:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.String()
			if result != tt.expected {
				t.Errorf("NetworkAddress.String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestNetworkAddress_IsPrivate tests private address detection.
func TestNetworkAddress_IsPrivate(t *testing.T) {
	tests := []struct {
		name     string
		addr     NetworkAddress
		expected bool
	}{
		{
			name: "Private IPv4 - 192.168.x.x",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: []byte{192, 168, 1, 1},
			},
			expected: true,
		},
		{
			name: "Private IPv4 - 10.x.x.x",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: []byte{10, 0, 0, 1},
			},
			expected: true,
		},
		{
			name: "Private IPv4 - 172.16-31.x.x",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: []byte{172, 16, 0, 1},
			},
			expected: true,
		},
		{
			name: "Public IPv4 - 8.8.8.8",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: []byte{8, 8, 8, 8},
			},
			expected: false,
		},
		{
			name: "Localhost IPv4",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: []byte{127, 0, 0, 1},
			},
			expected: true,
		},
		{
			name: "Onion address (always private)",
			addr: NetworkAddress{
				Type: AddressTypeOnion,
				Data: []byte("example.onion"),
			},
			expected: true,
		},
		{
			name: "I2P address (always private)",
			addr: NetworkAddress{
				Type: AddressTypeI2P,
				Data: []byte("example.b32.i2p"),
			},
			expected: true,
		},
		{
			name: "Unknown address type (assume private)",
			addr: NetworkAddress{
				Type: AddressTypeUnknown,
				Data: []byte("unknown"),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.IsPrivate()
			if result != tt.expected {
				t.Errorf("NetworkAddress.IsPrivate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestNetworkAddress_IsRoutable tests routable address detection.
func TestNetworkAddress_IsRoutable(t *testing.T) {
	tests := []struct {
		name     string
		addr     NetworkAddress
		expected bool
	}{
		{
			name: "Private IPv4 (not routable)",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: []byte{192, 168, 1, 1},
			},
			expected: false,
		},
		{
			name: "Public IPv4 (routable)",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: []byte{8, 8, 8, 8},
			},
			expected: true,
		},
		{
			name: "Onion address (routable through Tor)",
			addr: NetworkAddress{
				Type: AddressTypeOnion,
				Data: []byte("example.onion"),
			},
			expected: true,
		},
		{
			name: "I2P address (routable through I2P)",
			addr: NetworkAddress{
				Type: AddressTypeI2P,
				Data: []byte("example.b32.i2p"),
			},
			expected: true,
		},
		{
			name: "Unknown address type (not routable)",
			addr: NetworkAddress{
				Type: AddressTypeUnknown,
				Data: []byte("unknown"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.IsRoutable()
			if result != tt.expected {
				t.Errorf("NetworkAddress.IsRoutable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestNetworkAddress_ToNetAddr tests conversion to net.Addr.
func TestNetworkAddress_ToNetAddr(t *testing.T) {
	tests := []struct {
		name        string
		addr        NetworkAddress
		expectedNet string
		expectedStr string
	}{
		{
			name: "IPv4 UDP address",
			addr: NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    []byte{192, 168, 1, 1},
				Port:    8080,
				Network: "udp",
			},
			expectedNet: "udp",
			expectedStr: "192.168.1.1:8080",
		},
		{
			name: "IPv4 TCP address",
			addr: NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    []byte{192, 168, 1, 1},
				Port:    8080,
				Network: "tcp",
			},
			expectedNet: "tcp",
			expectedStr: "192.168.1.1:8080",
		},
		{
			name: "Onion address",
			addr: NetworkAddress{
				Type:    AddressTypeOnion,
				Data:    []byte("example.onion"),
				Port:    8080,
				Network: "tcp",
			},
			expectedNet: "tcp",
			expectedStr: "example.onion:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.ToNetAddr()
			if result == nil {
				t.Fatalf("ToNetAddr() returned nil")
			}

			if result.Network() != tt.expectedNet {
				t.Errorf("ToNetAddr().Network() = %v, want %v", result.Network(), tt.expectedNet)
			}

			if result.String() != tt.expectedStr {
				t.Errorf("ToNetAddr().String() = %v, want %v", result.String(), tt.expectedStr)
			}
		})
	}
}

// TestConvertNetAddrToNetworkAddress tests conversion from net.Addr.
func TestConvertNetAddrToNetworkAddress(t *testing.T) {
	tests := []struct {
		name        string
		input       net.Addr
		expectedErr bool
		expected    *NetworkAddress
	}{
		{
			name:        "nil address",
			input:       nil,
			expectedErr: true,
		},
		{
			name: "UDP IPv4 address",
			input: &net.UDPAddr{
				IP:   net.IPv4(192, 168, 1, 1),
				Port: 8080,
			},
			expectedErr: false,
			expected: &NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    []byte{192, 168, 1, 1},
				Port:    8080,
				Network: "udp",
			},
		},
		{
			name: "TCP IPv4 address",
			input: &net.TCPAddr{
				IP:   net.IPv4(192, 168, 1, 1),
				Port: 8080,
			},
			expectedErr: false,
			expected: &NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    []byte{192, 168, 1, 1},
				Port:    8080,
				Network: "tcp",
			},
		},
		{
			name: "IPv6 address",
			input: &net.TCPAddr{
				IP:   net.ParseIP("2001:db8::1"),
				Port: 8080,
			},
			expectedErr: false,
			expected: &NetworkAddress{
				Type:    AddressTypeIPv6,
				Port:    8080,
				Network: "tcp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertNetAddrToNetworkAddress(tt.input)

			if tt.expectedErr {
				if err == nil {
					t.Errorf("ConvertNetAddrToNetworkAddress() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ConvertNetAddrToNetworkAddress() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Fatalf("ConvertNetAddrToNetworkAddress() returned nil")
			}

			if result.Type != tt.expected.Type {
				t.Errorf("ConvertNetAddrToNetworkAddress().Type = %v, want %v", result.Type, tt.expected.Type)
			}

			if result.Port != tt.expected.Port {
				t.Errorf("ConvertNetAddrToNetworkAddress().Port = %v, want %v", result.Port, tt.expected.Port)
			}

			if result.Network != tt.expected.Network {
				t.Errorf("ConvertNetAddrToNetworkAddress().Network = %v, want %v", result.Network, tt.expected.Network)
			}

			// For IPv4, check the data
			if tt.expected.Type == AddressTypeIPv4 {
				if len(result.Data) != 4 {
					t.Errorf("ConvertNetAddrToNetworkAddress().Data length = %v, want 4", len(result.Data))
				}
				for i := 0; i < 4; i++ {
					if result.Data[i] != tt.expected.Data[i] {
						t.Errorf("ConvertNetAddrToNetworkAddress().Data[%d] = %v, want %v", i, result.Data[i], tt.expected.Data[i])
					}
				}
			}
		})
	}
}

// TestCustomAddrImplementation tests the customAddr struct implementation.
func TestCustomAddrImplementation(t *testing.T) {
	addr := &customAddr{
		network: "tor",
		address: "example.onion:8080",
	}

	if addr.Network() != "tor" {
		t.Errorf("customAddr.Network() = %v, want %v", addr.Network(), "tor")
	}

	if addr.String() != "example.onion:8080" {
		t.Errorf("customAddr.String() = %v, want %v", addr.String(), "example.onion:8080")
	}
}

// TestNetworkAddress_isPrivateIPv4EdgeCases tests edge cases for IPv4 private detection.
func TestNetworkAddress_isPrivateIPv4EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "insufficient data length",
			data:     []byte{192, 168},
			expected: true, // Should return true for safety
		},
		{
			name:     "empty data",
			data:     []byte{},
			expected: true, // Should return true for safety
		},
		{
			name:     "172.15.x.x (not private)",
			data:     []byte{172, 15, 0, 1},
			expected: false,
		},
		{
			name:     "172.32.x.x (not private)",
			data:     []byte{172, 32, 0, 1},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := &NetworkAddress{
				Type: AddressTypeIPv4,
				Data: tt.data,
			}
			result := addr.isPrivateIPv4()
			if result != tt.expected {
				t.Errorf("isPrivateIPv4() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestNetworkAddress_isPrivateIPv6EdgeCases tests edge cases for IPv6 private detection.
func TestNetworkAddress_isPrivateIPv6EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "insufficient data length",
			data:     []byte{0x20, 0x01},
			expected: true, // Should return true for safety
		},
		{
			name:     "empty data",
			data:     []byte{},
			expected: true, // Should return true for safety
		},
		{
			name:     "localhost IPv6",
			data:     []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := &NetworkAddress{
				Type: AddressTypeIPv6,
				Data: tt.data,
			}
			result := addr.isPrivateIPv6()
			if result != tt.expected {
				t.Errorf("isPrivateIPv6() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// BenchmarkConvertNetAddrToNetworkAddress benchmarks the conversion function.
func BenchmarkConvertNetAddrToNetworkAddress(b *testing.B) {
	addr := &net.UDPAddr{
		IP:   net.IPv4(192, 168, 1, 1),
		Port: 8080,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ConvertNetAddrToNetworkAddress(addr)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

// BenchmarkNetworkAddress_ToNetAddr benchmarks the ToNetAddr conversion.
func BenchmarkNetworkAddress_ToNetAddr(b *testing.B) {
	na := &NetworkAddress{
		Type:    AddressTypeIPv4,
		Data:    []byte{192, 168, 1, 1},
		Port:    8080,
		Network: "udp",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := na.ToNetAddr()
		if result == nil {
			b.Fatalf("ToNetAddr returned nil")
		}
	}
}

// TestValidateAddress_IPv6LinkLocal tests that IPv6 link-local addresses are rejected.
func TestValidateAddress_IPv6LinkLocal(t *testing.T) {
	tests := []struct {
		name      string
		addr      *net.UDPAddr
		shouldErr bool
		errMsg    string
	}{
		{
			name: "link-local IPv6 address should be rejected",
			addr: &net.UDPAddr{
				IP:   net.ParseIP("fe80::1"),
				Port: 8080,
			},
			shouldErr: true,
			errMsg:    "link-local",
		},
		{
			name: "multicast IPv6 address should be rejected",
			addr: &net.UDPAddr{
				IP:   net.ParseIP("ff02::1"),
				Port: 8080,
			},
			shouldErr: true,
			errMsg:    "multicast",
		},
		{
			name: "global IPv6 address should be accepted",
			addr: &net.UDPAddr{
				IP:   net.ParseIP("2001:db8::1"),
				Port: 8080,
			},
			shouldErr: false,
		},
		{
			name: "IPv4 address should be accepted",
			addr: &net.UDPAddr{
				IP:   net.ParseIP("192.168.1.1"),
				Port: 8080,
			},
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			na, err := ConvertNetAddrToNetworkAddress(tt.addr)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("ConvertNetAddrToNetworkAddress() expected error containing %q, got nil", tt.errMsg)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ConvertNetAddrToNetworkAddress() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ConvertNetAddrToNetworkAddress() unexpected error: %v", err)
				}
				if na == nil {
					t.Errorf("ConvertNetAddrToNetworkAddress() returned nil address")
				}
			}
		})
	}
}

// contains checks if a string contains a substring (case-insensitive helper).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestNetworkAddress_IsConnectivitySupported tests connectivity support detection.
func TestNetworkAddress_IsConnectivitySupported(t *testing.T) {
	tests := []struct {
		name     string
		addr     NetworkAddress
		expected bool
	}{
		{
			name: "IPv4 address (supported)",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: []byte{8, 8, 8, 8},
			},
			expected: true,
		},
		{
			name: "IPv6 address (supported)",
			addr: NetworkAddress{
				Type: AddressTypeIPv6,
				Data: make([]byte, 16),
			},
			expected: true,
		},
		{
			name: "Onion address (supported via SOCKS5)",
			addr: NetworkAddress{
				Type: AddressTypeOnion,
				Data: []byte("example.onion"),
			},
			expected: true,
		},
		{
			name: "I2P address (supported via SAM)",
			addr: NetworkAddress{
				Type: AddressTypeI2P,
				Data: []byte("example.b32.i2p"),
			},
			expected: true,
		},
		{
			name: "Loki address (supported via SOCKS5)",
			addr: NetworkAddress{
				Type: AddressTypeLoki,
				Data: []byte("example.loki"),
			},
			expected: true,
		},
		{
			name: "Nym address (stub only - NOT supported)",
			addr: NetworkAddress{
				Type: AddressTypeNym,
				Data: []byte("example.nym"),
			},
			expected: false,
		},
		{
			name: "Unknown address type (NOT supported)",
			addr: NetworkAddress{
				Type: AddressTypeUnknown,
				Data: []byte("unknown"),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.IsConnectivitySupported()
			if result != tt.expected {
				t.Errorf("NetworkAddress.IsConnectivitySupported() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestNetworkAddress_ConnectivityStatus tests connectivity status descriptions.
func TestNetworkAddress_ConnectivityStatus(t *testing.T) {
	tests := []struct {
		name            string
		addr            NetworkAddress
		expectedContain string
	}{
		{
			name: "IPv4 address",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
			},
			expectedContain: "fully supported",
		},
		{
			name: "IPv6 address",
			addr: NetworkAddress{
				Type: AddressTypeIPv6,
			},
			expectedContain: "fully supported",
		},
		{
			name: "Onion address",
			addr: NetworkAddress{
				Type: AddressTypeOnion,
			},
			expectedContain: "SOCKS5",
		},
		{
			name: "I2P address",
			addr: NetworkAddress{
				Type: AddressTypeI2P,
			},
			expectedContain: "SAM bridge",
		},
		{
			name: "Loki address",
			addr: NetworkAddress{
				Type: AddressTypeLoki,
			},
			expectedContain: "SOCKS5",
		},
		{
			name: "Nym address",
			addr: NetworkAddress{
				Type: AddressTypeNym,
			},
			expectedContain: "not yet implemented",
		},
		{
			name: "Unknown address type",
			addr: NetworkAddress{
				Type: AddressTypeUnknown,
			},
			expectedContain: "not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.ConnectivityStatus()
			if !contains(result, tt.expectedContain) {
				t.Errorf("NetworkAddress.ConnectivityStatus() = %v, want to contain %v", result, tt.expectedContain)
			}
		})
	}
}

// TestIsConnectivitySupported_NymAddressWarnsUsers verifies that Nym addresses
// correctly report no connectivity support to help users understand that
// address parsing success does not guarantee connection capability.
func TestIsConnectivitySupported_NymAddressWarnsUsers(t *testing.T) {
	// This test validates the AUDIT.md recommendation:
	// "Consider adding a validation method that indicates whether connectivity
	// is actually supported for a given address type."

	nymAddr := NetworkAddress{
		Type:    AddressTypeNym,
		Data:    []byte("example.nym:8080"),
		Port:    8080,
		Network: "nym",
	}

	// Address is parseable and appears routable through Nym network
	if !nymAddr.IsRoutable() {
		t.Error("Nym address should appear routable through Nym network")
	}

	// But connectivity is NOT actually supported (stub implementation)
	if nymAddr.IsConnectivitySupported() {
		t.Error("Nym address should report connectivity as NOT supported (stub only)")
	}

	// ConnectivityStatus should clearly indicate the limitation
	status := nymAddr.ConnectivityStatus()
	if !contains(status, "not yet implemented") && !contains(status, "stub") {
		t.Errorf("ConnectivityStatus should indicate Nym is stub/not implemented, got: %s", status)
	}
}

// TestConvertNetAddrToNetworkAddressEdgeCases tests additional edge cases.
func TestConvertNetAddrToNetworkAddressEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       net.Addr
		expectType  AddressType
		expectedErr bool
	}{
		{
			name: "TCP IPv4 address",
			input: &net.TCPAddr{
				IP:   net.IPv4(10, 0, 0, 1),
				Port: 8080,
			},
			expectType:  AddressTypeIPv4,
			expectedErr: false,
		},
		{
			name: "TCP IPv6 address",
			input: &net.TCPAddr{
				IP:   net.ParseIP("::1"),
				Port: 8080,
			},
			expectType:  AddressTypeIPv6,
			expectedErr: false,
		},
		{
			name: "UDP IPv6 address",
			input: &net.UDPAddr{
				IP:   net.ParseIP("2001:db8::1"),
				Port: 33445,
			},
			expectType:  AddressTypeIPv6,
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertNetAddrToNetworkAddress(tt.input)
			if tt.expectedErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result.Type != tt.expectType {
				t.Errorf("Expected type %v, got %v", tt.expectType, result.Type)
			}
		})
	}
}

// TestToIPAddrEdgeCases tests edge cases for toIPAddr conversion.
func TestToIPAddrEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		addr      NetworkAddress
		expectNil bool
	}{
		{
			name: "IPv4 address",
			addr: NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    net.IPv4(127, 0, 0, 1).To4(),
				Port:    8080,
				Network: "udp",
			},
			expectNil: false,
		},
		{
			name: "IPv6 address",
			addr: NetworkAddress{
				Type:    AddressTypeIPv6,
				Data:    net.ParseIP("::1").To16(),
				Port:    8080,
				Network: "udp",
			},
			expectNil: false,
		},
		{
			name: "Unsupported type returns nil",
			addr: NetworkAddress{
				Type:    AddressTypeOnion,
				Data:    []byte("test.onion"),
				Port:    8080,
				Network: "tcp",
			},
			expectNil: true,
		},
		{
			name: "Empty data returns nil",
			addr: NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    []byte{},
				Port:    8080,
				Network: "udp",
			},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.toIPAddr()
			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil result, got %v", result)
				}
				return
			}
			if result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

// TestNetworkAddressStringEdgeCases tests edge cases for String() method.
func TestNetworkAddressStringEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		addr     NetworkAddress
		contains string
	}{
		{
			name: "Onion address",
			addr: NetworkAddress{
				Type:    AddressTypeOnion,
				Data:    []byte("example.onion"),
				Port:    80,
				Network: "tcp",
			},
			contains: "example.onion",
		},
		{
			name: "I2P address",
			addr: NetworkAddress{
				Type:    AddressTypeI2P,
				Data:    []byte("test.b32.i2p"),
				Port:    8080,
				Network: "i2p",
			},
			contains: "test.b32.i2p",
		},
		{
			name: "Unknown type",
			addr: NetworkAddress{
				Type:    AddressTypeUnknown,
				Data:    []byte("unknown-data"),
				Port:    0,
				Network: "unknown",
			},
			contains: "unknown-data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.String()
			if !strings.Contains(result, tt.contains) {
				t.Errorf("String() = %q, want to contain %q", result, tt.contains)
			}
		})
	}
}

// TestToNetAddrEdgeCases tests additional edge cases for ToNetAddr.
func TestToNetAddrEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		addr      NetworkAddress
		expectNil bool
	}{
		{
			name: "TCP IPv4 address",
			addr: NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    net.IPv4(192, 168, 1, 1).To4(),
				Port:    443,
				Network: "tcp",
			},
			expectNil: false,
		},
		{
			name: "TCP6 IPv6 address",
			addr: NetworkAddress{
				Type:    AddressTypeIPv6,
				Data:    net.ParseIP("2001:db8::1").To16(),
				Port:    443,
				Network: "tcp6",
			},
			expectNil: false,
		},
		{
			name: "UDP4 IPv4 address",
			addr: NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    net.IPv4(10, 0, 0, 1).To4(),
				Port:    53,
				Network: "udp4",
			},
			expectNil: false,
		},
		{
			name: "Onion address returns custom addr",
			addr: NetworkAddress{
				Type:    AddressTypeOnion,
				Data:    []byte("example.onion"),
				Port:    80,
				Network: "tcp",
			},
			expectNil: false, // ToNetAddr returns customAddr for onion
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.ToNetAddr()
			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Error("Expected non-nil result")
				}
			}
		})
	}
}

// TestIsPrivateEdgeCases tests edge cases for IsPrivate.
func TestIsPrivateEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		addr     NetworkAddress
		expected bool
	}{
		{
			name: "Private IPv4 10.x.x.x",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: net.IPv4(10, 255, 255, 255).To4(),
			},
			expected: true,
		},
		{
			name: "Private IPv4 172.16.x.x",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: net.IPv4(172, 16, 0, 1).To4(),
			},
			expected: true,
		},
		{
			name: "Private IPv4 192.168.x.x",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: net.IPv4(192, 168, 100, 200).To4(),
			},
			expected: true,
		},
		{
			name: "Public IPv4",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: net.IPv4(8, 8, 8, 8).To4(),
			},
			expected: false,
		},
		{
			name: "Private IPv6 fc00::",
			addr: NetworkAddress{
				Type: AddressTypeIPv6,
				Data: net.ParseIP("fc00::1").To16(),
			},
			expected: true,
		},
		{
			name: "Localhost IPv6",
			addr: NetworkAddress{
				Type: AddressTypeIPv6,
				Data: net.ParseIP("::1").To16(),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.IsPrivate()
			if result != tt.expected {
				t.Errorf("IsPrivate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestParseIPAddressEdgeCases tests additional edge cases for parseIPAddress.
func TestParseIPAddressEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		addr       net.Addr
		network    string
		expectType AddressType
		expectErr  bool
	}{
		{
			name:       "TCP IPv4",
			addr:       &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 80},
			network:    "tcp",
			expectType: AddressTypeIPv4,
			expectErr:  false,
		},
		{
			name:       "TCP IPv6 full",
			addr:       &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 443},
			network:    "tcp",
			expectType: AddressTypeIPv6,
			expectErr:  false,
		},
		{
			name:       "UDP IPv4",
			addr:       &net.UDPAddr{IP: net.IPv4(192, 168, 0, 1), Port: 33445},
			network:    "udp",
			expectType: AddressTypeIPv4,
			expectErr:  false,
		},
		{
			name:       "IP only address",
			addr:       &net.IPAddr{IP: net.IPv4(127, 0, 0, 1)},
			network:    "ip",
			expectType: AddressTypeIPv4,
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseIPAddress(tt.addr, tt.network)
			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result.Type != tt.expectType {
				t.Errorf("Expected type %v, got %v", tt.expectType, result.Type)
			}
		})
	}
}

// TestToIPAddrNetworkTypes tests toIPAddr with different network types.
func TestToIPAddrNetworkTypes(t *testing.T) {
	tests := []struct {
		name    string
		addr    NetworkAddress
		network string
	}{
		{
			name: "TCP network",
			addr: NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    net.IPv4(192, 168, 1, 1).To4(),
				Port:    8080,
				Network: "tcp",
			},
		},
		{
			name: "TCP4 network",
			addr: NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    net.IPv4(10, 0, 0, 1).To4(),
				Port:    443,
				Network: "tcp4",
			},
		},
		{
			name: "TCP6 network",
			addr: NetworkAddress{
				Type:    AddressTypeIPv6,
				Data:    net.ParseIP("2001:db8::1").To16(),
				Port:    443,
				Network: "tcp6",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.toIPAddr()
			if result == nil {
				t.Error("Expected non-nil result")
				return
			}
			// Only exact "tcp" network returns TCPAddr, all others return UDPAddr
			if tt.addr.Network == "tcp" {
				if _, ok := result.(*net.TCPAddr); !ok {
					t.Errorf("Expected *net.TCPAddr for %s network, got %T", tt.addr.Network, result)
				}
			} else {
				if _, ok := result.(*net.UDPAddr); !ok {
					t.Errorf("Expected *net.UDPAddr for %s network, got %T", tt.addr.Network, result)
				}
			}
		})
	}
}

// TestConnectivityStatusAllTypes tests ConnectivityStatus for all address types.
func TestConnectivityStatusAllTypes(t *testing.T) {
	tests := []struct {
		name     string
		addrType AddressType
		contains string
	}{
		{
			name:     "IPv4",
			addrType: AddressTypeIPv4,
			contains: "fully supported",
		},
		{
			name:     "IPv6",
			addrType: AddressTypeIPv6,
			contains: "fully supported",
		},
		{
			name:     "Onion",
			addrType: AddressTypeOnion,
			contains: "Tor SOCKS5",
		},
		{
			name:     "I2P",
			addrType: AddressTypeI2P,
			contains: "I2P SAM",
		},
		{
			name:     "Loki",
			addrType: AddressTypeLoki,
			contains: "Lokinet",
		},
		{
			name:     "Nym",
			addrType: AddressTypeNym,
			contains: "Nym SDK",
		},
		{
			name:     "Unknown",
			addrType: AddressTypeUnknown,
			contains: "unknown address type",
		},
		{
			name:     "Invalid type",
			addrType: AddressType(255),
			contains: "unknown address type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := &NetworkAddress{Type: tt.addrType}
			status := addr.ConnectivityStatus()
			if !strings.Contains(status, tt.contains) {
				t.Errorf("ConnectivityStatus() = %q, want to contain %q", status, tt.contains)
			}
		})
	}
}

// TestNetworkAddressStringShortData tests String() with insufficient data length.
func TestNetworkAddressStringShortData(t *testing.T) {
	tests := []struct {
		name     string
		addr     NetworkAddress
		contains string
	}{
		{
			name: "IPv4 with short data fallback",
			addr: NetworkAddress{
				Type: AddressTypeIPv4,
				Data: []byte{1, 2},
				Port: 0,
			},
			contains: "IPv4://",
		},
		{
			name: "IPv6 with short data fallback",
			addr: NetworkAddress{
				Type: AddressTypeIPv6,
				Data: []byte{0x20},
				Port: 0,
			},
			contains: "IPv6://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.String()
			if !strings.Contains(result, tt.contains) {
				t.Errorf("String() = %q, want to contain %q", result, tt.contains)
			}
		})
	}
}

// TestConvertNetAddrUnknownType tests ConvertNetAddrToNetworkAddress with unknown address type.
func TestConvertNetAddrUnknownType(t *testing.T) {
	// Test with custom net.Addr that parseIPAddress can't handle
	customAddr := &customAddr{
		network: "custom",
		address: "some-custom-address",
	}

	na, err := ConvertNetAddrToNetworkAddress(customAddr)
	if err != nil {
		t.Errorf("Expected no error for custom address, got %v", err)
	}
	if na == nil {
		t.Fatal("Expected non-nil NetworkAddress")
	}
	if na.Type != AddressTypeUnknown {
		t.Errorf("Expected AddressTypeUnknown, got %v", na.Type)
	}
}

// TestToNetAddrDefault tests ToNetAddr with default/unknown address type.
func TestToNetAddrDefault(t *testing.T) {
	addr := NetworkAddress{
		Type:    AddressTypeUnknown,
		Data:    []byte("custom-data"),
		Port:    1234,
		Network: "unknown",
	}

	result := addr.ToNetAddr()
	if result == nil {
		t.Fatal("Expected non-nil net.Addr")
	}
	// Default case returns customAddr
	if result.Network() != "unknown" {
		t.Errorf("Expected network 'unknown', got %q", result.Network())
	}
}

// TestToIPAddrEmptyData tests toIPAddr with empty data.
func TestToIPAddrEmptyData(t *testing.T) {
	addr := NetworkAddress{
		Type:    AddressTypeIPv4,
		Data:    []byte{}, // Empty data
		Port:    8080,
		Network: "udp",
	}

	result := addr.toIPAddr()
	if result != nil {
		t.Errorf("Expected nil for empty data, got %v", result)
	}
}

// TestToIPAddrInsufficientIPv4Data tests toIPAddr with insufficient IPv4 data.
func TestToIPAddrInsufficientIPv4Data(t *testing.T) {
	addr := NetworkAddress{
		Type:    AddressTypeIPv4,
		Data:    []byte{1, 2}, // Less than 4 bytes
		Port:    8080,
		Network: "udp",
	}

	result := addr.toIPAddr()
	if result != nil {
		t.Errorf("Expected nil for insufficient IPv4 data, got %v", result)
	}
}

// TestToIPAddrInsufficientIPv6Data tests toIPAddr with insufficient IPv6 data.
func TestToIPAddrInsufficientIPv6Data(t *testing.T) {
	addr := NetworkAddress{
		Type:    AddressTypeIPv6,
		Data:    []byte{0x20, 0x01, 0x0d, 0xb8}, // Less than 16 bytes
		Port:    443,
		Network: "udp",
	}

	result := addr.toIPAddr()
	if result != nil {
		t.Errorf("Expected nil for insufficient IPv6 data, got %v", result)
	}
}

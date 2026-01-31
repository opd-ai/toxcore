package transport

import (
	"net"
	"testing"
)

// TestNetworkAddressToBytes tests the ToBytes method for NetworkAddress.
func TestNetworkAddressToBytes(t *testing.T) {
	tests := []struct {
		name      string
		addr      *NetworkAddress
		wantBytes []byte
		wantErr   bool
	}{
		{
			name: "IPv4 address with port",
			addr: &NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    []byte{192, 168, 1, 100},
				Port:    8080,
				Network: "udp",
			},
			wantBytes: []byte{192, 168, 1, 100, 0x1f, 0x90}, // 8080 = 0x1f90
			wantErr:   false,
		},
		{
			name: "IPv4 with port 0",
			addr: &NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    []byte{10, 0, 0, 1},
				Port:    0,
				Network: "udp",
			},
			wantBytes: []byte{10, 0, 0, 1, 0, 0},
			wantErr:   false,
		},
		{
			name: "IPv6 address with port",
			addr: &NetworkAddress{
				Type:    AddressTypeIPv6,
				Data:    []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
				Port:    443,
				Network: "udp",
			},
			wantBytes: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0x01, 0xbb}, // 443 = 0x01bb
			wantErr:   false,
		},
		{
			name: "invalid IPv4 length",
			addr: &NetworkAddress{
				Type:    AddressTypeIPv4,
				Data:    []byte{192, 168, 1}, // Only 3 bytes
				Port:    8080,
				Network: "udp",
			},
			wantBytes: nil,
			wantErr:   true,
		},
		{
			name: "invalid IPv6 length",
			addr: &NetworkAddress{
				Type:    AddressTypeIPv6,
				Data:    []byte{0x20, 0x01, 0x0d, 0xb8}, // Only 4 bytes
				Port:    443,
				Network: "udp",
			},
			wantBytes: nil,
			wantErr:   true,
		},
		{
			name: "unsupported address type",
			addr: &NetworkAddress{
				Type:    AddressTypeOnion,
				Data:    []byte("example.onion"),
				Port:    9050,
				Network: "tor",
			},
			wantBytes: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := tt.addr.ToBytes()
			if (err != nil) != tt.wantErr {
				t.Errorf("NetworkAddress.ToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(gotBytes) != len(tt.wantBytes) {
					t.Errorf("NetworkAddress.ToBytes() length = %d, want %d", len(gotBytes), len(tt.wantBytes))
					return
				}
				for i := range gotBytes {
					if gotBytes[i] != tt.wantBytes[i] {
						t.Errorf("NetworkAddress.ToBytes() byte[%d] = %d, want %d", i, gotBytes[i], tt.wantBytes[i])
					}
				}
			}
		})
	}
}

// TestSerializeNetAddrToBytes tests the SerializeNetAddrToBytes function.
func TestSerializeNetAddrToBytes(t *testing.T) {
	tests := []struct {
		name      string
		addr      net.Addr
		wantBytes []byte
		wantErr   bool
	}{
		{
			name:      "UDP IPv4 address",
			addr:      &net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 8080},
			wantBytes: []byte{192, 168, 1, 100, 0x1f, 0x90},
			wantErr:   false,
		},
		{
			name:      "TCP IPv4 address",
			addr:      &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 443},
			wantBytes: []byte{10, 0, 0, 1, 0x01, 0xbb},
			wantErr:   false,
		},
		{
			name:      "UDP IPv6 address",
			addr:      &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 9000},
			wantBytes: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0x23, 0x28},
			wantErr:   false,
		},
		{
			name:      "nil address",
			addr:      nil,
			wantBytes: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBytes, err := SerializeNetAddrToBytes(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("SerializeNetAddrToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(gotBytes) != len(tt.wantBytes) {
					t.Errorf("SerializeNetAddrToBytes() length = %d, want %d", len(gotBytes), len(tt.wantBytes))
					return
				}
				for i := range gotBytes {
					if gotBytes[i] != tt.wantBytes[i] {
						t.Errorf("SerializeNetAddrToBytes() byte[%d] = %d, want %d", i, gotBytes[i], tt.wantBytes[i])
					}
				}
			}
		})
	}
}

// TestSerializeNetAddrToBytes_PortEncoding verifies port encoding is big-endian.
func TestSerializeNetAddrToBytes_PortEncoding(t *testing.T) {
	tests := []struct {
		port     int
		expected [2]byte // High byte, low byte
	}{
		{port: 0, expected: [2]byte{0x00, 0x00}},
		{port: 1, expected: [2]byte{0x00, 0x01}},
		{port: 255, expected: [2]byte{0x00, 0xff}},
		{port: 256, expected: [2]byte{0x01, 0x00}},
		{port: 8080, expected: [2]byte{0x1f, 0x90}},
		{port: 65535, expected: [2]byte{0xff, 0xff}},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: tt.port}
			bytes, err := SerializeNetAddrToBytes(addr)
			if err != nil {
				t.Fatalf("SerializeNetAddrToBytes() error = %v", err)
			}
			// IPv4 is 4 bytes, port is last 2 bytes
			if len(bytes) != 6 {
				t.Fatalf("expected 6 bytes, got %d", len(bytes))
			}
			// Check port encoding (bytes 4 and 5)
			if bytes[4] != tt.expected[0] || bytes[5] != tt.expected[1] {
				t.Errorf("port %d: got [%02x %02x], want [%02x %02x]",
					tt.port, bytes[4], bytes[5], tt.expected[0], tt.expected[1])
			}
		})
	}
}

// TestSerializeNetAddrToBytes_IPv4vsIPv6 verifies correct handling of IPv4 and IPv6.
func TestSerializeNetAddrToBytes_IPv4vsIPv6(t *testing.T) {
	ipv4Addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 8080}
	ipv4Bytes, err := SerializeNetAddrToBytes(ipv4Addr)
	if err != nil {
		t.Fatalf("IPv4 serialization failed: %v", err)
	}
	if len(ipv4Bytes) != 6 {
		t.Errorf("IPv4 bytes length = %d, want 6", len(ipv4Bytes))
	}

	ipv6Addr := &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 8080}
	ipv6Bytes, err := SerializeNetAddrToBytes(ipv6Addr)
	if err != nil {
		t.Fatalf("IPv6 serialization failed: %v", err)
	}
	if len(ipv6Bytes) != 18 {
		t.Errorf("IPv6 bytes length = %d, want 18", len(ipv6Bytes))
	}

	// Verify port is the same for both (last 2 bytes)
	if ipv4Bytes[4] != ipv6Bytes[16] || ipv4Bytes[5] != ipv6Bytes[17] {
		t.Errorf("port encoding mismatch: IPv4 [%02x %02x], IPv6 [%02x %02x]",
			ipv4Bytes[4], ipv4Bytes[5], ipv6Bytes[16], ipv6Bytes[17])
	}
}

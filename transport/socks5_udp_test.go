package transport

import (
	"encoding/binary"
	"net"
	"testing"
)

// TestBuildUDPHeader tests the UDP header building function.
func TestBuildUDPHeader(t *testing.T) {
	// Create a minimal association for testing the header builder
	association := &SOCKS5UDPAssociation{}

	tests := []struct {
		name       string
		addr       net.Addr
		expectLen  int
		expectType byte
	}{
		{
			name:       "IPv4 address",
			addr:       &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 8080},
			expectLen:  10,
			expectType: socks5AddrTypeIPv4,
		},
		{
			name:       "IPv6 address",
			addr:       &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 8080},
			expectLen:  22,
			expectType: socks5AddrTypeIPv6,
		},
		{
			name:       "IPv4 localhost",
			addr:       &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33445},
			expectLen:  10,
			expectType: socks5AddrTypeIPv4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header, err := association.buildUDPHeader(tt.addr)
			if err != nil {
				t.Fatalf("buildUDPHeader failed: %v", err)
			}

			if len(header) != tt.expectLen {
				t.Errorf("Expected header length %d, got %d", tt.expectLen, len(header))
			}

			// Verify RSV and FRAG fields
			if header[0] != 0x00 || header[1] != 0x00 {
				t.Errorf("RSV fields not zero: %x %x", header[0], header[1])
			}
			if header[2] != 0x00 {
				t.Errorf("FRAG field not zero: %x", header[2])
			}

			if header[3] != tt.expectType {
				t.Errorf("Expected address type %d, got %d", tt.expectType, header[3])
			}

			// Verify port is correctly encoded
			udpAddr := tt.addr.(*net.UDPAddr)
			var portOffset int
			if tt.expectType == socks5AddrTypeIPv4 {
				portOffset = 8
			} else {
				portOffset = 20
			}
			encodedPort := binary.BigEndian.Uint16(header[portOffset:])
			if int(encodedPort) != udpAddr.Port {
				t.Errorf("Expected port %d, got %d", udpAddr.Port, encodedPort)
			}
		})
	}
}

// TestParseUDPHeader tests the UDP header parsing function.
func TestParseUDPHeader(t *testing.T) {
	association := &SOCKS5UDPAssociation{}

	tests := []struct {
		name        string
		header      []byte
		expectAddr  string
		expectLen   int
		expectError bool
	}{
		{
			name: "Valid IPv4 header",
			header: []byte{
				0x00, 0x00, // RSV
				0x00,           // FRAG
				0x01,           // ATYP (IPv4)
				192, 168, 1, 1, // IP
				0x1F, 0x90, // Port (8080)
				'h', 'e', 'l', 'l', 'o', // payload
			},
			expectAddr:  "192.168.1.1:8080",
			expectLen:   10,
			expectError: false,
		},
		{
			name: "Valid IPv6 header",
			header: []byte{
				0x00, 0x00, // RSV
				0x00, // FRAG
				0x04, // ATYP (IPv6)
				// ::1 in IPv6
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
				0x1F, 0x90, // Port (8080)
			},
			expectAddr:  "[::1]:8080",
			expectLen:   22,
			expectError: false,
		},
		{
			name: "Header too short",
			header: []byte{
				0x00, 0x00, 0x00,
			},
			expectError: true,
		},
		{
			name: "Invalid address type",
			header: []byte{
				0x00, 0x00, // RSV
				0x00, // FRAG
				0xFF, // Invalid ATYP
				192, 168, 1, 1,
				0x1F, 0x90,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, headerLen, err := association.parseUDPHeader(tt.header)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("parseUDPHeader failed: %v", err)
			}

			if headerLen != tt.expectLen {
				t.Errorf("Expected header length %d, got %d", tt.expectLen, headerLen)
			}

			if addr.String() != tt.expectAddr {
				t.Errorf("Expected address %s, got %s", tt.expectAddr, addr.String())
			}
		})
	}
}

// TestSOCKS5Constants verifies the SOCKS5 constants are correct per RFC 1928.
func TestSOCKS5Constants(t *testing.T) {
	// Verify address type constants
	if socks5AddrTypeIPv4 != 0x01 {
		t.Errorf("socks5AddrTypeIPv4 should be 0x01, got 0x%02x", socks5AddrTypeIPv4)
	}
	if socks5AddrTypeDomain != 0x03 {
		t.Errorf("socks5AddrTypeDomain should be 0x03, got 0x%02x", socks5AddrTypeDomain)
	}
	if socks5AddrTypeIPv6 != 0x04 {
		t.Errorf("socks5AddrTypeIPv6 should be 0x04, got 0x%02x", socks5AddrTypeIPv6)
	}

	// Verify command constants
	if socks5CmdConnect != 0x01 {
		t.Errorf("socks5CmdConnect should be 0x01, got 0x%02x", socks5CmdConnect)
	}
	if socks5CmdBind != 0x02 {
		t.Errorf("socks5CmdBind should be 0x02, got 0x%02x", socks5CmdBind)
	}
	if socks5CmdUDPAssociate != 0x03 {
		t.Errorf("socks5CmdUDPAssociate should be 0x03, got 0x%02x", socks5CmdUDPAssociate)
	}

	// Verify reply constants
	if socks5ReplySuccess != 0x00 {
		t.Errorf("socks5ReplySuccess should be 0x00, got 0x%02x", socks5ReplySuccess)
	}
	if socks5ReplyGeneralFailure != 0x01 {
		t.Errorf("socks5ReplyGeneralFailure should be 0x01, got 0x%02x", socks5ReplyGeneralFailure)
	}
}

// TestRoundTripUDPHeader tests building and parsing a header to ensure consistency.
func TestRoundTripUDPHeader(t *testing.T) {
	association := &SOCKS5UDPAssociation{}

	addresses := []net.Addr{
		&net.UDPAddr{IP: net.ParseIP("192.168.1.100"), Port: 12345},
		&net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 80},
		&net.UDPAddr{IP: net.ParseIP("2001:db8::8a2e:370:7334"), Port: 443},
		&net.UDPAddr{IP: net.ParseIP("::1"), Port: 33445},
	}

	for _, addr := range addresses {
		t.Run(addr.String(), func(t *testing.T) {
			// Build header
			header, err := association.buildUDPHeader(addr)
			if err != nil {
				t.Fatalf("buildUDPHeader failed: %v", err)
			}

			// Parse header back
			parsedAddr, _, err := association.parseUDPHeader(header)
			if err != nil {
				t.Fatalf("parseUDPHeader failed: %v", err)
			}

			// Compare addresses
			if parsedAddr.String() != addr.String() {
				t.Errorf("Round-trip address mismatch: got %s, want %s", parsedAddr.String(), addr.String())
			}
		})
	}
}

// TestSOCKS5UDPAssociationClosedState tests that operations fail on closed association.
func TestSOCKS5UDPAssociationClosedState(t *testing.T) {
	association := &SOCKS5UDPAssociation{
		closed: true,
	}

	// Test SendUDP on closed association
	err := association.SendUDP([]byte("test"), &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080})
	if err == nil {
		t.Errorf("Expected error on SendUDP with closed association")
	}

	// Test ReceiveUDP on closed association
	buf := make([]byte, 1024)
	_, _, err = association.ReceiveUDP(buf)
	if err == nil {
		t.Errorf("Expected error on ReceiveUDP with closed association")
	}

	// Test IsClosed
	if !association.IsClosed() {
		t.Errorf("Expected IsClosed to return true")
	}
}

// TestIsTimeoutError tests the timeout error detection function.
func TestIsTimeoutError(t *testing.T) {
	// Test with nil error
	if isTimeoutError(nil) {
		t.Errorf("nil should not be a timeout error")
	}

	// Test with regular error
	regularErr := net.UnknownNetworkError("test")
	if isTimeoutError(regularErr) {
		t.Errorf("regular error should not be a timeout error")
	}
}

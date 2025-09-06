package transport

import (
	"bytes"
	"testing"
	"time"
)

// TestNodeEntry_String tests the string representation of NodeEntry.
func TestNodeEntry_String(t *testing.T) {
	entry := &NodeEntry{
		PublicKey: [32]byte{0x01, 0x02, 0x03, 0x04},
		Address: &NetworkAddress{
			Type:    AddressTypeIPv4,
			Data:    []byte{192, 168, 1, 1},
			Port:    8080,
			Network: "udp",
		},
		LastSeen: time.Date(2025, 9, 6, 15, 30, 45, 0, time.UTC),
	}

	result := entry.String()
	expected := "Node{PubKey: 01020304..., Address: IPv4://192.168.1.1:8080, LastSeen: 15:30:45}"

	if result != expected {
		t.Errorf("NodeEntry.String() = %q, want %q", result, expected)
	}
}

// TestNodeEntry_IsExpired tests the expiration check functionality.
func TestNodeEntry_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		lastSeen time.Time
		timeout  time.Duration
		expected bool
	}{
		{
			name:     "recently seen node",
			lastSeen: now.Add(-1 * time.Minute),
			timeout:  5 * time.Minute,
			expected: false,
		},
		{
			name:     "expired node",
			lastSeen: now.Add(-10 * time.Minute),
			timeout:  5 * time.Minute,
			expected: true,
		},
		{
			name:     "exactly at timeout",
			lastSeen: now.Add(-5 * time.Minute),
			timeout:  5 * time.Minute,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &NodeEntry{LastSeen: tt.lastSeen}
			result := entry.IsExpired(tt.timeout)
			if result != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestLegacyIPParser_ParseNodeEntry tests parsing legacy IP-based node entries.
func TestLegacyIPParser_ParseNodeEntry(t *testing.T) {
	parser := NewLegacyIPParser()

	// Create test data for IPv4 node entry
	data := make([]byte, 50)

	// Public key
	pubkey := [32]byte{0x01, 0x02, 0x03, 0x04}
	copy(data[0:32], pubkey[:])

	// IPv4 address in IPv4-mapped IPv6 format (192.168.1.1)
	data[42] = 0xff // byte 10
	data[43] = 0xff // byte 11
	data[44] = 192  // IPv4 byte 0
	data[45] = 168  // IPv4 byte 1
	data[46] = 1    // IPv4 byte 2
	data[47] = 1    // IPv4 byte 3

	// Port (8080 = 0x1F90)
	data[48] = 0x1F
	data[49] = 0x90

	entry, nextOffset, err := parser.ParseNodeEntry(data, 0)
	if err != nil {
		t.Fatalf("ParseNodeEntry failed: %v", err)
	}

	if nextOffset != 50 {
		t.Errorf("Expected next offset 50, got %d", nextOffset)
	}

	if entry.PublicKey != pubkey {
		t.Errorf("Public key mismatch")
	}

	if entry.Address.Type != AddressTypeIPv4 {
		t.Errorf("Expected IPv4 address type, got %s", entry.Address.Type.String())
	}

	if entry.Address.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", entry.Address.Port)
	}

	if !bytes.Equal(entry.Address.Data, []byte{192, 168, 1, 1}) {
		t.Errorf("Expected IPv4 data [192 168 1 1], got %v", entry.Address.Data)
	}
}

// TestLegacyIPParser_ParseNodeEntry_IPv6 tests parsing IPv6 addresses.
func TestLegacyIPParser_ParseNodeEntry_IPv6(t *testing.T) {
	parser := NewLegacyIPParser()

	data := make([]byte, 50)

	// Public key
	pubkey := [32]byte{0xAA, 0xBB, 0xCC, 0xDD}
	copy(data[0:32], pubkey[:])

	// IPv6 address (2001:db8::1)
	ipv6 := []byte{0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	copy(data[32:48], ipv6)

	// Port (443)
	data[48] = 0x01
	data[49] = 0xBB

	entry, nextOffset, err := parser.ParseNodeEntry(data, 0)
	if err != nil {
		t.Fatalf("ParseNodeEntry failed: %v", err)
	}

	if nextOffset != 50 {
		t.Errorf("Expected next offset 50, got %d", nextOffset)
	}

	if entry.Address.Type != AddressTypeIPv6 {
		t.Errorf("Expected IPv6 address type, got %s", entry.Address.Type.String())
	}

	if entry.Address.Port != 443 {
		t.Errorf("Expected port 443, got %d", entry.Address.Port)
	}

	if !bytes.Equal(entry.Address.Data, ipv6) {
		t.Errorf("IPv6 data mismatch")
	}
}

// TestLegacyIPParser_ParseNodeEntry_InsufficientData tests error handling.
func TestLegacyIPParser_ParseNodeEntry_InsufficientData(t *testing.T) {
	parser := NewLegacyIPParser()

	shortData := make([]byte, 30) // Less than required 50 bytes

	_, _, err := parser.ParseNodeEntry(shortData, 0)
	if err == nil {
		t.Error("Expected error for insufficient data, got nil")
	}
}

// TestLegacyIPParser_SerializeNodeEntry tests serialization of node entries.
func TestLegacyIPParser_SerializeNodeEntry(t *testing.T) {
	parser := NewLegacyIPParser()

	entry := &NodeEntry{
		PublicKey: [32]byte{0x11, 0x22, 0x33, 0x44},
		Address: &NetworkAddress{
			Type:    AddressTypeIPv4,
			Data:    []byte{10, 0, 0, 1},
			Port:    8080,
			Network: "udp",
		},
	}

	data, err := parser.SerializeNodeEntry(entry)
	if err != nil {
		t.Fatalf("SerializeNodeEntry failed: %v", err)
	}

	if len(data) != 50 {
		t.Errorf("Expected 50 bytes, got %d", len(data))
	}

	// Verify public key
	if !bytes.Equal(data[0:32], entry.PublicKey[:]) {
		t.Error("Public key mismatch in serialized data")
	}

	// Verify IPv4 in mapped format
	if data[42] != 0xff || data[43] != 0xff {
		t.Error("IPv4-mapped IPv6 format incorrect")
	}

	if !bytes.Equal(data[44:48], []byte{10, 0, 0, 1}) {
		t.Error("IPv4 address mismatch in serialized data")
	}

	// Verify port
	expectedPort := uint16(8080)
	actualPort := uint16(data[48])<<8 | uint16(data[49])
	if actualPort != expectedPort {
		t.Errorf("Port mismatch: expected %d, got %d", expectedPort, actualPort)
	}
}

// TestLegacyIPParser_SerializeNodeEntry_UnsupportedAddressType tests error handling for unsupported types.
func TestLegacyIPParser_SerializeNodeEntry_UnsupportedAddressType(t *testing.T) {
	parser := NewLegacyIPParser()

	entry := &NodeEntry{
		PublicKey: [32]byte{},
		Address: &NetworkAddress{
			Type:    AddressTypeOnion,
			Data:    []byte("example.onion"),
			Port:    8080,
			Network: "tcp",
		},
	}

	_, err := parser.SerializeNodeEntry(entry)
	if err == nil {
		t.Error("Expected error for unsupported address type, got nil")
	}
}

// TestLegacyIPParser_SupportedAddressTypes tests the supported address types.
func TestLegacyIPParser_SupportedAddressTypes(t *testing.T) {
	parser := NewLegacyIPParser()

	supported := parser.SupportedAddressTypes()
	expected := []AddressType{AddressTypeIPv4, AddressTypeIPv6}

	if len(supported) != len(expected) {
		t.Errorf("Expected %d supported types, got %d", len(expected), len(supported))
	}

	for i, addrType := range expected {
		if supported[i] != addrType {
			t.Errorf("Expected supported type %s at index %d, got %s", addrType.String(), i, supported[i].String())
		}
	}
}

// TestLegacyIPParser_GetWireFormatVersion tests the wire format version.
func TestLegacyIPParser_GetWireFormatVersion(t *testing.T) {
	parser := NewLegacyIPParser()

	version := parser.GetWireFormatVersion()
	if version != ProtocolLegacy {
		t.Errorf("Expected ProtocolLegacy, got %s", version.String())
	}
}

// TestExtendedParser_ParseNodeEntry tests parsing extended format node entries.
func TestExtendedParser_ParseNodeEntry(t *testing.T) {
	parser := NewExtendedParser()

	// Create test data for Onion address node entry
	pubkey := [32]byte{0xAA, 0xBB, 0xCC, 0xDD}
	addrData := []byte("example.onion")

	data := make([]byte, 32+1+1+len(addrData)+2) // pubkey + type + len + addr + port
	offset := 0

	// Public key
	copy(data[offset:offset+32], pubkey[:])
	offset += 32

	// Address type
	data[offset] = byte(AddressTypeOnion)
	offset++

	// Address length
	data[offset] = byte(len(addrData))
	offset++

	// Address data
	copy(data[offset:offset+len(addrData)], addrData)
	offset += len(addrData)

	// Port (8080)
	data[offset] = 0x1F
	data[offset+1] = 0x90

	entry, nextOffset, err := parser.ParseNodeEntry(data, 0)
	if err != nil {
		t.Fatalf("ParseNodeEntry failed: %v", err)
	}

	expectedNextOffset := len(data)
	if nextOffset != expectedNextOffset {
		t.Errorf("Expected next offset %d, got %d", expectedNextOffset, nextOffset)
	}

	if entry.PublicKey != pubkey {
		t.Error("Public key mismatch")
	}

	if entry.Address.Type != AddressTypeOnion {
		t.Errorf("Expected Onion address type, got %s", entry.Address.Type.String())
	}

	if entry.Address.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", entry.Address.Port)
	}

	if !bytes.Equal(entry.Address.Data, addrData) {
		t.Errorf("Address data mismatch: expected %s, got %s", string(addrData), string(entry.Address.Data))
	}

	if entry.Address.Network != "tcp" {
		t.Errorf("Expected network 'tcp', got %s", entry.Address.Network)
	}
}

// TestExtendedParser_SerializeNodeEntry tests serialization of extended format.
func TestExtendedParser_SerializeNodeEntry(t *testing.T) {
	parser := NewExtendedParser()

	entry := &NodeEntry{
		PublicKey: [32]byte{0x55, 0x66, 0x77, 0x88},
		Address: &NetworkAddress{
			Type:    AddressTypeI2P,
			Data:    []byte("example12345678901234567890123456.b32.i2p"),
			Port:    8080,
			Network: "tcp",
		},
	}

	data, err := parser.SerializeNodeEntry(entry)
	if err != nil {
		t.Fatalf("SerializeNodeEntry failed: %v", err)
	}

	expectedSize := 32 + 1 + 1 + len(entry.Address.Data) + 2
	if len(data) != expectedSize {
		t.Errorf("Expected %d bytes, got %d", expectedSize, len(data))
	}

	// Verify public key
	if !bytes.Equal(data[0:32], entry.PublicKey[:]) {
		t.Error("Public key mismatch in serialized data")
	}

	// Verify address type
	if data[32] != byte(AddressTypeI2P) {
		t.Errorf("Address type mismatch: expected %d, got %d", AddressTypeI2P, data[32])
	}

	// Verify address length
	expectedLen := len(entry.Address.Data)
	if data[33] != byte(expectedLen) {
		t.Errorf("Address length mismatch: expected %d, got %d", expectedLen, data[33])
	}

	// Verify address data
	addrStart := 34
	addrEnd := addrStart + expectedLen
	if !bytes.Equal(data[addrStart:addrEnd], entry.Address.Data) {
		t.Error("Address data mismatch in serialized data")
	}

	// Verify port
	portOffset := addrEnd
	expectedPort := uint16(8080)
	actualPort := uint16(data[portOffset])<<8 | uint16(data[portOffset+1])
	if actualPort != expectedPort {
		t.Errorf("Port mismatch: expected %d, got %d", expectedPort, actualPort)
	}
}

// TestExtendedParser_SupportedAddressTypes tests the supported address types.
func TestExtendedParser_SupportedAddressTypes(t *testing.T) {
	parser := NewExtendedParser()

	supported := parser.SupportedAddressTypes()
	expected := []AddressType{
		AddressTypeIPv4,
		AddressTypeIPv6,
		AddressTypeOnion,
		AddressTypeI2P,
		AddressTypeNym,
		AddressTypeLoki,
		AddressTypeUnknown,
	}

	if len(supported) != len(expected) {
		t.Errorf("Expected %d supported types, got %d", len(expected), len(supported))
	}

	for i, addrType := range expected {
		if supported[i] != addrType {
			t.Errorf("Expected supported type %s at index %d, got %s", addrType.String(), i, supported[i].String())
		}
	}
}

// TestExtendedParser_GetWireFormatVersion tests the wire format version.
func TestExtendedParser_GetWireFormatVersion(t *testing.T) {
	parser := NewExtendedParser()

	version := parser.GetWireFormatVersion()
	if version != ProtocolNoiseIK {
		t.Errorf("Expected ProtocolNoiseIK, got %s", version.String())
	}
}

// TestParserSelector_SelectParser tests parser selection by protocol version.
func TestParserSelector_SelectParser(t *testing.T) {
	selector := NewParserSelector()

	tests := []struct {
		version      ProtocolVersion
		expectedType string
	}{
		{ProtocolLegacy, "*transport.LegacyIPParser"},
		{ProtocolNoiseIK, "*transport.ExtendedParser"},
		{ProtocolVersion(99), "*transport.LegacyIPParser"}, // Unknown defaults to legacy
	}

	for _, tt := range tests {
		t.Run(tt.version.String(), func(t *testing.T) {
			parser := selector.SelectParser(tt.version)

			// Check by testing the wire format version
			var actualType string
			if parser.GetWireFormatVersion() == ProtocolLegacy {
				actualType = "*transport.LegacyIPParser"
			} else {
				actualType = "*transport.ExtendedParser"
			}

			if actualType != tt.expectedType {
				t.Errorf("Expected %s, got %s", tt.expectedType, actualType)
			}
		})
	}
}

// TestParserSelector_SelectParserForAddressType tests parser selection by address type.
func TestParserSelector_SelectParserForAddressType(t *testing.T) {
	selector := NewParserSelector()

	tests := []struct {
		addrType     AddressType
		expectedType string
	}{
		{AddressTypeIPv4, "*transport.LegacyIPParser"},
		{AddressTypeIPv6, "*transport.LegacyIPParser"},
		{AddressTypeOnion, "*transport.ExtendedParser"},
		{AddressTypeI2P, "*transport.ExtendedParser"},
		{AddressTypeNym, "*transport.ExtendedParser"},
		{AddressTypeLoki, "*transport.ExtendedParser"},
		{AddressTypeUnknown, "*transport.ExtendedParser"},
	}

	for _, tt := range tests {
		t.Run(tt.addrType.String(), func(t *testing.T) {
			parser := selector.SelectParserForAddressType(tt.addrType)

			// Check by testing the wire format version
			var actualType string
			if parser.GetWireFormatVersion() == ProtocolLegacy {
				actualType = "*transport.LegacyIPParser"
			} else {
				actualType = "*transport.ExtendedParser"
			}

			if actualType != tt.expectedType {
				t.Errorf("Expected %s, got %s", tt.expectedType, actualType)
			}
		})
	}
}

// TestRoundTripCompatibility tests that data can be serialized and parsed correctly.
func TestRoundTripCompatibility(t *testing.T) {
	tests := []struct {
		name   string
		parser PacketParser
		entry  *NodeEntry
	}{
		{
			name:   "Legacy IPv4",
			parser: NewLegacyIPParser(),
			entry: &NodeEntry{
				PublicKey: [32]byte{0x11, 0x22, 0x33, 0x44},
				Address: &NetworkAddress{
					Type:    AddressTypeIPv4,
					Data:    []byte{192, 168, 1, 100},
					Port:    8080,
					Network: "udp",
				},
				LastSeen: time.Now(),
			},
		},
		{
			name:   "Extended Onion",
			parser: NewExtendedParser(),
			entry: &NodeEntry{
				PublicKey: [32]byte{0xAA, 0xBB, 0xCC, 0xDD},
				Address: &NetworkAddress{
					Type:    AddressTypeOnion,
					Data:    []byte("exampleexampleexample.onion"),
					Port:    8080,
					Network: "tcp",
				},
				LastSeen: time.Now(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			data, err := tt.parser.SerializeNodeEntry(tt.entry)
			if err != nil {
				t.Fatalf("Serialization failed: %v", err)
			}

			// Parse
			parsed, _, err := tt.parser.ParseNodeEntry(data, 0)
			if err != nil {
				t.Fatalf("Parsing failed: %v", err)
			}

			// Compare
			if parsed.PublicKey != tt.entry.PublicKey {
				t.Error("Public key mismatch after round trip")
			}

			if parsed.Address.Type != tt.entry.Address.Type {
				t.Errorf("Address type mismatch: expected %s, got %s",
					tt.entry.Address.Type.String(), parsed.Address.Type.String())
			}

			if parsed.Address.Port != tt.entry.Address.Port {
				t.Errorf("Port mismatch: expected %d, got %d", tt.entry.Address.Port, parsed.Address.Port)
			}

			if !bytes.Equal(parsed.Address.Data, tt.entry.Address.Data) {
				t.Errorf("Address data mismatch: expected %v, got %v", tt.entry.Address.Data, parsed.Address.Data)
			}
		})
	}
}

// BenchmarkLegacyParser_ParseNodeEntry benchmarks legacy parsing performance.
func BenchmarkLegacyParser_ParseNodeEntry(b *testing.B) {
	parser := NewLegacyIPParser()
	data := make([]byte, 50)

	// Set up valid test data
	data[42] = 0xff
	data[43] = 0xff
	data[44] = 192
	data[45] = 168
	data[46] = 1
	data[47] = 1
	data[48] = 0x1F
	data[49] = 0x90

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := parser.ParseNodeEntry(data, 0)
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

// BenchmarkExtendedParser_ParseNodeEntry benchmarks extended parsing performance.
func BenchmarkExtendedParser_ParseNodeEntry(b *testing.B) {
	parser := NewExtendedParser()

	// Create test data
	addrData := []byte("example.onion")
	data := make([]byte, 32+1+1+len(addrData)+2)

	offset := 32
	data[offset] = byte(AddressTypeOnion)
	data[offset+1] = byte(len(addrData))
	copy(data[offset+2:], addrData)
	data[len(data)-2] = 0x1F
	data[len(data)-1] = 0x90

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := parser.ParseNodeEntry(data, 0)
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

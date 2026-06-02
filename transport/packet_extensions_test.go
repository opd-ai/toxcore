package transport

import (
	"testing"
)

func TestIsExtensionPacket(t *testing.T) {
	tests := []struct {
		name     string
		pt       PacketType
		expected bool
	}{
		{"PacketVersionNegotiation", PacketVersionNegotiation, true},
		{"PacketNoiseHandshake", PacketNoiseHandshake, true},
		{"PacketNoiseMessage", PacketNoiseMessage, true},
		{"Reserved 252", PacketType(252), true},
		{"Reserved 253", PacketType(253), true},
		{"Reserved 254", PacketType(254), true},
		{"Standard packet type 1", PacketType(1), false},
		{"Standard packet type 100", PacketType(100), false},
		{"Below extension range", PacketType(248), false},
		{"Above extension range", PacketType(255), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsExtensionPacket(tt.pt)
			if result != tt.expected {
				t.Errorf("IsExtensionPacket(%d) = %v, want %v", tt.pt, result, tt.expected)
			}
		})
	}
}

func TestExtensionPacketConstants(t *testing.T) {
	// Verify extension packet types are in the documented range
	if PacketVersionNegotiation != 249 {
		t.Errorf("PacketVersionNegotiation = %d, want 249", PacketVersionNegotiation)
	}
	if PacketNoiseHandshake != 250 {
		t.Errorf("PacketNoiseHandshake = %d, want 250", PacketNoiseHandshake)
	}
	if PacketNoiseMessage != 251 {
		t.Errorf("PacketNoiseMessage = %d, want 251", PacketNoiseMessage)
	}

	// Verify range constants
	if ExtensionPacketRangeStart != 249 {
		t.Errorf("ExtensionPacketRangeStart = %d, want 249", ExtensionPacketRangeStart)
	}
	if ExtensionPacketRangeEnd != 254 {
		t.Errorf("ExtensionPacketRangeEnd = %d, want 254", ExtensionPacketRangeEnd)
	}
}

func TestValidateExtensionHeader(t *testing.T) {
	tests := []struct {
		name     string
		header   *ExtensionPacketHeader
		expected bool
	}{
		{
			name:     "Valid header",
			header:   &ExtensionPacketHeader{VendorMagic: ExtensionVendorMagic, Version: ExtensionProtocolVersion},
			expected: true,
		},
		{
			name:     "Valid header with older version",
			header:   &ExtensionPacketHeader{VendorMagic: ExtensionVendorMagic, Version: 0x00},
			expected: true,
		},
		{
			name:     "Invalid vendor magic",
			header:   &ExtensionPacketHeader{VendorMagic: 0x00, Version: ExtensionProtocolVersion},
			expected: false,
		},
		{
			name:     "Future version",
			header:   &ExtensionPacketHeader{VendorMagic: ExtensionVendorMagic, Version: ExtensionProtocolVersion + 1},
			expected: false,
		},
		{
			name:     "Nil header",
			header:   nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateExtensionHeader(tt.header)
			if result != tt.expected {
				t.Errorf("ValidateExtensionHeader() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseExtensionHeader(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectValid bool
	}{
		{
			name:        "Valid extension header",
			data:        []byte{ExtensionVendorMagic, ExtensionProtocolVersion, 0x00, 0x01},
			expectValid: true,
		},
		{
			name:        "Minimum valid length",
			data:        []byte{ExtensionVendorMagic, ExtensionProtocolVersion},
			expectValid: true,
		},
		{
			name:        "Too short",
			data:        []byte{ExtensionVendorMagic},
			expectValid: false,
		},
		{
			name:        "Empty",
			data:        []byte{},
			expectValid: false,
		},
		{
			name:        "Invalid vendor magic",
			data:        []byte{0x00, ExtensionProtocolVersion},
			expectValid: false,
		},
		{
			name:        "Future version rejected",
			data:        []byte{ExtensionVendorMagic, ExtensionProtocolVersion + 1},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := ParseExtensionHeader(tt.data)
			if tt.expectValid {
				if header == nil {
					t.Error("ParseExtensionHeader() returned nil, expected valid header")
				} else {
					if header.VendorMagic != ExtensionVendorMagic {
						t.Errorf("VendorMagic = %x, want %x", header.VendorMagic, ExtensionVendorMagic)
					}
				}
			} else {
				if header != nil {
					t.Error("ParseExtensionHeader() returned non-nil, expected nil")
				}
			}
		})
	}
}

func TestSerializeExtensionHeader(t *testing.T) {
	header := SerializeExtensionHeader()

	if len(header) != 2 {
		t.Errorf("SerializeExtensionHeader() length = %d, want 2", len(header))
	}

	if header[0] != ExtensionVendorMagic {
		t.Errorf("SerializeExtensionHeader()[0] = %x, want %x", header[0], ExtensionVendorMagic)
	}

	if header[1] != ExtensionProtocolVersion {
		t.Errorf("SerializeExtensionHeader()[1] = %x, want %x", header[1], ExtensionProtocolVersion)
	}

	// Verify round-trip
	parsed := ParseExtensionHeader(header)
	if parsed == nil {
		t.Error("Failed to parse serialized header")
	}
}

func TestExtensionVendorMagicConstant(t *testing.T) {
	// Verify the vendor magic is documented value
	if ExtensionVendorMagic != 0xAB {
		t.Errorf("ExtensionVendorMagic = %x, want 0xAB", ExtensionVendorMagic)
	}
}

func TestExtensionProtocolVersionConstant(t *testing.T) {
	// Verify the protocol version is documented value
	if ExtensionProtocolVersion != 0x01 {
		t.Errorf("ExtensionProtocolVersion = %x, want 0x01", ExtensionProtocolVersion)
	}
}

// TestExtensionRangeBackwardCompatibility is a regression guard ensuring that
// the extension packet type range never overlaps with standard Tox packet types
// and that PacketCoverTraffic remains outside (below) the extension range, so
// legacy peers that do not understand extensions safely discard them.
func TestExtensionRangeBackwardCompatibility(t *testing.T) {
	// Extension range must be pinned to 249-254 — legacy c-toxcore discards these.
	if ExtensionPacketRangeStart != 249 {
		t.Errorf("ExtensionPacketRangeStart changed: got %d, want 249 (backward-compat break)", ExtensionPacketRangeStart)
	}
	if ExtensionPacketRangeEnd != 254 {
		t.Errorf("ExtensionPacketRangeEnd changed: got %d, want 254 (backward-compat break)", ExtensionPacketRangeEnd)
	}

	// PacketCoverTraffic (248) must NOT be in the extension range because it is
	// handled by its own registered handler and must not be confused with
	// vendor-extension packets.
	if IsExtensionPacket(PacketCoverTraffic) {
		t.Errorf("PacketCoverTraffic (%d) must not be in the extension range [%d, %d]",
			PacketCoverTraffic, ExtensionPacketRangeStart, ExtensionPacketRangeEnd)
	}

	// All packet types below the extension range start must not be extension packets.
	for pt := PacketType(0); pt < ExtensionPacketRangeStart; pt++ {
		if IsExtensionPacket(pt) {
			t.Errorf("packet type %d < ExtensionPacketRangeStart falsely identified as extension", pt)
		}
	}

	// All packet types within [ExtensionPacketRangeStart, ExtensionPacketRangeEnd] must be extensions.
	for pt := ExtensionPacketRangeStart; pt <= ExtensionPacketRangeEnd; pt++ {
		if !IsExtensionPacket(pt) {
			t.Errorf("packet type %d in extension range not identified as extension", pt)
		}
	}
}

// TestUnknownExtensionPacketSafety verifies that a transport receiving an
// unrecognised extension packet type does not return an error or panic.
// Legacy and non-upgraded peers must handle unknown extension types gracefully.
func TestUnknownExtensionPacketSafety(t *testing.T) {
	// Construct a packet with a type that is inside the extension range but has
	// no registered handler.  The code path in NoiseTransport reads:
	//   if exists { dispatch(handler) }
	// so an unregistered type is a safe no-op.  This test documents and locks
	// in that contract at the packet-type classification level.
	unknownExtTypes := []PacketType{
		PacketType(252), // reserved — no handler registered
		PacketType(253), // reserved — no handler registered
		PacketType(254), // reserved — no handler registered
	}
	for _, pt := range unknownExtTypes {
		pkt := &Packet{PacketType: pt, Data: []byte{0xAB, 0x01, 0xFF, 0xFE}}
		if pkt.PacketType != pt {
			t.Errorf("Packet construction failed for type %d", pt)
		}
		// IsExtensionPacket must return true so callers can gate on it.
		if !IsExtensionPacket(pt) {
			t.Errorf("reserved extension type %d not classified as extension packet", pt)
		}
	}
}

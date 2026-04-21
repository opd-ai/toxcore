package dht

import (
	"net"
	"testing"
)

// FuzzParseLANDiscoveryPacket fuzzes the LAN discovery packet parser.
// It verifies that the parser never panics on arbitrary input and that
// successful parses always produce a valid 32-byte public key and port.
func FuzzParseLANDiscoveryPacket(f *testing.F) {
	// Valid 34-byte packet: 32-byte public key + 2-byte port.
	validPacket := make([]byte, 34)
	for i := range validPacket {
		validPacket[i] = byte(i)
	}
	f.Add(validPacket)
	f.Add([]byte{})                // Empty
	f.Add([]byte{0x00})            // Single byte
	f.Add(make([]byte, 33))        // One byte short
	f.Add(make([]byte, 35))        // One byte over minimum
	f.Add(make([]byte, 100))       // Large packet

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic.
		pubKey, port, err := ParseLANDiscoveryPacket(data)
		if err != nil {
			return
		}
		// On success the public key must be exactly 32 bytes.
		if len(pubKey) != 32 {
			t.Errorf("expected 32-byte public key, got %d bytes", len(pubKey))
		}
		// Port may be any uint16 value including zero.
		_ = port
	})
}

// FuzzParseIPFromType fuzzes the IP address parser with arbitrary IP type bytes
// and data buffers.  The function must never panic.
func FuzzParseIPFromType(f *testing.F) {
	// IPv4 seed: type=2, 4-byte address.
	f.Add(byte(2), []byte{127, 0, 0, 1})
	// IPv6 seed: type=10, 16-byte address.
	f.Add(byte(10), make([]byte, 16))
	// Unknown type seeds.
	f.Add(byte(0), []byte{})
	f.Add(byte(255), make([]byte, 32))

	f.Fuzz(func(t *testing.T, ipType byte, data []byte) {
		ip, ipLen, err := parseIPFromType(data, 0, ipType)
		if err != nil {
			return
		}
		// On success validate returned values.
		if ip == nil {
			t.Error("successful parse returned nil IP")
		}
		if ipLen <= 0 {
			t.Errorf("successful parse returned non-positive ipLen %d", ipLen)
		}
		if ipLen > len(data) {
			t.Errorf("ipLen %d exceeds data length %d", ipLen, len(data))
		}
	})
}

// FuzzParsePort fuzzes the 2-byte big-endian port parser.
func FuzzParsePort(f *testing.F) {
	f.Add([]byte{0x00, 0x50})      // Port 80
	f.Add([]byte{0x1a, 0xe1})      // Port 6881
	f.Add([]byte{})                // Empty
	f.Add([]byte{0xff})            // Truncated
	f.Add([]byte{0xff, 0xff})      // Max port

	f.Fuzz(func(t *testing.T, data []byte) {
		port, newOffset, err := parsePort(data, 0)
		if err != nil {
			return
		}
		// Successful parse must advance offset by exactly 2.
		if newOffset != 2 {
			t.Errorf("expected newOffset=2, got %d", newOffset)
		}
		_ = port
	})
}

// FuzzParsePublicKey fuzzes the 32-byte public key parser.
func FuzzParsePublicKey(f *testing.F) {
	validKey := make([]byte, 32)
	for i := range validKey {
		validKey[i] = byte(i * 7 % 256)
	}
	f.Add(validKey)
	f.Add(make([]byte, 31))        // One byte short
	f.Add(make([]byte, 33))        // One byte over
	f.Add([]byte{})               // Empty

	f.Fuzz(func(t *testing.T, data []byte) {
		pk, newOffset, err := parsePublicKey(data, 0)
		if err != nil {
			return
		}
		// Successful parse must advance offset by exactly 32.
		if newOffset != 32 {
			t.Errorf("expected newOffset=32, got %d", newOffset)
		}
		if len(pk) != 32 {
			t.Errorf("expected 32-byte key, got %d bytes", len(pk))
		}
	})
}

// FuzzDeserializeRelayAnnouncement fuzzes the relay announcement deserializer.
func FuzzDeserializeRelayAnnouncement(f *testing.F) {
	// Build a minimal valid announcement (53 bytes header + 0-byte address).
	validAnnouncement := make([]byte, 53)
	// pubKey: bytes 0-31 (all zero is fine for testing)
	// port: bytes 32-33
	validAnnouncement[32] = 0x1a
	validAnnouncement[33] = 0xe1
	// priority: bytes 34-37
	// timestamp: bytes 38-45
	// capacity: bytes 46-49
	// load: byte 50
	// addrLen: bytes 51-52 (zero means empty address string)
	f.Add(validAnnouncement)
	f.Add([]byte{})
	f.Add(make([]byte, 52))          // One byte short of minimum
	f.Add(make([]byte, 54))          // Minimum + 1

	// Announcement with a non-empty address.
	withAddr := make([]byte, 53+9)
	withAddr[51] = 0
	withAddr[52] = 9 // addrLen = 9
	copy(withAddr[53:], []byte("127.0.0.1"))
	f.Add(withAddr)

	f.Fuzz(func(t *testing.T, data []byte) {
		ann, err := DeserializeRelayAnnouncement(data)
		if err != nil {
			return
		}
		// On success the announcement must have a valid public key length.
		if len(ann.PublicKey) != 32 {
			t.Errorf("expected 32-byte public key, got %d", len(ann.PublicKey))
		}
	})
}

// FuzzGossipParseNodeEntry fuzzes the GossipBootstrap node-entry parser with
// both IPv4 and IPv6 address types.
func FuzzGossipParseNodeEntry(f *testing.F) {
	// Build a valid IPv4 gossip node entry: 1+4+2+32 = 39 bytes.
	ipv4Entry := make([]byte, 39)
	ipv4Entry[0] = 2 // IPv4
	copy(ipv4Entry[1:5], net.ParseIP("127.0.0.1").To4())
	ipv4Entry[5] = 0x1a // port high byte
	ipv4Entry[6] = 0xe1 // port low byte
	// Public key bytes 7-38 are zero.
	f.Add(ipv4Entry)

	// Build a valid IPv6 gossip node entry: 1+16+2+32 = 51 bytes.
	ipv6Entry := make([]byte, 51)
	ipv6Entry[0] = 10 // IPv6
	copy(ipv6Entry[1:17], net.IPv6loopback)
	ipv6Entry[17] = 0x1a
	ipv6Entry[18] = 0xe1
	f.Add(ipv6Entry)

	// Edge cases.
	f.Add([]byte{})
	f.Add([]byte{2})       // Type only, no data
	f.Add(make([]byte, 5)) // Truncated IPv4

	gb := newFuzzGossipBootstrap()
	senderAddr := newMockAddr("127.0.0.1:33445")

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic; errors are expected for malformed input.
		_, _, _ = gb.parseNodeEntry(data, 0, senderAddr)
	})
}

// newFuzzGossipBootstrap constructs a minimal GossipBootstrap for use in fuzz tests.
func newFuzzGossipBootstrap() *GossipBootstrap {
	selfID := createTestToxID(1)
	mockT := newMockTransportForGossip()
	return NewGossipBootstrap(selfID, mockT, nil, DefaultGossipConfig())
}

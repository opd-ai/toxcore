// Package transport implements the network transport layer for the Tox protocol.
//
// # Extension Packet Types
//
// This file documents extension packet types used by opd-ai/toxcore that are not
// part of the standard c-toxcore implementation. These extensions occupy packet
// type values 249-254 (0xF9-0xFE), which are in the custom/reserved range per
// the Tox protocol specification.
//
// Extension packet types are designed to:
//   - Be ignored by legacy c-toxcore clients (graceful degradation)
//   - Support version negotiation for feature detection
//   - Maintain backward compatibility with standard Tox protocol
//
// # Vendor Extension Scheme
//
// To prevent future collision with c-toxcore extensions or other implementations,
// opd-ai extension packets use an internal vendor identifier scheme. Extension
// packets include a vendor magic byte (0xAB) in their payload header to distinguish
// them from other potential uses of these packet type values.
//
// Format for extension packet payloads:
//
//	[vendor_magic(1)][extension_version(1)][payload_data(variable)]
//
// Where:
//   - vendor_magic: 0xAB (opd-ai vendor identifier)
//   - extension_version: Version of the extension protocol (currently 0x01)
//   - payload_data: Extension-specific payload
//
// # Registered Extension Types
//
// | Value | Constant                  | Description                         | Since |
// |-------|---------------------------|-------------------------------------|-------|
// | 249   | PacketVersionNegotiation  | Protocol version negotiation        | v0.1  |
// | 250   | PacketNoiseHandshake      | Noise-IK protocol handshake         | v0.1  |
// | 251   | PacketNoiseMessage        | Noise-encrypted message payload     | v0.1  |
// | 252   | (reserved)                | Reserved for future use             | -     |
// | 253   | (reserved)                | Reserved for future use             | -     |
// | 254   | (reserved)                | Reserved for future use             | -     |
//
// Note: Value 255 (0xFF) is avoided as it has special meaning in some contexts.
//
// # Compatibility Notes
//
// When communicating with standard c-toxcore peers:
//   - Extension packets will be ignored (unknown packet type)
//   - Version negotiation detects peer capabilities before using extensions
//   - Fallback to legacy protocol is supported when peer lacks extensions
//
// For interoperability with other Tox implementations, the version negotiation
// protocol should be used to detect extension support before sending extension
// packet types.
package transport

// ExtensionVendorMagic is the vendor identifier for opd-ai extension packets.
// This magic byte is included in extension packet payloads to distinguish
// opd-ai extensions from other potential uses of reserved packet type values.
const ExtensionVendorMagic byte = 0xAB

// ExtensionProtocolVersion is the current version of the opd-ai extension protocol.
// This version number is included in extension packet payloads and should be
// incremented when breaking changes are made to extension packet formats.
const ExtensionProtocolVersion byte = 0x01

// ExtensionPacketRange defines the packet type range used for opd-ai extensions.
const (
	// ExtensionPacketRangeStart is the first packet type value reserved for extensions.
	ExtensionPacketRangeStart PacketType = 249

	// ExtensionPacketRangeEnd is the last packet type value reserved for extensions.
	// Value 255 is avoided to prevent conflicts with special sentinel values.
	ExtensionPacketRangeEnd PacketType = 254
)

// IsExtensionPacket returns true if the packet type is in the opd-ai extension range.
func IsExtensionPacket(pt PacketType) bool {
	return pt >= ExtensionPacketRangeStart && pt <= ExtensionPacketRangeEnd
}

// ExtensionPacketHeader represents the common header for all extension packets.
// Extension packets include this header in their payload for identification.
type ExtensionPacketHeader struct {
	VendorMagic byte // Should be ExtensionVendorMagic (0xAB)
	Version     byte // Extension protocol version
}

// ValidateExtensionHeader validates that an extension packet header is valid.
// Returns true if the header has the correct vendor magic and a recognized version.
func ValidateExtensionHeader(header *ExtensionPacketHeader) bool {
	if header == nil {
		return false
	}
	return header.VendorMagic == ExtensionVendorMagic && header.Version <= ExtensionProtocolVersion
}

// ParseExtensionHeader parses the extension header from raw packet data.
// Returns nil if the data is too short or doesn't contain a valid extension header.
func ParseExtensionHeader(data []byte) *ExtensionPacketHeader {
	if len(data) < 2 {
		return nil
	}

	header := &ExtensionPacketHeader{
		VendorMagic: data[0],
		Version:     data[1],
	}

	if !ValidateExtensionHeader(header) {
		return nil
	}

	return header
}

// SerializeExtensionHeader creates the byte representation of an extension header.
func SerializeExtensionHeader() []byte {
	return []byte{ExtensionVendorMagic, ExtensionProtocolVersion}
}

// Package transport implements packet parsing interfaces for protocol versioning.
//
// This file provides the PacketParser interface and implementations that support
// both legacy IP-based packet formats and extended formats for multi-network support.
package transport

import (
	"errors"
	"fmt"
	"time"
)

// NodeEntry represents a network node in the DHT with address abstraction.
// This replaces the IP-specific node representations in the legacy protocol.
type NodeEntry struct {
	// PublicKey is the Ed25519 public key of the node
	PublicKey [32]byte
	// Address is the network address using the new multi-network system
	Address *NetworkAddress
	// LastSeen tracks when this node was last observed (for DHT maintenance)
	LastSeen time.Time
}

// String returns a human-readable representation of the NodeEntry.
func (ne *NodeEntry) String() string {
	return fmt.Sprintf("Node{PubKey: %x..., Address: %s, LastSeen: %s}",
		ne.PublicKey[:4], ne.Address.String(), ne.LastSeen.Format("15:04:05"))
}

// IsExpired checks if the node entry is considered expired based on the given timeout.
func (ne *NodeEntry) IsExpired(timeout time.Duration) bool {
	return time.Since(ne.LastSeen) > timeout
}

// PacketParser defines the interface for parsing different packet formats.
// This abstraction allows the protocol to support both legacy IP-based formats
// and new extended formats that include multi-network address support.
type PacketParser interface {
	// ParseNodeEntry extracts a node entry from packet data at the given offset.
	// Returns the parsed node entry, the next offset, and any error.
	ParseNodeEntry(data []byte, offset int) (*NodeEntry, int, error)

	// SerializeNodeEntry converts a node entry to its wire format representation.
	// Returns the serialized bytes and any error.
	SerializeNodeEntry(entry *NodeEntry) ([]byte, error)

	// SupportedAddressTypes returns the address types this parser can handle.
	SupportedAddressTypes() []AddressType

	// GetWireFormatVersion returns the wire format version this parser implements.
	GetWireFormatVersion() ProtocolVersion
}

// LegacyIPParser implements PacketParser for backward compatibility with IP-only formats.
// This parser handles the original 16-byte IP + 2-byte port format used in legacy Tox.
type LegacyIPParser struct{}

// NewLegacyIPParser creates a new parser for legacy IP-based packet formats.
func NewLegacyIPParser() PacketParser {
	return &LegacyIPParser{}
}

// ParseNodeEntry implements PacketParser.ParseNodeEntry for legacy IP format.
// Legacy format: [public_key(32)][ip(16)][port(2)] = 50 bytes per node
func (p *LegacyIPParser) ParseNodeEntry(data []byte, offset int) (*NodeEntry, int, error) {
	const legacyNodeSize = 32 + 16 + 2 // pubkey + ip + port

	if len(data) < offset+legacyNodeSize {
		return nil, offset, fmt.Errorf("insufficient data for legacy node entry: need %d bytes, have %d",
			legacyNodeSize, len(data)-offset)
	}

	entry := &NodeEntry{
		LastSeen: time.Now(),
	}

	// Extract public key
	copy(entry.PublicKey[:], data[offset:offset+32])

	// Extract IP and port using the legacy format
	var ip [16]byte
	copy(ip[:], data[offset+32:offset+48])
	port := uint16(data[offset+48])<<8 | uint16(data[offset+49])

	// Convert legacy IP format to NetworkAddress
	var addrType AddressType
	var addrData []byte

	// Check if this is IPv4 (IPv4-mapped IPv6 format)
	if ip[0] == 0 && ip[1] == 0 && ip[2] == 0 && ip[3] == 0 &&
		ip[4] == 0 && ip[5] == 0 && ip[6] == 0 && ip[7] == 0 &&
		ip[8] == 0 && ip[9] == 0 && ip[10] == 0xff && ip[11] == 0xff {
		addrType = AddressTypeIPv4
		addrData = ip[12:16]
	} else {
		addrType = AddressTypeIPv6
		addrData = ip[:]
	}

	entry.Address = &NetworkAddress{
		Type:    addrType,
		Data:    addrData,
		Port:    port,
		Network: "udp", // Legacy protocol uses UDP
	}

	return entry, offset + legacyNodeSize, nil
}

// SerializeNodeEntry implements PacketParser.SerializeNodeEntry for legacy IP format.
func (p *LegacyIPParser) SerializeNodeEntry(entry *NodeEntry) ([]byte, error) {
	if entry == nil {
		return nil, errors.New("node entry cannot be nil")
	}

	if entry.Address == nil {
		return nil, errors.New("node entry address cannot be nil")
	}

	// Only support IPv4 and IPv6 in legacy format
	if entry.Address.Type != AddressTypeIPv4 && entry.Address.Type != AddressTypeIPv6 {
		return nil, fmt.Errorf("legacy parser only supports IPv4/IPv6, got %s", entry.Address.Type.String())
	}

	data := make([]byte, 50) // 32 + 16 + 2

	// Copy public key
	copy(data[0:32], entry.PublicKey[:])

	// Format address in legacy 16-byte format
	var ip [16]byte
	if entry.Address.Type == AddressTypeIPv4 {
		// IPv4-mapped IPv6 format
		ip[10] = 0xff
		ip[11] = 0xff
		if len(entry.Address.Data) >= 4 {
			copy(ip[12:16], entry.Address.Data[:4])
		}
	} else {
		// IPv6 format
		if len(entry.Address.Data) >= 16 {
			copy(ip[:], entry.Address.Data[:16])
		}
	}
	copy(data[32:48], ip[:])

	// Copy port
	data[48] = byte(entry.Address.Port >> 8)
	data[49] = byte(entry.Address.Port & 0xff)

	return data, nil
}

// SupportedAddressTypes implements PacketParser.SupportedAddressTypes for legacy format.
func (p *LegacyIPParser) SupportedAddressTypes() []AddressType {
	return []AddressType{AddressTypeIPv4, AddressTypeIPv6}
}

// GetWireFormatVersion implements PacketParser.GetWireFormatVersion for legacy format.
func (p *LegacyIPParser) GetWireFormatVersion() ProtocolVersion {
	return ProtocolLegacy
}

// ExtendedParser implements PacketParser for the new variable-length address format.
// This parser supports all network types including Tor, I2P, Nym, and Lokinet.
type ExtendedParser struct{}

// NewExtendedParser creates a new parser for extended multi-network packet formats.
func NewExtendedParser() PacketParser {
	return &ExtendedParser{}
}

// ParseNodeEntry implements PacketParser.ParseNodeEntry for extended format.
// Extended format: [public_key(32)][addr_type(1)][addr_len(1)][address(var)][port(2)]
func (p *ExtendedParser) ParseNodeEntry(data []byte, offset int) (*NodeEntry, int, error) {
	if len(data) < offset+35 { // minimum: 32 + 1 + 1 + 0 + 2
		return nil, offset, fmt.Errorf("insufficient data for extended node entry: need at least 35 bytes, have %d",
			len(data)-offset)
	}

	entry := &NodeEntry{
		LastSeen: time.Now(),
	}

	// Extract public key
	copy(entry.PublicKey[:], data[offset:offset+32])
	currentOffset := offset + 32

	// Extract address type
	addrType := AddressType(data[currentOffset])
	currentOffset++

	// Extract address length
	addrLen := int(data[currentOffset])
	currentOffset++

	// Validate we have enough data for the address and port
	if len(data) < currentOffset+addrLen+2 {
		return nil, offset, fmt.Errorf("insufficient data for address: need %d bytes, have %d",
			addrLen+2, len(data)-currentOffset)
	}

	// Extract address data
	addrData := make([]byte, addrLen)
	copy(addrData, data[currentOffset:currentOffset+addrLen])
	currentOffset += addrLen

	// Extract port
	port := uint16(data[currentOffset])<<8 | uint16(data[currentOffset+1])
	currentOffset += 2

	// Determine network type based on address type
	network := "udp" // default
	switch addrType {
	case AddressTypeOnion, AddressTypeI2P, AddressTypeNym, AddressTypeLoki:
		network = "tcp" // overlay networks typically use TCP-like semantics
	}

	entry.Address = &NetworkAddress{
		Type:    addrType,
		Data:    addrData,
		Port:    port,
		Network: network,
	}

	return entry, currentOffset, nil
}

// SerializeNodeEntry implements PacketParser.SerializeNodeEntry for extended format.
func (p *ExtendedParser) SerializeNodeEntry(entry *NodeEntry) ([]byte, error) {
	if entry == nil {
		return nil, errors.New("node entry cannot be nil")
	}

	if entry.Address == nil {
		return nil, errors.New("node entry address cannot be nil")
	}

	// Validate address data length
	addrLen := len(entry.Address.Data)
	if addrLen > 255 {
		return nil, fmt.Errorf("address data too long: %d bytes (max 255)", addrLen)
	}

	// Calculate total size: pubkey(32) + addr_type(1) + addr_len(1) + address(var) + port(2)
	totalSize := 32 + 1 + 1 + addrLen + 2
	data := make([]byte, totalSize)

	offset := 0

	// Copy public key
	copy(data[offset:offset+32], entry.PublicKey[:])
	offset += 32

	// Copy address type
	data[offset] = byte(entry.Address.Type)
	offset++

	// Copy address length
	data[offset] = byte(addrLen)
	offset++

	// Copy address data
	copy(data[offset:offset+addrLen], entry.Address.Data)
	offset += addrLen

	// Copy port
	data[offset] = byte(entry.Address.Port >> 8)
	data[offset+1] = byte(entry.Address.Port & 0xff)

	return data, nil
}

// SupportedAddressTypes implements PacketParser.SupportedAddressTypes for extended format.
func (p *ExtendedParser) SupportedAddressTypes() []AddressType {
	return []AddressType{
		AddressTypeIPv4,
		AddressTypeIPv6,
		AddressTypeOnion,
		AddressTypeI2P,
		AddressTypeNym,
		AddressTypeLoki,
		AddressTypeUnknown,
	}
}

// GetWireFormatVersion implements PacketParser.GetWireFormatVersion for extended format.
func (p *ExtendedParser) GetWireFormatVersion() ProtocolVersion {
	return ProtocolNoiseIK // Extended format introduced with Noise-IK protocol
}

// ParserSelector helps choose the appropriate parser based on protocol version.
type ParserSelector struct {
	LegacyParser   PacketParser
	ExtendedParser PacketParser
}

// NewParserSelector creates a new parser selector with both legacy and extended parsers.
func NewParserSelector() *ParserSelector {
	return &ParserSelector{
		LegacyParser:   NewLegacyIPParser(),
		ExtendedParser: NewExtendedParser(),
	}
}

// SelectParser returns the appropriate parser for the given protocol version.
func (ps *ParserSelector) SelectParser(version ProtocolVersion) PacketParser {
	switch version {
	case ProtocolLegacy:
		return ps.LegacyParser
	case ProtocolNoiseIK:
		return ps.ExtendedParser
	default:
		// Default to legacy for unknown versions (backward compatibility)
		return ps.LegacyParser
	}
}

// SelectParserForAddressType returns the appropriate parser for a given address type.
// This is useful when creating packets - we need to know which format to use.
func (ps *ParserSelector) SelectParserForAddressType(addrType AddressType) PacketParser {
	switch addrType {
	case AddressTypeIPv4, AddressTypeIPv6:
		// IP addresses can use either parser, but legacy is more compatible
		return ps.LegacyParser
	case AddressTypeOnion, AddressTypeI2P, AddressTypeNym, AddressTypeLoki:
		// Non-IP addresses require extended parser
		return ps.ExtendedParser
	default:
		// Unknown types use extended parser for future compatibility
		return ps.ExtendedParser
	}
}
